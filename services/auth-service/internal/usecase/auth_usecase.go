package usecase

import (
	"context"
	"errors"
	"fmt"

	"auth-service/internal/config"
	"auth-service/internal/domain"
	"auth-service/internal/repository"
	"auth-service/pkg/jwt"
	"auth-service/pkg/password"

	"github.com/google/uuid"

	redisClient "auth-service/pkg/redis"
)

// authUsecase adalah implementasi konkret dari interface AuthUsecase
// Struct ini berisi semua dependencies yang dibutuhkan untuk authentication business logic
type authUsecase struct {
	userRepo         repository.UserRepository         // Repository untuk akses data user
	refreshTokenRepo repository.RefreshTokenRepository // Repository untuk manage refresh token
	activityRepo     repository.UserActivityRepository // Repository untuk log aktivitas user
	roleRepo         repository.RoleRepository         // Repository untuk role & permission
	tokenService     *jwt.TokenService                 // Service untuk generate & validate JWT
	config           *config.Config                    // Konfigurasi aplikasi (JWT expiry, dll)
	redisClient      *redisClient.Client
}

// NewAuthUsecase adalah constructor untuk membuat instance authUsecase
// Semua dependencies di-inject dari luar (Dependency Injection pattern)
//
// Parameters:
//   - userRepo: Repository untuk CRUD operations user
//   - refreshTokenRepo: Repository untuk manage refresh token di database
//   - activityRepo: Repository untuk log user activity
//   - roleRepo: Repository untuk role & permission
//   - tokenService: Service untuk JWT operations
//   - cfg: Application configuration
//
// Returns:
//   - AuthUsecase: Interface (bukan concrete struct) untuk loose coupling
func NewAuthUsecase(
	userRepo repository.UserRepository,
	refreshTokenRepo repository.RefreshTokenRepository,
	activityRepo repository.UserActivityRepository,
	roleRepo repository.RoleRepository,
	tokenService *jwt.TokenService,
	redisClient *redisClient.Client,
	cfg *config.Config,
) AuthUsecase {
	return &authUsecase{
		userRepo:         userRepo,
		refreshTokenRepo: refreshTokenRepo,
		activityRepo:     activityRepo,
		roleRepo:         roleRepo,
		tokenService:     tokenService,
		redisClient:      redisClient,
		config:           cfg,
	}
}

// Register melakukan registrasi user baru ke sistem
// Business logic: validasi username & email unique, hash password, create user, assign role default
//
// Flow:
//  1. Validasi username belum dipakai
//  2. Validasi email belum terdaftar
//  3. Hash password menggunakan bcrypt
//  4. Generate unique user ID
//  5. Simpan user ke database
//  6. Assign role "user" sebagai default role
//  7. Log activity registration
//
// Parameters:
//   - ctx: Context untuk cancellation & timeout
//   - req: Data user baru (username, email, password, fullname)
//   - ip: IP address user untuk logging
//   - userAgent: User agent dari browser untuk logging
//   - deviceInfo: Info device (mobile/desktop) untuk logging
//
// Returns:
//   - *domain.UserResponse: Data user yang sudah dibuat (tanpa password)
//   - error: ErrUsernameExists jika username sudah ada, ErrEmailExists jika email sudah terdaftar
func (u *authUsecase) Register(ctx context.Context, req *domain.UserCreate, ip, userAgent, deviceInfo string) (*domain.UserResponse, error) {
	// Step 1: Cek apakah username sudah dipakai user lain
	_, err := u.userRepo.FindByUsername(ctx, req.Username)
	if err == nil {
		// User found - username already exists
		return nil, domain.ErrUsernameAlreadyExists
	}
	if !errors.Is(err, domain.ErrUserNotFound) {
		// Database error (not "not found")
		return nil, err
	}

	// Step 2: Cek apakah email sudah terdaftar
	_, err = u.userRepo.FindByEmail(ctx, req.Email)
	if err == nil {
		// User found - email already exists
		return nil, domain.ErrEmailAlreadyExists
	}
	if !errors.Is(err, domain.ErrUserNotFound) {
		// Database error (not "not found")
		return nil, err
	}

	// Step 3: Hash password menggunakan bcrypt agar aman disimpan di database
	hashedPassword, err := password.Hash(req.Password)
	if err != nil {
		return nil, fmt.Errorf("failed to hash password: %v", err)
	}

	// Step 4: Generate unique user ID menggunakan UUID
	userID := uuid.New()

	// Step 5: Buat entity user dengan data dari request
	user := &domain.User{
		ID:             userID,
		Username:       req.Username,
		Email:          req.Email,
		HashedPassword: hashedPassword,
		FullName:       req.FullName,
		IsActive:       true, // User baru otomatis aktif
	}

	// Step 6: Simpan user ke database via repository
	if err := u.userRepo.Create(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to create user: %w", err)
	}

	// Step 7: Assign role "user" sebagai default role untuk user baru
	role, err := u.roleRepo.FindByName(ctx, "user")
	if err == nil {
		// Role found - assign to user
		_ = u.roleRepo.AssignToUser(ctx, user.ID, role.ID)
	}
	// If role not found or error, just skip (user created without role)

	// Step 8: Log aktivitas registrasi untuk audit trail
	u.logActivity(ctx, user.ID.String(), "account_created", "New account registered", ip, userAgent, deviceInfo)

	// Return user response tanpa password (security best practice)
	return u.toUserResponse(user), nil
}

