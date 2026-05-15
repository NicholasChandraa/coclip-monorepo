package handler

import (
	"fmt"
	"net/http"
	"strconv"
	"strings"

	"auth-service/internal/config"
	"auth-service/internal/domain"
	"auth-service/internal/middleware"
	"auth-service/internal/usecase"
	"auth-service/pkg/logger"

	"github.com/gin-gonic/gin"

	redisClient "auth-service/pkg/redis"
)

// AuthHandler adalah HTTP handler layer untuk endpoint authentication
// Layer ini bertanggung jawab untuk:
// - Parse HTTP request (JSON, query params, headers)
// - Validasi input dari user
// - Call business logic di usecase layer
// - Format response dalam bentuk JSON
// - Set HTTP status code yang sesuai
// - Handle cookies (untuk refresh token)
type AuthHandler struct {
	authUsecase usecase.AuthUsecase // Interface usecase untuk business logic (loose coupling)
	config      *config.Config      // Config untuk cookie settings, JWT, dll
	redisClient *redisClient.Client
}

// NewAuthHandler adalah constructor untuk membuat instance AuthHandler
// Dependencies di-inject dari luar (DI pattern)
//
// Parameters:
//   - authUsecase: Business logic layer untuk authentication
//   - cfg: Application configuration
//
// Returns:
//   - *AuthHandler: Pointer ke struct handler (concrete type, karena dipakai di router)
func NewAuthHandler(authUsecase usecase.AuthUsecase, cfg *config.Config, redisClient *redisClient.Client) *AuthHandler {
	return &AuthHandler{
		authUsecase: authUsecase,
		config:      cfg,
		redisClient: redisClient,
	}
}

// Register adalah HTTP handler untuk endpoint registrasi user baru
// Endpoint: POST /auth/register
// Request Body: JSON dengan struktur UserCreate (username, email, password, fullname)
// Response: 201 Created dengan data user, atau 400/409/500 dengan error message
//
// Flow:
//  1. Parse & validate request body JSON
//  2. Extract metadata (IP, user agent, device info) untuk logging
//  3. Call usecase.Register() untuk business logic
//  4. Map error dari usecase ke HTTP status code
//  5. Return JSON response
func (h *AuthHandler) Register(c *gin.Context) {
	log := logger.GetLoggerFromGinContext(c)

	// Step 1: Parse request body ke struct UserCreate & validasi
	var req domain.UserCreate
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warn().Err(err).Msg("Invalid registration request")
		// Validation error (required fields, format, dll) → 400 Bad Request
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{
			Detail: FormatValidationError(err),
		})
		return
	}

	// Step 2: Extract metadata dari HTTP request untuk audit logging
	ip := h.getClientIP(c)                     // IP address dari X-Forwarded-For atau Remote-Addr
	userAgent := c.Request.UserAgent()         // Browser/app info dari header
	deviceInfo := h.parseDeviceInfo(userAgent) //Parsed device: "Mobile - Chrome", "Desktop - Firefox", dll

	// Step 3: Call business logic di usecase layer
	user, err := h.authUsecase.Register(c.Request.Context(), &req, ip, userAgent, deviceInfo)
	if err != nil {
		log.Error().Err(err).Str("username", req.Username).Msg("Registration failed")
		// Step 4: Map business logic error ke HTTP status code
		httpErr := MapError(err)
		c.JSON(httpErr.Status, domain.ErrorResponse{
			Detail: httpErr.Message,
		})
		return
	}

	// Step 5: Success response dengan status 201 Created
	c.JSON(http.StatusCreated, user)
}

