package handler

import (
	"auth-service/internal/config"
	"auth-service/internal/domain"
	"auth-service/internal/middleware"
	"auth-service/internal/repository"
	"auth-service/pkg/jwt"
	"net/http"

	"github.com/gin-gonic/gin"

	redisClient "auth-service/pkg/redis"
)

// Router adalah struct yang mengatur semua HTTP routes dan middleware
// Router menghubungkan URL endpoints dengan handler functions
type Router struct {
	engine            *gin.Engine               // Gin engine instance
	authHandler       *AuthHandler              // Handler untuk auth endpoints
	roleHandler       *RoleHandler              // Handler untuk role endpoints
	permissionHandler *PermissionHandler        // Handler untuk permission endpoints
	internalHandler   *InternalHandler          // Handler untuk internal endpoints
	socialHandler     *SocialHandler            // Handler untuk social OAuth endpoints
	tokenService      *jwt.TokenService         // Service untuk validasi JWT
	userRepo          repository.UserRepository // Repository untuk lookup user di middleware
	roleRepo          repository.RoleRepository // Repository untuk check role di middleware
	redisClient       *redisClient.Client       // Redis Client unuk konfigurasi redis
	config            *config.Config            // App config
}

// NewRouter membuat instance Router baru dengan semua dependencies
// Gin engine dibuat di sini dengan Recovery dan Logger middleware
func NewRouter(
	authHandler *AuthHandler,
	roleHandler *RoleHandler,
	permissionHandler *PermissionHandler,
	internalHandler *InternalHandler,
	socialHandler *SocialHandler,
	tokenService *jwt.TokenService,
	userRepo repository.UserRepository,
	roleRepo repository.RoleRepository,
	redisClient *redisClient.Client,
	cfg *config.Config,
) *Router {
	// Set Gin mode (debug/release/test)
	gin.SetMode(cfg.Server.Mode)

	// Create Gin engine dengan default middleware
	engine := gin.New()
	engine.Use(gin.Recovery())                   // Recover dari panic, return 500
	engine.Use(gin.Logger())                     // Log semua requests
	engine.Use(middleware.RequestIDMiddleware()) // Inject request ID & logger

	return &Router{
		engine:            engine,
		authHandler:       authHandler,
		roleHandler:       roleHandler,
		permissionHandler: permissionHandler,
		internalHandler:   internalHandler,
		socialHandler:     socialHandler,
		tokenService:      tokenService,
		userRepo:          userRepo,
		roleRepo:          roleRepo,
		redisClient:       redisClient,
		config:            cfg,
	}
}

// Setup mengkonfigurasi semua routes dan middleware, lalu return Gin engine
// Dipanggil di main.go setelah Router dibuat
//
// Route groups:
//   - /health: Health check (public)
//   - /api/v1/auth/*: Authentication endpoints
//   - /api/v1/roles/*: Role management endpoints
//   - /api/v1/internal/*: Internal service endpoints
func (r *Router) Setup() *gin.Engine {
	// CORS middleware untuk handle cross-origin requests
	r.engine.Use(middleware.CORSMiddleware(r.config))

	// Health check endpoint (untuk load balancer & monitoring)
	r.engine.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, domain.HealthResponse{
			Status:  "healthy",
			Service: "auth-service",
		})
	})

	// API v1 group
	v1 := r.engine.Group("/api/v1")

	// Setup route groups
	r.setupAuthRoutes(v1)       // /auth/*
	r.setupRoleRoutes(v1)       // /roles/*
	r.setupPermissionRoutes(v1) // /permissions/*
	r.setupInternalRoutes(v1)   // /internal/*
	r.setupSocialRoutes(v1)     // /social/*

	return r.engine
}