// Login melakukan autentikasi user dan generate JWT tokens
// Business logic: validasi credentials, generate access & refresh token, store refresh token
//
// Flow:
//  1. Cari user berdasarkan username atau email
//  2. Validasi user aktif (tidak di-disable)
//  3. Verify password menggunakan bcrypt
//  4. Generate access token (short-lived, untuk API access)
//  5. Generate refresh token (long-lived, untuk renew access token)
//  6. Simpan refresh token ke database (untuk token rotation & revocation)
//  7. Update last login timestamp
//  8. Log aktivitas login
//
// Parameters:
//   - ctx: Context untuk cancellation & timeout
//   - req: Login credential (username/email dan password)
//   - ip: IP address untuk security logging
//   - userAgent: Browser/app info untuk device tracking
//   - deviceInfo: Parsed device info (Mobile/Desktop - Browser)
//
// Returns:
//   - *domain.TokenResponse: Response berisi access token & metadata
//   - string: Refresh token (dikirim via HttpOnly cookie, tidak di response body)
//   - error: ErrInvalidCredentials jika user/pass salah, ErrAccountDisabled jika akun di-disable
func (u *authUsecase) Login(ctx context.Context, req *domain.LoginRequest, ip, userAgent, deviceInfo string) (*domain.TokenResponse, string, error) {
	// Step 1: Cari user berdasarkan username ATAU email (fleksibel)
	user, err := u.userRepo.FindByUsernameOrEmail(ctx, req.Username)
	if err != nil {
		// Return generic error - don't reveal whether user exists
		return nil, "", domain.ErrInvalidCredentials
	}

	// Step 2: Cek apakah akun user masih aktif (bisa di-disable oleh admin)
	if !user.IsActive {
		return nil, "", domain.ErrAccountInactive
	}

	// Step 3: Verify password menggunakan bcrypt.CompareHashAndPassword
	if !password.Verify(req.Password, user.HashedPassword) {
		return nil, "", domain.ErrInvalidCredentials
	}

	// Extract role names dari user.Roles
	roleNames := make([]string, len(user.Roles))
	for i, role := range user.Roles {
		roleNames[i] = role.Name
	}

	// Step 4: Generate access token (JWT dengan expiry pendek, misal 15 menit)
	accessToken, _, err := u.tokenService.GenerateAccessToken(user.ID.String(), user.Username, user.Email, roleNames)
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate access token: %w", err)
	}

	// Step 5: Generate refresh token (JWT dengan expiry panjang, misal 7 hari)
	// Refresh token di-hash sebelum disimpan di database (security best practice)
	refreshToken, refreshHash, refreshExpiry, err := u.tokenService.GenerateRefreshToken(user.ID.String())
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate refresh token: %w", err)
	}

	// Step 6: Simpan refresh token hash ke database untuk token rotation & revocation
	// Kita simpan hash, bukan token asli (kalau database breach, token tetap aman)
	rtEntity := &domain.RefreshToken{
		TokenHash:  refreshHash, // Hash dari refresh token
		UserID:     user.ID,
		ExpiresAt:  refreshExpiry,
		IsValid:    true,       // Flag untuk invalidate token (logout)
		DeviceInfo: deviceInfo, // Track device user untuk security audit
		IPAddress:  ip,         // Track IP untuk detect suspicious activity
	}

	if err := u.refreshTokenRepo.Create(ctx, rtEntity); err != nil {
		return nil, "", fmt.Errorf("failed to store refresh token: %w", err)
	}

	// Step 7: Update last login timestamp untuk tracking
	_ = u.userRepo.UpdateLastLogin(ctx, user.ID)

	// Step 8: Log login activity untuk audit trail
	u.logActivity(ctx, user.ID.String(), "login", "User logged in", ip, userAgent, deviceInfo)

	// Step 9: Return response
	// Access token dikirim di response body (untuk Authorization header)
	// Refresh token dikirim via HttpOnly cookie (untuk security)
	return &domain.TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: "", // Tidak dikembalikan di body, dikirim via cookie
		TokenType:    "bearer",
		ExpiresIn:    int64(u.tokenService.GetAccessTokenExpiry().Seconds()),
	}, refreshToken, nil
}