// Login adalah HTTP handler untuk endpoint autentikasi user
// Endpoint: POST /auth/token
// Request Body: JSON dengan struktur LoginRequest (username/email, password)
// Response: 200 OK dengan access token, refresh token dikirim via HttpOnly cookie
//
// Flow:
//  1. Parse & validate request body
//  2. Extract metadata untuk security logging
//  3. Call usecase.Login() untuk authentication
//  4. Set refresh token sebagai HttpOnly cookie (security best practice)
//  5. Return access token di response body
func (h *AuthHandler) Login(c *gin.Context) {
	log := logger.GetLoggerFromGinContext(c)

	// Step 1: Parse request body (bisa dari JSON atau form data)
	var req domain.LoginRequest
	if err := c.ShouldBind(&req); err != nil {
		log.Warn().Err(err).Msg("Invalid login request")
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{
			Detail: FormatValidationError(err),
		})
		return
	}

	// Step 2: Extract metadata untuk security audit
	ip := h.getClientIP(c)
	userAgent := c.Request.UserAgent()
	deviceInfo := h.parseDeviceInfo(userAgent)

	// Step 3: Authenticate user & generate tokens
	tokenResp, refreshToken, err := h.authUsecase.Login(c.Request.Context(), &req, ip, userAgent, deviceInfo)
	if err != nil {
		log.Error().Err(err).Str("username", req.Username).Msg("Login failed")
		// Map error ke HTTP status code
		httpErr := MapError(err)
		c.JSON(httpErr.Status, domain.ErrorResponse{
			Detail: httpErr.Message,
		})
		return
	}

	// Step 4: Set tokens sebagai HttpOnly cookie
	h.setRefreshTokenCookie(c, refreshToken)
	h.setAccessTokenCookie(c, tokenResp.AccessToken)

	// Step 5: Return access token di response body juga (untuk backward compat)
	c.JSON(http.StatusOK, tokenResp)
}

// RefreshToken adalah handler untuk refresh access token
// Endpoint: POST /auth/refresh
// Refresh token diambil dari HttpOnly cookie (bukan body) untuk keamanan
//
// Flow:
//  1. Ambil refresh token dari cookie
//  2. Validate token & generate token baru (dengan token rotation)
//  3. Set refresh token baru sebagai cookie
//  4. Return access token baru
func (h *AuthHandler) RefreshToken(c *gin.Context) {
	log := logger.GetLoggerFromGinContext(c)

	// Ambil refresh token dari HttpOnly cookie
	refreshToken, err := c.Cookie("refresh_token")
	if err != nil {
		log.Warn().Msg("Refresh token not found in cookie")
		c.JSON(http.StatusUnauthorized, domain.ErrorResponse{
			Detail: "Refresh token not found",
		})
		return
	}

	// Extract metadata untuk logging
	ip := h.getClientIP(c)
	userAgent := c.Request.UserAgent()
	deviceInfo := h.parseDeviceInfo(userAgent)

	// Validate & generate token baru (token rotation: token lama di-revoke)
	tokenResp, newRefreshToken, err := h.authUsecase.RefreshToken(c.Request.Context(), refreshToken, ip, userAgent, deviceInfo)
	if err != nil {
		log.Error().Err(err).Msg("Token refresh failed")
		h.clearRefreshTokenCookie(c) // Clear cookie jika gagal
		c.JSON(http.StatusUnauthorized, domain.ErrorResponse{
			Detail: err.Error(),
		})
		return
	}

	// Set refresh token baru (token rotation) + access token cookie baru
	h.setRefreshTokenCookie(c, newRefreshToken)
	h.setAccessTokenCookie(c, tokenResp.AccessToken)

	c.JSON(http.StatusOK, tokenResp)
}

// Logout adalah handler untuk logout user dari device saat ini
// Endpoint: POST /auth/logout
// Memerlukan authentication (access token di header)
//
// Flow:
//  1. Ambil user ID dari context (dari auth middleware)
//  2. Revoke refresh token di database
//  3. Clear cookie di browser
func (h *AuthHandler) Logout(c *gin.Context) {
	userID := middleware.GetUserID(c)

	// Ambil refresh token untuk di-revoke
	refreshToken, _ := c.Cookie("refresh_token")

	// Revoke refresh token (error diabaikan, cookie tetap harus di-clear)
	_ = h.authUsecase.Logout(c.Request.Context(), userID, refreshToken)

	// Delete session cache dari Redis
	_ = h.redisClient.DeleteUserSession(c.Request.Context(), userID)

	// Clear cookies di browser
	h.clearRefreshTokenCookie(c)
	h.clearAccessTokenCookie(c)

	c.JSON(http.StatusOK, domain.MessageResponse{
		Detail: "Successfully logged out",
	})
}

