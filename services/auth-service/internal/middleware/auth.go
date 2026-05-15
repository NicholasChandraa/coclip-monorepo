package middleware

import (
	"context"
	"errors"
	"net/http"
	"slices"
	"strings"

	"auth-service/internal/domain"
	"auth-service/internal/repository"
	"auth-service/pkg/jwt"
	"auth-service/pkg/logger"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"

	redisClient "auth-service/pkg/redis"
)

// Context keys untuk menyimpan data user di Gin Context
// Keys ini dipakai untuk pass data dari middleware ke handler
const (
	UserIDKey   = "user_id"  // Key untuk user ID (string)
	UsernameKey = "username" // Key untuk username (string)
	EmailKey    = "email"    // Key untuk email (string)
	UserKey     = "user"     // Key untuk full user object (*domain.User)
)

// AuthMiddleware adalah middleware untuk validasi JWT access token
// Middleware ini akan:
//  1. Extract token dari Authorization header (format: "Bearer <token>")
//  2. Validate JWT token (signature, expiry, claims)
//  3. Verify user exists di database & masih aktif
//  4. Inject user info ke Gin Context untuk dipakai handler
//  5. Update last activity user (async)
//
// Middleware ini WAJIB untuk protected endpoints!
// Kalau token invalid/expired/missing → request di-reject dengan 401 Unauthorized
//
// Usage di router:
//
//	protected := router.Group("/auth")
//	protected.Use(AuthMiddleware(tokenService, userRepo))
//	protected.GET("/me", handler.GetCurrentUser) // ← butuh valid token
func AuthMiddleware(tokenService *jwt.TokenService, userRepo repository.UserRepository, redisCache *redisClient.Client) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Step 1: Cek Authorization header ada atau tidak
		authHeader := c.GetHeader("Authorization")
		if authHeader == "" {
			c.JSON(http.StatusUnauthorized, domain.ErrorResponse{
				Detail: "Authorization header is required",
			})
			c.Abort() // Stop request, jangan lanjut ke handler
			return
		}

		// Step 2: Validasi format header: "Bearer <token>"
		parts := strings.SplitN(authHeader, " ", 2)
		if len(parts) != 2 || strings.ToLower(parts[0]) != "bearer" {
			c.JSON(http.StatusUnauthorized, domain.ErrorResponse{
				Detail: "Invalid authorization header format",
			})
			c.Abort()
			return
		}

		tokenString := parts[1] // Extract token dari "Bearer <token>"

		// Step 3: Validate JWT token (signature, expiry, dll)
		claims, err := tokenService.ValidateAccessToken(tokenString)
		if err != nil {
			status := http.StatusUnauthorized
			message := "Invalid token"

			// Kasih pesan lebih spesifik kalau token expired
			if err == jwt.ErrExpiredToken {
				message = "Token has expired"
			}

			c.JSON(status, domain.ErrorResponse{
				Detail: message,
			})
			c.Abort()
			return
		}

		// Step 4: Parse userID string to UUID
		userUUID, err := uuid.Parse(claims.UserID)
		if err != nil {
			c.JSON(http.StatusUnauthorized, domain.ErrorResponse{
				Detail: "Invalid user ID format",
			})
			c.Abort()
			return
		}

		// Step 5: Check session cache first (redis)
		session, cacheHit := redisCache.GetUserSession(c.Request.Context(), claims.UserID)
		var user *domain.User

		if cacheHit && session != nil {
			// CACHE HIT - tidak query ke database
			logger.Debug().Str("user_id", claims.UserID).Msg("✅ Session cache HIT - serving from Redis")

			// Validate user masih aktif dari cache
			if !session.IsActive {
				c.JSON(http.StatusUnauthorized, domain.ErrorResponse{
					Detail: "Account is disabled",
				})
				c.Abort()
				return
			}

			// Reconstruct user object dari cache
			user = &domain.User{
				ID:       userUUID,
				Username: session.Username,
				Email:    session.Email,
				FullName: session.FullName,
				IsActive: session.IsActive,
			}

			// Note: Roles & Permissions tidak perlu di-reconstruct ke user object
			// RequireRole middleware langsung pakai session.Roles
			// RequirePermission middleware langsung pakai session.Permissions
		} else {
			// CACHE MISS - query database (avec roles untuk session cache)
			logger.Warn().Str("user_id", claims.UserID).Msg("❌ Session cache MISS - querying database")

			user, err = userRepo.GetUserWithRoles(c.Request.Context(), userUUID)
			if err != nil {
				c.JSON(http.StatusUnauthorized, domain.ErrorResponse{
					Detail: "User not found",
				})
				c.Abort()
				return
			}

			// Validate user aktif
			if !user.IsActive {
				c.JSON(http.StatusUnauthorized, domain.ErrorResponse{
					Detail: "Account is disabled",
				})
				c.Abort()
				return
			}

			// Save to cache untuk request berikutnya
			roleNames := make([]string, len(user.Roles))
			for i, role := range user.Roles {
				roleNames[i] = role.Name
			}

			// Get user permissions untuk cache
			permissions, _ := userRepo.GetUserPermissions(c.Request.Context(), userUUID)

			session := &redisClient.UserSession{
				ID:          user.ID.String(),
				Username:    user.Username,
				Email:       user.Email,
				FullName:    user.FullName,
				IsActive:    user.IsActive,
				Roles:       roleNames,
				Permissions: permissions,
			}

			// Async cache save
			go func() {
				_ = redisCache.SetUserSession(context.Background(), session)
			}()
		}

		// Step 6: Update last activity timestamp (async agar tidak blocking request)
		// Use context.Background() karena goroutine harus continue after request selesai
		// Request context akan di-cancel setelah response dikirim → can't use c.Request.Context()
		go func() {
			_ = userRepo.UpdateLastActivity(context.Background(), userUUID)
		}()

		// Step 6: Inject user info ke Gin Context
		// Data ini bisa diambil di handler menggunakan GetUserID(), GetUsername(), dll
		c.Set(UserIDKey, claims.UserID)
		c.Set(UsernameKey, claims.Username)
		c.Set(EmailKey, claims.Email)
		c.Set(UserKey, user)

		// Step 7: Lanjutkan ke handler berikutnya di chain
		c.Next()
	}
}