// RefreshToken melakukan refresh access token menggunakan refresh token
// Business logic: validate refresh token, invalidate old token, generate new tokens (token rotation)
//
// Token Rotation adalah security best practice:
//   - Setiap kali refresh, token lama di-invalidate
//   - Generate token baru untuk replace token lama
//   - Kalau ada attacker curi token, token cuma valid sekali pakai
//
// Flow:
//  1. Validate refresh token JWT (signature, expiry, claims)
//  2. Verify token exists di database & belum di-invalidate
//  3. Verify user exists & aktif
//  4. Invalidate old refresh token (token rotation security)
//  5. Generate new access token & refresh token
//  6. Simpan new refresh token ke database
//  7. Log activity
//
// Parameters:
//   - ctx: Context untuk cancellation & timeout
//   - refreshToken: Refresh token dari HttpOnly cookie
//   - ip: IP address untuk logging
//   - userAgent: User agent untuk device tracking
//   - deviceInfo: Parsed device info
//
// Returns:
//   - *domain.TokenResponse: New access token
//   - string: New refresh token (untuk replace old token di cookie)
//   - error: ErrInvalidToken jika token invalid/expired/already used
func (u *authUsecase) RefreshToken(ctx context.Context, refreshToken, ip, userAgent, deviceInfo string) (*domain.TokenResponse, string, error) {
	// Step 1: Validate JWT refresh token (signature, expiry, format)
	userIDStr, err := u.tokenService.ValidateRefreshToken(refreshToken)
	if err != nil {
		return nil, "", domain.ErrRefreshTokenInvalid
	}

	// Parse userID string to UUID
	userID, err := uuid.Parse(userIDStr)
	if err != nil {
		return nil, "", domain.ErrRefreshTokenInvalid
	}

	// Step 2: Cek apakah token ada di database & masih valid (belum di-invalidate)
	// Ini untuk prevent token yg sudah dipakai ulang (replay attack)
	tokenHash := jwt.HashToken(refreshToken)

	// Check apakah token sudah ada di blacklist (redis)
	if u.redisClient.IsTokenBlacklisted(ctx, tokenHash) {
		return nil, "", errors.New("refresh token has been revoked")
	}

	storedToken, err := u.refreshTokenRepo.FindByHash(ctx, tokenHash)
	if err != nil || storedToken == nil {
		return nil, "", domain.ErrRefreshTokenInvalid // Token tidak ditemukan atau sudah dihapus
	}

	// Step 3: Verify user masih exists & aktif
	// Use GetUserWithRoles to ensure roles are preloaded for the new access token
	user, err := u.userRepo.GetUserWithRoles(ctx, userID)
	if err != nil {
		return nil, "", domain.ErrRefreshTokenInvalid // User dihapus atau di-disable
	}
	if !user.IsActive {
		return nil, "", domain.ErrRefreshTokenInvalid // User disabled
	}

	// Step 4: Invalidate old refresh token (TOKEN ROTATION)
	// Token lama tidak bisa dipakai lagi, harus pakai token baru
	_ = u.refreshTokenRepo.Invalidate(ctx, tokenHash)

	// Extract role names dari user.Roles
	roleNames := make([]string, len(user.Roles))
	for i, role := range user.Roles {
		roleNames[i] = role.Name
	}

	// Step 5: Generate new access token
	accessToken, _, err := u.tokenService.GenerateAccessToken(user.ID.String(), user.Username, user.Email, roleNames)
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate access token: %w", err)
	}

	// Step 6: Generate new refresh token (replace old token)
	newRefreshToken, newRefreshHash, refreshExpiry, err := u.tokenService.GenerateRefreshToken(user.ID.String())
	if err != nil {
		return nil, "", fmt.Errorf("failed to generate refresh token: %w", err)
	}

	// Step 7: Simpan new refresh token ke database
	rtEntity := &domain.RefreshToken{
		TokenHash:  newRefreshHash,
		UserID:     user.ID,
		ExpiresAt:  refreshExpiry,
		IsValid:    true,
		DeviceInfo: deviceInfo,
		IPAddress:  ip,
	}

	if err := u.refreshTokenRepo.Create(ctx, rtEntity); err != nil {
		return nil, "", fmt.Errorf("failed to store refresh token: %w", err)
	}

	// Step 8: Log refresh activity untuk audit
	u.logActivity(ctx, user.ID.String(), "token_refresh", "Token Refreshed", ip, userAgent, deviceInfo)

	// Return new tokens
	return &domain.TokenResponse{
		AccessToken:  accessToken,
		RefreshToken: "", // Not in response body
		TokenType:    "bearer",
		ExpiresIn:    int64(u.tokenService.GetAccessTokenExpiry().Seconds()),
	}, newRefreshToken, nil
}