// LogoutAll adalah handler untuk logout user dari semua device
// Endpoint: POST /auth/logout-all
// Berguna jika user curiga ada device lain yang memakai akunnya
//
// Flow:
//  1. Revoke SEMUA refresh token user di database
//  2. Clear cookie di browser saat ini
func (h *AuthHandler) LogoutAll(c *gin.Context) {
	log := logger.GetLoggerFromGinContext(c)
	userID := middleware.GetUserID(c)

	// Revoke semua refresh token user
	if err := h.authUsecase.LogoutAll(c.Request.Context(), userID); err != nil {
		log.Error().Err(err).Str("user_id", userID).Msg("Logout all failed")
		c.JSON(http.StatusInternalServerError, domain.ErrorResponse{
			Detail: err.Error(),
		})
		return
	}

	// Delete session cache dari Redis ← TAMBAHKAN INI
	_ = h.redisClient.DeleteUserSession(c.Request.Context(), userID)

	// Clear cookies di browser saat ini
	h.clearRefreshTokenCookie(c)
	h.clearAccessTokenCookie(c)

	c.JSON(http.StatusOK, domain.MessageResponse{
		Detail: "Successfully logged out from all devices",
	})
}

// GetCurrentUser adalah handler untuk mengambil data user yang sedang login
// Endpoint: GET /auth/me
// Return data user lengkap dengan roles dan permissions
//
// OPTIMIZED: Uses cached session from AuthMiddleware instead of DB query
// AuthMiddleware already fetched & cached user data → reuse it!
func (h *AuthHandler) GetCurrentUser(c *gin.Context) {
	log := logger.GetLoggerFromGinContext(c)
	userID := middleware.GetUserID(c)

	// Get user object dari middleware context (already cached!)
	userInterface, exists := c.Get(middleware.UserKey)
	if !exists {
		log.Error().Str("user_id", userID).Msg("User not found in context (middleware issue)")
		c.JSON(http.StatusInternalServerError, domain.ErrorResponse{
			Detail: "Internal server error",
		})
		return
	}

	user, ok := userInterface.(*domain.User)
	if !ok {
		log.Error().Str("user_id", userID).Msg("Invalid user type in context")
		c.JSON(http.StatusInternalServerError, domain.ErrorResponse{
			Detail: "Internal server error",
		})
		return
	}

	// Get roles and permissions from Redis session cache
	session, cacheHit := h.redisClient.GetUserSession(c.Request.Context(), userID)

	var roles []string
	var permissions []string

	if cacheHit && session != nil {
		// Cache HIT - use cached roles & permissions (99% of requests)
		roles = session.Roles
		permissions = session.Permissions
		log.Debug().Str("user_id", userID).Msg("User data served from cache")
	} else {
		// Cache MISS - extremely rare (cache expired between middleware and handler)
		// This shouldn't happen under normal circumstances since middleware refreshes cache
		// Fallback: Query DB using existing GetCurrentUser usecase
		log.Warn().Str("user_id", userID).Msg("Session cache miss in GetCurrentUser - falling back to DB")

		userWithPerms, err := h.authUsecase.GetCurrentUser(c.Request.Context(), userID)
		if err != nil {
			log.Error().Err(err).Str("user_id", userID).Msg("Failed to get user from DB fallback")
			// Return empty as last resort
			roles = []string{}
			permissions = []string{}
		} else {
			// Extract roles and permissions from DB response
			// Both are already []string, just assign directly
			roles = userWithPerms.Roles
			permissions = userWithPerms.Permissions
		}
	}

	// Build response dengan data dari cache
	response := map[string]any{
		"id":          user.ID.String(),
		"username":    user.Username,
		"email":       user.Email,
		"full_name":   user.FullName,
		"is_active":   user.IsActive,
		"roles":       roles,
		"permissions": permissions,
	}

	c.JSON(http.StatusOK, response)
}