// setupAuthRoutes mengkonfigurasi routes untuk authentication
// Base path: /auth
//
// Public routes (tidak perlu login):
//   - POST /auth/register: Registrasi user baru
//   - POST /auth/token: Login (get access token)
//   - POST /auth/refresh: Refresh access token
//
// Protected routes (perlu login):
//   - POST /auth/logout: Logout dari device ini
//   - POST /auth/logout-all: Logout dari semua device
//   - GET /auth/me: Get current user info
//   - PUT /auth/profile: Update profile
//   - POST /auth/change-password: Ganti password
//   - GET /auth/activities: Get activity history
//   - GET /auth/thinking-mode: Get thinking mode status
//   - PUT /auth/thinking-mode: Update thinking mode
func (r *Router) setupAuthRoutes(v1 *gin.RouterGroup) {
	auth := v1.Group("/auth")
	{
		// ===== Public routes (tidak perlu authentication) =====
		auth.POST("/register", r.authHandler.Register)
		auth.POST("/token", r.authHandler.Login)
		auth.POST("/refresh", r.authHandler.RefreshToken)

		// ===== Protected routes (perlu authentication) =====
		protected := auth.Group("")
		protected.Use(middleware.AuthMiddleware(r.tokenService, r.userRepo, r.redisClient))
		{
			// User session management
			protected.POST("/logout", r.authHandler.Logout)
			protected.POST("/logout-all", r.authHandler.LogoutAll)

			// User profile
			protected.GET("/me", r.authHandler.GetCurrentUser)
			protected.PUT("/profile", r.authHandler.UpdateProfile)
			protected.POST("/change-password", r.authHandler.ChangePassword)

			// Activity history
			protected.GET("/activities", r.authHandler.GetActivities)
			protected.GET("/activities/summary", r.authHandler.GetActivitySummary)
			protected.GET("/activities/recent", r.authHandler.GetRecentActivities)
		}
	}
}

// setupRoleRoutes mengkonfigurasi routes untuk role management
// Base path: /roles
// Semua routes memerlukan authentication
//
// Access levels:
//   - Any user: /me/* (akses roles/permissions sendiri)
//   - Admin/Moderator: GET roles, permissions, users
//   - Admin only: CRUD roles, assign/remove roles
func (r *Router) setupRoleRoutes(v1 *gin.RouterGroup) {
	roles := v1.Group("/roles")
	roles.Use(middleware.AuthMiddleware(r.tokenService, r.userRepo, r.redisClient))
	{
		// ===== Any authenticated user (akses data sendiri) =====
		roles.GET("/me/roles", r.roleHandler.GetMyRoles)
		roles.GET("/me/permissions", r.roleHandler.GetMyPermissions)
		roles.GET("/me/permissions/:permission_name", r.roleHandler.CheckMyPermission)

		// ===== Admin/Moderator routes (read-only) =====
		adminMod := roles.Group("")
		adminMod.Use(middleware.RequireRole(r.roleRepo, r.redisClient, "admin", "moderator", "admin_testing"))
		{
			adminMod.GET("", r.roleHandler.GetAllRoles)
			adminMod.GET("/:role_id", r.roleHandler.GetRoleByID)
			adminMod.GET("/name/:role_name", r.roleHandler.GetRoleByName)
			adminMod.GET("/permissions", r.roleHandler.GetAllPermissions)
			adminMod.GET("/role/:role_name/users", r.roleHandler.GetUsersWithRole)
		}

		// ===== User Inspection (requires user.inspect permission) =====
		// Used by admin dashboard to view other users' roles/permissions
		roles.GET("/user/:user_id", middleware.RequirePermission(r.userRepo, r.redisClient, "user.inspect"), r.roleHandler.GetUserRoles)
		roles.GET("/user/:user_id/permissions", middleware.RequirePermission(r.userRepo, r.redisClient, "user.inspect"), r.roleHandler.GetUserPermissions)
		roles.GET("/user/:user_id/permissions/:permission_name", middleware.RequirePermission(r.userRepo, r.redisClient, "user.inspect"), r.roleHandler.CheckUserPermission)

		// ===== Admin only (full CRUD) =====
		admin := roles.Group("")
		admin.Use(middleware.RequireRole(r.roleRepo, r.redisClient, "admin"))
		{
			// Role CRUD
			admin.POST("", r.roleHandler.CreateRole)
			admin.PUT("/:role_id", r.roleHandler.UpdateRole)
			admin.DELETE("/:role_id", r.roleHandler.DeleteRole)

			// Permission management
			admin.POST("/:role_id/permissions", r.roleHandler.AddPermissionsToRole)
			admin.DELETE("/:role_id/permissions", r.roleHandler.RemovePermissionsFromRole)

			// User-role assignment
			admin.POST("/assign", r.roleHandler.AssignRoleToUser)
			admin.DELETE("/remove", r.roleHandler.RemoveRoleFromUser)

			// Analytics
			admin.GET("/analytics/statistics", r.roleHandler.GetRoleStatistics)
		}
	}
}