// Logout melakukan logout user dari device saat ini
// Business logic: invalidate refresh token untuk device ini saja
//
// Flow:
//  1. Invalidate refresh token dari device saat ini (kalau ada)
//  2. Log logout activity
//
// Note: User masih login di device lain (kalau ada multiple sessions)
// Kalau mau logout dari semua device, pakai LogoutAll()
//
// Parameters:
//   - ctx: Context untuk cancellation & timeout
//   - userID: ID user yang logout
//   - refreshToken: Refresh token dari cookie (optional, bisa kosong)
//
// Returns:
//   - error: Selalu nil, logout tidak bisa gagal
func (u *authUsecase) Logout(ctx context.Context, userID, refreshToken string) error {
	// Invalidate refresh token untuk device ini (kalau ada)
	if refreshToken != "" {
		tokenHash := jwt.HashToken(refreshToken)
		_ = u.refreshTokenRepo.Invalidate(ctx, tokenHash) // Ignore error, best effort

		// Blacklist token di Redis (instant invalidation)
		// TTL = refresh token expiry (misal 7 hari)
		// Setelah 7 hari, token udah expired dan Redis auto-delete key
		ttl := u.config.JWT.RefreshTokenExpiry
		_ = u.redisClient.BlacklistToken(ctx, tokenHash, ttl)
	}

	// Log logout activity
	u.logActivity(ctx, userID, "logout", "User logged out", "", "", "")
	return nil
}

// LogoutAll melakukan logout user dari SEMUA device/session
// Business logic: invalidate SEMUA refresh token milik user
//
// Use case:
//   - User ganti password (force logout semua device untuk security)
//   - User klik "Logout dari semua perangkat"
//   - Suspicious activity detected (admin force logout user)
//
// Flow:
//  1. Invalidate semua refresh token milik user di database
//  2. Log logout activity
//
// Parameters:
//   - ctx: Context untuk cancellation & timeout
//   - userID: ID user yang mau di-logout dari semua device
//
// Returns:
//   - error: Error jika gagal invalidate tokens di database
func (u *authUsecase) LogoutAll(ctx context.Context, userID string) error {
	// Parse userID string to UUID
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("invalid user ID: %w", err)
	}

	// Invalidate SEMUA refresh token milik user di database
	// Setelah ini, user harus login ulang di semua device
	if err := u.refreshTokenRepo.InvalidateAllForUser(ctx, userUUID); err != nil {
		return fmt.Errorf("failed to invalidate tokens: %w", err)
	}

	// Delete session cache dari Redis
	// Ini memaksa semua devices untuk re-authenticate dan melihat token sudah invalid
	_ = u.redisClient.DeleteUserSession(ctx, userID)

	// Log activity
	u.logActivity(ctx, userID, "logout_all_devices", "User logged out from all devices", "", "", "")
	return nil
}

// GetCurrentUser mengambil data user yang sedang login
// Business logic: get user info + roles + permissions untuk ditampilkan di UI
//
// Endpoint: GET /auth/me
// Dipakai untuk:
//   - Display user profile di navbar
//   - Check user permissions di frontend
//   - Verify user masih login & aktif
//
// Flow:
//  1. Get user data dengan preload roles
//  2. Get all permissions user dari roles (aggregated)
//  3. Extract role names untuk display
//  4. Return complete user info + roles + permissions
//
// Parameters:
//   - ctx: Context untuk cancellation & timeout
//   - userID: ID user dari JWT token (sudah terverifikasi di middleware)
//
// Returns:
//   - *domain.UserWithPermissionsResponse: User data + roles + permissions
//   - error: ErrUserNotFound jika user tidak ada (edge case: user dihapus setelah token issued)
func (u *authUsecase) GetCurrentUser(ctx context.Context, userID string) (*domain.UserWithPermissionsResponse, error) {
	// Parse userID string to UUID
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	// Get user data dengan preload roles (JOIN query untuk efficiency)
	user, err := u.userRepo.GetUserWithRoles(ctx, userUUID)
	if err != nil || user == nil {
		return nil, domain.ErrUserNotFound
	}

	// Get all permissions dari semua roles user (aggregated & distinct)
	// Misal user punya role "admin" dan "moderator", ambil semua permissions dari kedua role
	permissions, _ := u.userRepo.GetUserPermissions(ctx, userUUID)

	// Extract role names untuk response ("admin", "user", "moderator")
	var roleNames []string
	for _, role := range user.Roles {
		roleNames = append(roleNames, role.Name)
	}

	// Build response dengan user info + roles + permissions
	return &domain.UserWithPermissionsResponse{
		UserResponse: *u.toUserResponse(user), // User data (tanpa password)
		Permissions:  permissions,             // ["users:read", "users:write", "posts:create", ...]
		Roles:        roleNames,               // ["admin", "moderator"]
	}, nil
}