// UpdateProfile adalah handler untuk update profile user
// Endpoint: PUT /auth/profile
// Request Body: JSON dengan field yang mau diupdate (fullname, email, dll)
//
// Flow:
//  1. Parse & validate request body
//  2. Update profile di database
//  3. Log activity untuk audit trail
func (h *AuthHandler) UpdateProfile(c *gin.Context) {
	log := logger.GetLoggerFromGinContext(c)
	userID := middleware.GetUserID(c)

	var req domain.UserUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warn().Err(err).Str("user_id", userID).Msg("Invalid update profile request")
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{
			Detail: FormatValidationError(err),
		})
		return
	}

	// Extract metadata untuk activity logging
	ip := h.getClientIP(c)
	userAgent := c.Request.UserAgent()
	deviceInfo := h.parseDeviceInfo(userAgent)

	user, err := h.authUsecase.UpdateProfile(c.Request.Context(), userID, &req, ip, userAgent, deviceInfo)
	if err != nil {
		log.Error().Err(err).Str("user_id", userID).Msg("Failed to update profile")
		httpErr := MapError(err)
		c.JSON(httpErr.Status, domain.ErrorResponse{
			Detail: httpErr.Message,
		})
		return
	}

	// Delete session cache (user data might have changed) ← TAMBAHKAN INI
	_ = h.redisClient.DeleteUserSession(c.Request.Context(), userID)

	c.JSON(http.StatusOK, user)
}

// ChangePassword adalah handler untuk ganti password user
// Endpoint: POST /auth/change-password
// Request Body: JSON dengan current_password dan new_password
//
// Flow:
//  1. Validate password lama
//  2. Update password baru (di-hash dengan bcrypt)
//  3. Clear refresh token (force re-login untuk keamanan)
func (h *AuthHandler) ChangePassword(c *gin.Context) {
	log := logger.GetLoggerFromGinContext(c)
	userID := middleware.GetUserID(c)

	var req domain.ChangePasswordRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warn().Err(err).Str("user_id", userID).Msg("Invalid change password request")
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{
			Detail: FormatValidationError(err),
		})
		return
	}

	// Extract metadata untuk activity logging
	ip := h.getClientIP(c)
	userAgent := c.Request.UserAgent()
	deviceInfo := h.parseDeviceInfo(userAgent)

	err := h.authUsecase.ChangePassword(c.Request.Context(), userID, &req, ip, userAgent, deviceInfo)
	if err != nil {
		log.Error().Err(err).Str("user_id", userID).Msg("Failed to change password")
		httpErr := MapError(err)
		c.JSON(httpErr.Status, domain.ErrorResponse{
			Detail: httpErr.Message,
		})
		return
	}

	// Delete session cache (user data might have changed) ← TAMBAHKAN INI
	_ = h.redisClient.DeleteUserSession(c.Request.Context(), userID)

	// Clear cookies (force re-login setelah ganti password)
	h.clearRefreshTokenCookie(c)
	h.clearAccessTokenCookie(c)

	c.JSON(http.StatusOK, domain.MessageResponse{
		Detail: "Password changed successfully",
	})
}