// setupPermissionRoutes mengkonfigurasi routes untuk permission management
// Base path: /permissions
// Admin only
func (r *Router) setupPermissionRoutes(v1 *gin.RouterGroup) {
	permissions := v1.Group("/permissions")
	permissions.Use(middleware.AuthMiddleware(r.tokenService, r.userRepo, r.redisClient))

	// Admin only
	permissions.Use(middleware.RequireRole(r.roleRepo, r.redisClient, "admin"))
	{
		permissions.GET("", r.permissionHandler.GetAllPermissions)
		permissions.POST("", r.permissionHandler.CreatePermission)
		permissions.GET("/:id", r.permissionHandler.GetPermissionByID)
		permissions.PUT("/:id", r.permissionHandler.UpdatePermission)
		permissions.DELETE("/:id", r.permissionHandler.DeletePermission)
	}
}

// setupInternalRoutes mengkonfigurasi routes untuk internal service-to-service communication
// Base path: /internal
// Dilindungi oleh ServiceTokenMiddleware (bukan JWT user, tapi service token)
//
// Routes:
//   - GET /internal/health-internal: Internal health check
//   - PUT /internal/users/:user_id/status: Update user status (Admin only)
//   - GET /internal/social/token/:user_id/:platform: Fetch valid OAuth token for engine
func (r *Router) setupInternalRoutes(v1 *gin.RouterGroup) {
	internal := v1.Group("/internal")
	{
		// Service-to-service routes
		internal.GET("/health-internal", middleware.ServiceTokenMiddleware(r.config), r.internalHandler.HealthCheckInternal)

		// Engine calls this to get a valid OAuth access token before uploading a clip.
		// Protected by service token only — no user JWT required.
		internal.GET("/social/token/:user_id/:platform", middleware.ServiceTokenMiddleware(r.config), r.socialHandler.GetInternalToken)

		// Admin-only user management routes
		// Protected by Auth + Admin role middleware
		adminRoutes := internal.Group("/users")
		adminRoutes.Use(middleware.AuthMiddleware(r.tokenService, r.userRepo, r.redisClient))
		adminRoutes.Use(middleware.RequireRole(r.roleRepo, r.redisClient, "admin"))
		{
			adminRoutes.GET("/:user_id", r.authHandler.GetUserByID)
			adminRoutes.PUT("/:user_id/status", r.authHandler.UpdateUserStatus)
			adminRoutes.DELETE("/:user_id", r.authHandler.DeleteUser)
		}
	}
}

// setupSocialRoutes configures routes for OAuth social account management.
// Base path: /social
func (r *Router) setupSocialRoutes(v1 *gin.RouterGroup) {
	// JWT-protected group
	social := v1.Group("/social")
	social.Use(middleware.AuthMiddleware(r.tokenService, r.userRepo, r.redisClient))
	{
		social.GET("/auth/youtube/start", r.socialHandler.StartYouTubeOAuth)
		social.GET("/auth/tiktok/start", r.socialHandler.StartTikTokOAuth)
		social.GET("/accounts", r.socialHandler.GetAccounts)
		social.DELETE("/accounts/:platform", r.socialHandler.DisconnectAccount)
	}

	// Public callback — Provider redirects the browser here; no Bearer token is present.
	// Registered directly on v1 so it bypasses the JWT middleware group above.
	v1.GET("/social/auth/youtube/callback", r.socialHandler.YouTubeCallback)
	v1.GET("/social/auth/tiktok/callback", r.socialHandler.TikTokCallback)
}