// GetUserByID mengambil data user berdasarkan ID (admin only)
// Endpoint: GET /internal/users/:user_id
// Dipakai untuk:
//   - Display user detail di admin panel
//   - Get user status (is_active) untuk management
//
// Flow:
//  1. Parse userID string to UUID
//  2. Get user data dari database
//  3. Return user response (tanpa password)
//
// Parameters:
//   - ctx: Context untuk cancellation & timeout
//   - userID: ID user yang mau dilihat
//
// Returns:
//   - *domain.UserResponse: User data tanpa password
//   - error: ErrUserNotFound jika user tidak ada
func (u *authUsecase) GetUserByID(ctx context.Context, userID string) (*domain.UserResponse, error) {
	// Parse userID string to UUID
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	// Get user data
	user, err := u.userRepo.FindByID(ctx, userUUID)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			return nil, domain.ErrUserNotFound
		}
		return nil, err
	}

	// Return user response (tanpa password dan permissions)
	return u.toUserResponse(user), nil
}

// UpdateProfile melakukan update profil user (fullname, email)
// Business logic: validate perubahan, cek email unique, update data, log changes
//
// Endpoint: PUT /auth/profile
// User hanya bisa update profile sendiri (bukan user lain)
//
// Flow:
//  1. Get user data saat ini
//  2. Track perubahan apa saja yang dilakukan
//  3. Validate fullname jika berubah
//  4. Validate email unique jika berubah (dan log email change)
//  5. Return error jika tidak ada perubahan (save database query)
//  6. Update user di database
//  7. Log profile update activity
//
// Parameters:
//   - ctx: Context untuk cancellation & timeout
//   - userID: ID user yang mau update profile
//   - req: Data update (fullname, email) - field yang kosong tidak diupdate
//   - ip, userAgent, deviceInfo: Untuk logging
//
// Returns:
//   - *domain.UserResponse: User data yang sudah diupdate
//   - error: ErrUserNotFound, ErrEmailExists, atau ErrNoChanges
func (u *authUsecase) UpdateProfile(ctx context.Context, userID string, req *domain.UserUpdate, ip, userAgent, deviceInfo string) (*domain.UserResponse, error) {
	// Parse userID string to UUID
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	// Get user data saat ini
	user, err := u.userRepo.FindByID(ctx, userUUID)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			return nil, domain.ErrUserNotFound
		}
		return nil, err
	}

	// Track fields apa aja yang berubah (untuk logging)
	var changes []string

	// Update username jika ada perubahan
	if req.Username != "" && req.Username != user.Username {
		// Validate username belum dipakai user lain
		existing, err := u.userRepo.FindByUsername(ctx, req.Username)
		if err == nil && existing.ID != user.ID {
			return nil, domain.ErrUsernameAlreadyExists // Username sudah terdaftar
		}
		// Ignore ErrUserNotFound (username available)
		user.Username = req.Username
		changes = append(changes, "username")
	}

	// Update fullname jika ada perubahan
	if req.FullName != "" && req.FullName != user.FullName {
		user.FullName = req.FullName
		changes = append(changes, "full_name")
	}

	// Update email jika ada perubahan
	if req.Email != "" && req.Email != user.Email {
		// Validate email belum dipakai user lain
		existing, err := u.userRepo.FindByEmail(ctx, req.Email)
		if err == nil && existing.ID != user.ID {
			return nil, domain.ErrEmailAlreadyExists // Email sudah terdaftar
		}
		// Ignore ErrUserNotFound (email available)
		user.Email = req.Email
		changes = append(changes, "email")
		// Email change adalah perubahan sensitive, log terpisah
		u.logActivity(ctx, userID, "email_change", fmt.Sprintf("Email changed to %s", req.Email), ip, userAgent, deviceInfo)
	}

	// Validasi ada perubahan atau tidak
	// Jika tidak ada perubahan, return error untuk save database write
	if len(changes) == 0 {
		return nil, domain.ErrNoChanges
	}

	// Update user di database
	if err := u.userRepo.Update(ctx, user); err != nil {
		return nil, fmt.Errorf("failed to update user: %w", err)
	}

	// Log profile update dengan detail fields yang berubah
	u.logActivity(ctx, userID, "profile_update", fmt.Sprintf("Profile updated: %v", changes), ip, userAgent, deviceInfo)

	// Return user data yang sudah diupdate (tanpa password)
	return u.toUserResponse(user), nil
}