// GetActivities adalah handler untuk mengambil daftar activity user
// Endpoint: GET /auth/activities
// Query params: limit, offset, activity_type (optional filter)
// Berguna untuk menampilkan riwayat login, logout, password change, dll
func (h *AuthHandler) GetActivities(c *gin.Context) {
	log := logger.GetLoggerFromGinContext(c)
	userID := middleware.GetUserID(c)

	// Parse query parameters
	var query domain.ActivityQuery
	if err := c.ShouldBindQuery(&query); err != nil {
		log.Warn().Err(err).Str("user_id", userID).Msg("Invalid activity query params")
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{
			Detail: FormatValidationError(err),
		})
		return
	}

	// Set default & max limit untuk pagination
	if query.Limit == 0 {
		query.Limit = 50
	}
	if query.Limit > 100 {
		query.Limit = 100
	}

	activities, err := h.authUsecase.GetActivities(c.Request.Context(), userID, &query)
	if err != nil {
		log.Error().Err(err).Str("user_id", userID).Msg("Failed to get activities")
		c.JSON(http.StatusInternalServerError, domain.ErrorResponse{
			Detail: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, activities)
}

// GetActivitySummary adalah handler untuk mengambil ringkasan activity user
// Endpoint: GET /auth/activities/summary
// Query params: days (default 30, max 365)
// Return: total login, failed login, dll dalam periode tertentu
func (h *AuthHandler) GetActivitySummary(c *gin.Context) {
	log := logger.GetLoggerFromGinContext(c)
	userID := middleware.GetUserID(c)

	// Parse days parameter dengan default 30 hari
	days, _ := strconv.Atoi(c.DefaultQuery("days", "30"))

	// Validasi range days (1-365)
	if days < 1 {
		days = 1
	}
	if days > 365 {
		days = 365
	}

	summary, err := h.authUsecase.GetActivitySummary(c.Request.Context(), userID, days)
	if err != nil {
		log.Error().Err(err).Str("user_id", userID).Msg("Failed to get activity summary")
		c.JSON(http.StatusInternalServerError, domain.ErrorResponse{
			Detail: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, summary)
}

// GetRecentActivities adalah handler untuk mengambil activity terbaru user
// Endpoint: GET /auth/activities/recent
// Query params: limit (default 10, max 50)
// Shortcut untuk GetActivities dengan sorting terbaru
func (h *AuthHandler) GetRecentActivities(c *gin.Context) {
	log := logger.GetLoggerFromGinContext(c)
	userID := middleware.GetUserID(c)

	// Parse limit parameter dengan default 10
	limit, _ := strconv.Atoi(c.DefaultQuery("limit", "10"))

	// Validasi range limit (1-50)
	if limit < 1 {
		limit = 1
	}
	if limit > 50 {
		limit = 50
	}

	activities, err := h.authUsecase.GetRecentActivities(c.Request.Context(), userID, limit)
	if err != nil {
		log.Error().Err(err).Str("user_id", userID).Msg("Failed to get recent activities")
		c.JSON(http.StatusInternalServerError, domain.ErrorResponse{
			Detail: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, activities)
}

// UpdateUserStatus adalah handler untuk admin update user active status
// Endpoint: PUT /api/v1/internal/users/:user_id/status
// Request Body: JSON dengan is_active
// Admin only - untuk activate/deactivate user account
//
// Flow:
//  1. Parse user_id dari URL parameter
//  2. Parse & validate request body
//  3. Get current admin user ID dari context
//  4. Call usecase.UpdateUserStatus()
//  5. Return success message
func (h *AuthHandler) UpdateUserStatus(c *gin.Context) {
	log := logger.GetLoggerFromGinContext(c)
	adminID := middleware.GetUserID(c) // Admin yang melakukan aksi ini

	// Parse target user ID dari URL parameter
	targetUserID := c.Param("user_id")
	if targetUserID == "" {
		log.Warn().Msg("User ID parameter is missing")
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{
			Detail: "User ID is required",
		})
		return
	}

	// Parse request body
	var req domain.UpdateUserStatusRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warn().Err(err).Str("user_id", targetUserID).Msg("Invalid update status request")
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{
			Detail: FormatValidationError(err),
		})
		return
	}

	// Extract metadata untuk logging
	ip := h.getClientIP(c)
	userAgent := c.Request.UserAgent()
	deviceInfo := h.parseDeviceInfo(userAgent)

	// Call usecase untuk update status
	err := h.authUsecase.UpdateUserStatus(c.Request.Context(), targetUserID, req.IsActive, adminID, ip, userAgent, deviceInfo)
	if err != nil {
		log.Error().Err(err).Str("target_user_id", targetUserID).Str("admin_id", adminID).Msg("Failed to update user status")
		httpErr := MapError(err)
		c.JSON(httpErr.Status, domain.ErrorResponse{
			Detail: httpErr.Message,
		})
		return
	}

	// Success response
	statusText := "deactivated"
	if req.IsActive {
		statusText = "activated"
	}

	c.JSON(http.StatusOK, domain.MessageResponse{
		Detail: fmt.Sprintf("User successfully %s", statusText),
	})
}

// GetUserByID adalah handler untuk admin get user detail by ID
// Endpoint: GET /api/v1/internal/users/:user_id
// Admin only - untuk view user detail termasuk is_active status
//
// Flow:
//  1. Parse user_id dari URL parameter
//  2. Call usecase.GetUserByID()
//  3. Return user data
func (h *AuthHandler) GetUserByID(c *gin.Context) {
	log := logger.GetLoggerFromGinContext(c)

	// Parse user ID dari URL parameter
	userID := c.Param("user_id")
	if userID == "" {
		log.Warn().Msg("User ID parameter is missing")
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{
			Detail: "User ID is required",
		})
		return
	}

	// Call usecase untuk get user detail
	user, err := h.authUsecase.GetUserByID(c.Request.Context(), userID)
	if err != nil {
		log.Error().Err(err).Str("user_id", userID).Msg("Failed to get user by ID")
		httpErr := MapError(err)
		c.JSON(httpErr.Status, domain.ErrorResponse{
			Detail: httpErr.Message,
		})
		return
	}

	// Success response
	c.JSON(http.StatusOK, user)
}

// DeleteUser adalah handler untuk admin menghapus user secara permanen (hard delete)
// Endpoint: DELETE /api/v1/internal/users/:user_id
// Admin only - menghapus user beserta semua data terkait
//
// Flow:
//  1. Get admin ID dari middleware context
//  2. Parse user_id dari URL parameter
//  3. Extract metadata (IP, user agent, device info)
//  4. Call usecase.DeleteUser()
//  5. Return success message
func (h *AuthHandler) DeleteUser(c *gin.Context) {
	log := logger.GetLoggerFromGinContext(c)
	adminID := middleware.GetUserID(c)

	// Parse target user ID dari URL parameter
	targetUserID := c.Param("user_id")
	if targetUserID == "" {
		log.Warn().Msg("User ID parameter is missing")
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{
			Detail: "User ID is required",
		})
		return
	}

	// Extract metadata untuk logging
	ip := h.getClientIP(c)
	userAgent := c.Request.UserAgent()
	deviceInfo := h.parseDeviceInfo(userAgent)

	// Call usecase untuk delete user
	err := h.authUsecase.DeleteUser(c.Request.Context(), targetUserID, adminID, ip, userAgent, deviceInfo)
	if err != nil {
		log.Error().Err(err).Str("target_user_id", targetUserID).Str("admin_id", adminID).Msg("Failed to delete user")
		httpErr := MapError(err)
		c.JSON(httpErr.Status, domain.ErrorResponse{
			Detail: httpErr.Message,
		})
		return
	}

	c.JSON(http.StatusOK, domain.MessageResponse{
		Detail: "User permanently deleted",
	})
}