// RequirePermission adalah middleware untuk cek apakah user punya permission tertentu
// Harus dipakai SETELAH AuthMiddleware (butuh user_id di context)
//
// Use case:
//   - Endpoint yang butuh permission spesifik, misal "users:write", "roles:manage"
//
// Response:
//   - 401 Unauthorized: jika user belum login
//   - 403 Forbidden: jika user tidak punya permission
func RequirePermission(userRepo repository.UserRepository, redisCache *redisClient.Client, permission string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user ID from context
		userID := GetUserID(c)
		if userID == "" {
			c.JSON(http.StatusUnauthorized, domain.ErrorResponse{
				Detail: "Unauthorized",
			})
			c.Abort()
			return
		}

		// Check session cache first (includes permissions)
		session, cacheHit := redisCache.GetUserSession(c.Request.Context(), userID)

		var hasPermission bool
		if cacheHit && session != nil {
			// Cache HIT - check permission dari session cache
			hasPermission = slices.Contains(session.Permissions, permission)
		} else {
			// Cache MISS - query database
			userUUID, err := uuid.Parse(userID)
			if err != nil {
				c.JSON(http.StatusUnauthorized, domain.ErrorResponse{
					Detail: "Invalid user ID",
				})
				c.Abort()
				return
			}

			// Check permission di DB
			hasPermission, err = userRepo.HasPermission(c.Request.Context(), userUUID, permission)
			if err != nil {
				c.JSON(http.StatusInternalServerError, domain.ErrorResponse{
					Detail: "Failed to check permissions",
				})
				c.Abort()
				return
			}
			// Note: Session will be created on next AuthMiddleware hit
		}

		if !hasPermission {
			c.JSON(http.StatusForbidden, domain.ErrorResponse{
				Detail: "Insufficient permissions",
			})
			c.Abort()
			return
		}
		c.Next()
	}
}

// RequireRole adalah middleware untuk cek apakah user punya salah satu role yang dibutuhkan
// Harus dipakai SETELAH AuthMiddleware (butuh user_id di context)
//
// Parameters:
//   - roles: variadic string, user harus punya SALAH SATU dari roles ini
//
// Usage:
//
//	router.GET("/admin", RequireRole(roleRepo, redisCache, "admin"))
//	router.GET("/staff", RequireRole(roleRepo, redisCache, "admin", "moderator"))
//
// Ini bisa jadi tidak digunakan apabila di projek ini pake RequirePermission
func RequireRole(roleRepo repository.RoleRepository, redisCache *redisClient.Client, roles ...string) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Get user ID from context
		userID := GetUserID(c)
		if userID == "" {
			c.JSON(http.StatusUnauthorized, domain.ErrorResponse{
				Detail: "Unauthorized",
			})
			c.Abort()
			return
		}

		// Check session cache first (includes roles)
		session, cacheHit := redisCache.GetUserSession(c.Request.Context(), userID)

		var hasRole bool
		if cacheHit && session != nil {
			// Cache HIT - check role dari session cache
			for _, userRole := range session.Roles {
				for _, requiredRole := range roles {
					if userRole == requiredRole {
						hasRole = true
						break
					}
				}
				if hasRole {
					break
				}
			}
		} else {
			// Cache MISS - query database
			userUUID, err := getUserUUID(c)
			if err != nil {
				c.JSON(http.StatusUnauthorized, domain.ErrorResponse{
					Detail: "Unauthorized",
				})
				c.Abort()
				return
			}

			// Ambil semua roles user dari database
			userRoles, err := roleRepo.GetUserRoles(c.Request.Context(), userUUID)
			if err != nil {
				c.JSON(http.StatusInternalServerError, domain.ErrorResponse{
					Detail: "Failed to check roles",
				})
				c.Abort()
				return
			}

			// Cek apakah user punya salah satu role yang dibutuhkan
			for _, userRole := range userRoles {
				if slices.Contains(roles, userRole.Name) {
					hasRole = true
				}
				if hasRole {
					break
				}
			}
			// Note: Session will be created on next AuthMiddleware hit
		}

		if !hasRole {
			c.JSON(http.StatusForbidden, domain.ErrorResponse{
				Detail: "Insufficient role",
			})
			c.Abort()
			return
		}

		c.Next()
	}
}

// ========== Helper Functions ==========
// Helper functions untuk mengambil data user dari Gin Context
// Data ini di-set oleh AuthMiddleware setelah validasi token berhasil

// getUserUUID mengambil dan parse user UUID dari context
// Return UUID dan error jika user tidak ditemukan atau format invalid
func getUserUUID(c *gin.Context) (uuid.UUID, error) {
	userID, exists := c.Get(UserIDKey)
	if !exists {
		return uuid.Nil, errors.New("user not found in context")
	}
	return uuid.Parse(userID.(string))
}

// GetUserID mengambil user ID dari context
// Return empty string jika user belum login
func GetUserID(c *gin.Context) string {
	if userID, exists := c.Get(UserIDKey); exists {
		return userID.(string)
	}
	return ""
}

// GetUsername mengambil username dari context
// Return empty string jika user belum login
func GetUsername(c *gin.Context) string {
	if username, exists := c.Get(UsernameKey); exists {
		return username.(string)
	}
	return ""
}