// ChangePassword melakukan perubahan password user
// Business logic: validate old password, hash new password, logout all sessions
//
// Security best practices:
//   - Verify current password dulu (prevent unauthorized password change)
//   - Check new password tidak sama dengan old password
//   - Hash password dengan bcrypt sebelum simpan
//   - Force logout semua device setelah ganti password (invalidate all tokens)
//
// Flow:
//  1. Get user data
//  2. Verify current password benar
//  3. Validate new password tidak sama dengan current password
//  4. Hash new password dengan bcrypt
//  5. Update password di database
//  6. Invalidate SEMUA refresh tokens (force logout semua device)
//  7. Log password change activity
//
// Parameters:
//   - ctx: Context untuk cancellation & timeout
//   - userID: ID user yang mau ganti password
//   - req: Request berisi current password & new password
//   - ip, userAgent, deviceInfo: Untuk logging
//
// Returns:
//   - error: ErrPasswordMismatch jika current password salah, ErrSamePassword jika password sama
func (u *authUsecase) ChangePassword(ctx context.Context, userID string, req *domain.ChangePasswordRequest, ip, userAgent, deviceInfo string) error {
	// Parse userID string to UUID
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("invalid user ID: %w", err)
	}

	// Get user data
	user, err := u.userRepo.FindByID(ctx, userUUID)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			return domain.ErrUserNotFound
		}
		return err
	}

	// Step 1: Verify current password benar (security: prevent unauthorized change)
	if !password.Verify(req.CurrentPassword, user.HashedPassword) {
		return domain.ErrPasswordMismatch // Password lama salah
	}

	// Step 2: Validate new password tidak sama dengan current password
	if req.CurrentPassword == req.NewPassword {
		return domain.ErrSamePassword // Password baru harus berbeda
	}

	// Step 3: Hash new password dengan bcrypt (computationally expensive, tapi aman)
	hashedPassword, err := password.Hash(req.NewPassword)
	if err != nil {
		return fmt.Errorf("failed to hash password: %w", err)
	}

	// Step 4: Update password di database
	user.HashedPassword = hashedPassword

	if err := u.userRepo.Update(ctx, user); err != nil {
		return fmt.Errorf("failed to update password: %w", err)
	}

	// Step 5: PENTING! Invalidate all refresh tokens untuk security
	// User harus login ulang di semua device dengan password baru
	// Ini prevent kalau ada attacker yang sudah punya token lama
	_ = u.refreshTokenRepo.InvalidateAllForUser(ctx, userUUID)

	// Step 6: Log password change untuk audit trail (sensitive action!)
	u.logActivity(ctx, userID, "password_change", "Password changed", ip, userAgent, deviceInfo)

	return nil
}

// GetActivities mengambil daftar aktivitas user dengan filtering & pagination
// Endpoint: GET /auth/activities?action=login&limit=50&offset=0
//
// Use case:
//   - Display activity log di profile page
//   - Security audit: lihat login history, suspicious activity
//   - Track user behavior untuk analytics
//
// Parameters:
//   - ctx: Context untuk cancellation & timeout
//   - userID: ID user yang mau dilihat activity-nya
//   - query: Filter (action, start_date, end_date) dan pagination (limit, offset)
//
// Returns:
//   - []domain.ActivityResponse: List aktivitas user (sorted by created_at DESC)
//   - error: Error jika gagal query database
func (u *authUsecase) GetActivities(ctx context.Context, userID string, query *domain.ActivityQuery) ([]domain.ActivityResponse, error) {
	// Parse userID string to UUID
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	// Get activities dari database dengan filter & pagination
	activities, err := u.activityRepo.FindByUserID(ctx, userUUID, query)
	if err != nil {
		return nil, err
	}

	// Transform entity ke DTO response
	return u.toActivityResponses(activities), nil
}

// GetActivitySummary mengambil summary aktivitas user dalam N hari terakhir
// Endpoint: GET /auth/activities/summary?days=30
//
// Returns statistics seperti:
//   - Total activities
//   - Login count
//   - Failed login attempts
//   - Most active day
//
// Parameters:
//   - ctx: Context untuk cancellation & timeout
//   - userID: ID user
//   - days: Berapa hari ke belakang (1-365)
//
// Returns:
//   - *domain.ActivitySummary: Statistik aktivitas user
//   - error: Error jika gagal query
func (u *authUsecase) GetActivitySummary(ctx context.Context, userID string, days int) (*domain.ActivitySummary, error) {
	// Parse userID string to UUID
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	// Delegate ke repository (complex aggregation query)
	return u.activityRepo.GetSummary(ctx, userUUID, days)
}

// GetRecentActivities mengambil N aktivitas terakhir user
// Endpoint: GET /auth/activities/recent?limit=10
//
// Use case:
//   - Display "Recent activity" widget di dashboard
//   - Quick view login history tanpa pagination
//
// Parameters:
//   - ctx: Context untuk cancellation & timeout
//   - userID: ID user
//   - limit: Max jumlah activities (1-50)
//
// Returns:
//   - []domain.ActivityResponse: List aktivitas (newest first)
//   - error: Error jika gagal query
func (u *authUsecase) GetRecentActivities(ctx context.Context, userID string, limit int) ([]domain.ActivityResponse, error) {
	// Parse userID string to UUID
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	// Get recent activities (sorted by newest first)
	activities, err := u.activityRepo.GetRecentByUserID(ctx, userUUID, limit)
	if err != nil {
		return nil, err
	}

	// Transform entity ke DTO
	return u.toActivityResponses(activities), nil
}