// ========== Helper Functions ==========

// setRefreshTokenCookie set refresh token sebagai HttpOnly cookie
// HttpOnly = tidak bisa diakses JavaScript (XSS protection)
// Secure = hanya dikirim via HTTPS
// SameSite = CSRF protection
func (h *AuthHandler) setRefreshTokenCookie(c *gin.Context, token string) {
	maxAge := int(h.config.JWT.RefreshTokenExpiry.Seconds())

	// Parse SameSite dari config
	sameSite := http.SameSiteLaxMode
	switch h.config.Cookie.SameSite {
	case "Strict":
		sameSite = http.SameSiteStrictMode
	case "None":
		sameSite = http.SameSiteNoneMode
	}

	c.SetSameSite(sameSite)
	c.SetCookie(
		"refresh_token",
		token,
		maxAge,
		h.config.Cookie.Path,
		h.config.Cookie.Domain,
		h.config.Cookie.Secure,
		true, // HttpOnly = true untuk keamanan
	)
}

// clearRefreshTokenCookie menghapus refresh token cookie dari browser
// Set MaxAge = -1 untuk menghapus cookie
func (h *AuthHandler) clearRefreshTokenCookie(c *gin.Context) {
	c.SetCookie(
		"refresh_token",
		"",
		-1, // MaxAge -1 = hapus cookie
		h.config.Cookie.Path,
		h.config.Cookie.Domain,
		h.config.Cookie.Secure,
		true,
	)
}