// UpdateUserStatus is used by admin to update user active status (activate/deactivate user)
// Business logic: validate target user exists, prevent deactivating self, update status, clear cache
//
// Endpoint: PUT /api/v1/internal/users/:user_id/status (Admin only)
// Use case:
//   - Deactivate user account (soft delete - user can't login but data preserved)
//   - Reactivate previously deactivated user
//   - Security: Disable compromised accounts
//
// Flow:
//  1. Validate target user exists
//  2. Prevent admin from deactivating themselves
//  3. Check if status actually changed
//  4. Update user is_active field
//  5. If deactivated, invalidate all user sessions (force logout)
//  6. Clear user session cache
//  7. Log activity for audit trail
//
// Parameters:
//   - ctx: Context for cancellation & timeout
//   - targetUserID: ID of user whose status will be updated
//   - isActive: New status (true = activate, false = deactivate)
//   - adminID: ID of admin performing this action (for logging)
//   - ip, userAgent, deviceInfo: For activity logging
//
// Returns:
//   - error: ErrUserNotFound if target user doesn't exist, ErrCannotDeactivateSelf if admin tries to deactivate themselves
func (u *authUsecase) UpdateUserStatus(ctx context.Context, targetUserID string, isActive bool, adminID, ip, userAgent, deviceInfo string) error {
	// Parse target userID string to UUID
	targetUUID, err := uuid.Parse(targetUserID)
	if err != nil {
		return fmt.Errorf("invalid target user ID: %w", err)
	}

	// Step 1: Get target user data to verify exists
	user, err := u.userRepo.FindByID(ctx, targetUUID)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			return domain.ErrUserNotFound
		}
		return err
	}

	// Step 2: Prevent admin from deactivating themselves (security)
	if targetUserID == adminID && !isActive {
		return errors.New("cannot deactivate your own account")
	}

	// Step 3: Check if status actually changed (avoid unnecessary DB write)
	if user.IsActive == isActive {
		return domain.ErrNoChanges
	}

	// Step 4: Update user is_active field
	user.IsActive = isActive

	if err := u.userRepo.Update(ctx, user); err != nil {
		return fmt.Errorf("failed to update user status: %w", err)
	}

	// Step 5: If user is deactivated, invalidate all their refresh tokens (force logout)
	if !isActive {
		_ = u.refreshTokenRepo.InvalidateAllForUser(ctx, targetUUID)
	}

	// Step 6: Clear user session cache from Redis
	_ = u.redisClient.DeleteUserSession(ctx, targetUserID)

	// Step 7: Log activity for audit trail
	action := "user_activated"
	description := fmt.Sprintf("User %s activated by admin %s", user.Username, adminID)
	if !isActive {
		action = "user_deactivated"
		description = fmt.Sprintf("User %s deactivated by admin %s", user.Username, adminID)
	}

	// Log activity for both admin and target user
	u.logActivity(ctx, targetUserID, action, description, ip, userAgent, deviceInfo)
	u.logActivity(ctx, adminID, fmt.Sprintf("admin_%s", action), description, ip, userAgent, deviceInfo)

	return nil
}

// DeleteUser permanently deletes a user and all associated data (hard delete)
// Business logic: validate target user exists, prevent self-deletion, cleanup sessions, delete user
//
// Endpoint: DELETE /api/v1/internal/users/:user_id (Admin only)
// Use case:
//   - Permanently remove a user account and all their data
//   - GDPR compliance: right to be forgotten
//   - Cleanup test/spam accounts
//
// Flow:
//  1. Validate target user exists
//  2. Prevent admin from deleting themselves
//  3. Invalidate all refresh tokens (force logout)
//  4. Clear user session cache from Redis
//  5. Log admin activity (before delete, since user data will be gone)
//  6. Delete user from database (CASCADE deletes related records)
//
// Parameters:
//   - ctx: Context for cancellation & timeout
//   - targetUserID: ID of user to be deleted
//   - adminID: ID of admin performing this action (for logging)
//   - ip, userAgent, deviceInfo: For activity logging
//
// Returns:
//   - error: ErrUserNotFound if target doesn't exist, ErrCannotDeleteSelf if admin tries to delete themselves
func (u *authUsecase) DeleteUser(ctx context.Context, targetUserID, adminID, ip, userAgent, deviceInfo string) error {
	// Parse target userID string to UUID
	targetUUID, err := uuid.Parse(targetUserID)
	if err != nil {
		return fmt.Errorf("invalid target user ID: %w", err)
	}

	// Step 1: Verify target user exists
	user, err := u.userRepo.FindByID(ctx, targetUUID)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			return domain.ErrUserNotFound
		}
		return err
	}

	// Step 2: Prevent admin from deleting themselves
	if targetUserID == adminID {
		return domain.ErrCannotDeleteSelf
	}

	// Step 3: Invalidate all refresh tokens for the user (force logout)
	_ = u.refreshTokenRepo.InvalidateAllForUser(ctx, targetUUID)

	// Step 4: Clear user session cache from Redis
	_ = u.redisClient.DeleteUserSession(ctx, targetUserID)

	// Step 5: Log admin activity BEFORE delete (user data will be gone after delete)
	description := fmt.Sprintf("User %s (ID: %s) permanently deleted by admin %s", user.Username, targetUserID, adminID)
	u.logActivity(ctx, adminID, "admin_user_deleted", description, ip, userAgent, deviceInfo)

	// Step 6: Delete user from database (CASCADE will auto-delete RefreshTokens, Activities, user_roles)
	if err := u.userRepo.Delete(ctx, targetUUID); err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	return nil
}

// ========== HELPER FUNCTIONS ==========

// Helper functions untuk internal use, tidak di-expose ke interface

// logActivity adalah helper function untuk log user activity ke database
// Dipakai di semua function yang perlu audit trail (login, logout, password change, dll)
//
// Activity log dipakai untuk:
//   - Security audit trail
//   - Detect suspicious behavior (multiple failed login, login dari IP/device baru)
//   - User analytics (when user most active, what features they use)
//   - Compliance requirements (GDPR, SOC2, dll butuh activity log)
//
// Parameters:
//   - ctx: Context (bisa timeout jika log terlalu lama)
//   - userID: ID user yang melakukan activity
//   - action: Jenis activity ("login", "logout", "password_change", "profile_update", dll)
//   - description: Detail activity (free text, bisa kosong)
//   - ip: IP address user (untuk detect location change)
//   - userAgent: Browser/app info
//   - deviceInfo: Parsed device ("Mobile - Chrome", "Desktop - Firefox")
//
// Note: Function ini ignore error (best effort logging, tidak block main flow)
func (u *authUsecase) logActivity(ctx context.Context, userID, action, description, ip, userAgent, deviceInfo string) {
	// Parse userID string to UUID (best effort, ignore error)
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return // Skip logging if invalid UUID
	}

	// Build activity entity
	activity := &domain.UserActivity{
		UserID:      userUUID,
		Action:      action, // "login", "logout", "password_change", dll
		Description: description,
		IPAddress:   ip,         // Untuk detect login dari IP berbeda
		UserAgent:   userAgent,  // Raw user agent string
		DeviceInfo:  deviceInfo, // Parsed: "Mobile - Chrome"
	}
	// Ignore error - logging tidak boleh block main flow
	// Kalau logging fail, operation tetap sukses
	_ = u.activityRepo.Create(ctx, activity)
}

// toUserResponse adalah helper function untuk transform User entity ke UserResponse DTO
// Function ini REMOVE sensitive data (password, internal fields) sebelum dikirim ke client
//
// Security best practice:
//   - Jangan return password (even hashed) ke client
//   - Jangan return internal metadata yang tidak perlu
//   - Only return fields yang memang dibutuhkan frontend
//
// Parameters:
//   - user: User entity dari database (dengan semua fields)
//
// Returns:
//   - *domain.UserResponse: DTO tanpa sensitive data, siap dikirim ke client
func (u *authUsecase) toUserResponse(user *domain.User) *domain.UserResponse {
	return &domain.UserResponse{
		ID:        user.ID.String(), // Convert UUID to string for JSON
		Email:     user.Email,
		Username:  user.Username,
		FullName:  user.FullName,
		IsActive:  user.IsActive,  // Untuk display status (active/disabled)
		CreatedAt: user.CreatedAt, // Kapan user register
		UpdatedAt: user.UpdatedAt, // Kapan terakhir update profile
		LastLogin: user.LastLogin, // Kapan terakhir login
		// NOTE: HashedPassword TIDAK di-include! (security)
	}
}

// toActivityResponses adalah helper function untuk transform list UserActivity entities ke ActivityResponse DTOs
// Batch transformation untuk efficiency (daripada loop transform satu-satu)
//
// Parameters:
//   - activities: Slice UserActivity entities dari database
//
// Returns:
//   - []domain.ActivityResponse: Slice DTOs siap dikirim ke client
func (u *authUsecase) toActivityResponses(activities []domain.UserActivity) []domain.ActivityResponse {
	// Pre-allocate slice dengan size yang sama untuk efficiency
	responses := make([]domain.ActivityResponse, len(activities))

	// Transform setiap entity ke DTO
	for i, a := range activities {
		responses[i] = domain.ActivityResponse{
			ID:          a.ID.String(), // Convert UUID to string for JSON
			Action:      a.Action,      // "login", "logout", dll
			Description: a.Description, // Detail activity
			IPAddress:   a.IPAddress,   // IP yang dipakai saat activity
			DeviceInfo:  a.DeviceInfo,  // Device yang dipakai
			CreatedAt:   a.CreatedAt,   // Timestamp activity
		}
	}
	return responses
}