// setAccessTokenCookie set access token sebagai HttpOnly cookie
func (h *AuthHandler) setAccessTokenCookie(c *gin.Context, token string) {
	maxAge := int(h.config.JWT.AccessTokenExpiry.Seconds())

	sameSite := http.SameSiteLaxMode
	switch h.config.Cookie.SameSite {
	case "Strict":
		sameSite = http.SameSiteStrictMode
	case "None":
		sameSite = http.SameSiteNoneMode
	}

	c.SetSameSite(sameSite)
	c.SetCookie(
		"access_token",
		token,
		maxAge,
		h.config.Cookie.Path,
		h.config.Cookie.Domain,
		h.config.Cookie.Secure,
		true,
	)
}

// clearAccessTokenCookie menghapus access token cookie dari browser
func (h *AuthHandler) clearAccessTokenCookie(c *gin.Context) {
	c.SetCookie(
		"access_token",
		"",
		-1,
		h.config.Cookie.Path,
		h.config.Cookie.Domain,
		h.config.Cookie.Secure,
		true,
	)
}

// getClientIP mengambil IP address client dari request
// Prioritas: X-Forwarded-For → X-Real-IP → ClientIP
// X-Forwarded-For dipakai jika aplikasi di belakang reverse proxy (nginx, cloudflare, dll)
func (h *AuthHandler) getClientIP(c *gin.Context) string {
	// Cek X-Forwarded-For header (dari reverse proxy)
	if xff := c.GetHeader("X-Forwarded-For"); xff != "" {
		ips := strings.Split(xff, ",")
		if len(ips) > 0 {
			return strings.TrimSpace(ips[0]) // Ambil IP pertama (client asli)
		}
	}

	// Cek X-Real-IP (alternatif dari nginx)
	if xri := c.GetHeader("X-Real-IP"); xri != "" {
		return xri
	}

	// Fallback ke ClientIP dari Gin
	return c.ClientIP()
}

// parseDeviceInfo mengekstrak informasi device dari User-Agent header
// Return format: "Device - Browser" (contoh: "Mobile - Chrome", "Desktop - Firefox")
// Berguna untuk activity logging dan security monitoring
func (h *AuthHandler) parseDeviceInfo(userAgent string) string {
	ua := strings.ToLower(userAgent)

	// Deteksi tipe device
	var device string
	switch {
	case strings.Contains(ua, "mobile"):
		device = "Mobile"
	case strings.Contains(ua, "tablet"):
		device = "Tablet"
	default:
		device = "Desktop"
	}

	// Deteksi browser
	var browser string
	switch {
	case strings.Contains(ua, "chrome"):
		browser = "Chrome"
	case strings.Contains(ua, "firefox"):
		browser = "Firefox"
	case strings.Contains(ua, "safari"):
		browser = "Safari"
	case strings.Contains(ua, "edge"):
		browser = "Edge"
	default:
		browser = "Unknown"
	}

	return device + " - " + browser
}
