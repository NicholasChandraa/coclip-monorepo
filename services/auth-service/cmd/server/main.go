package main

import (
	"context"
	stdlog "log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"auth-service/internal/config"
	"auth-service/internal/database"
	"auth-service/internal/handler"
	"auth-service/internal/repository"
	"auth-service/internal/usecase"
	"auth-service/pkg/jwt"
	"auth-service/pkg/logger"
	redisClient "auth-service/pkg/redis"
)

// main adalah entry point aplikasi auth service
// Function ini melakukan:
//  1. Load konfigurasi dari environment variables
//  2. Setup logger untuk structured logging
//  3. Connect ke database & run migrations
//  4. Initialize semua dependencies (repositories, usecases, handlers)
//  5. Setup HTTP router & middleware
//  6. Start HTTP server dengan graceful shutdown
func main() {
	// ========== STEP 1: Configuration ==========
	// Load konfigurasi dari .env file atau environment variables
	cfg, err := config.Load()
	if err != nil {
		stdlog.Fatalf("Failed to load configuration: %v", err)
	}

	// ========== STEP 2: Logger ==========
	// Initialize structured logger (zerolog)
	// NOTE: ALWAYS run from project root (auth-service/) to ensure logs directory is correct
	// Command: go run ./cmd/server (from auth-service/ directory)
	loggerCfg := logger.DefaultConfig()
	loggerCfg.Level = cfg.Logger.Level // Set log level dari config (debug, info, warn, error)
	if err := logger.Init(loggerCfg); err != nil {
		stdlog.Fatalf("Failed to initialize logger: %v", err)
	}

	logger.Info().Str("port", cfg.Server.Port).Str("log_level", cfg.Logger.Level).Msg("Starting Auth Service")

	// ========== STEP 3: Database ==========
	// Connect ke PostgreSQL database menggunakan GORM
	db, err := database.NewPostgresDB(&cfg.Database)
	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to connect to database")
	}
	logger.Info().Msg("Database connected successfully")

	// Run auto migrations - create/update tables berdasarkan domain models
	// GORM akan create tables: users, roles, permissions, refresh_tokens, user_activities, user_roles, role_permissions
	if err := database.AutoMigrate(db); err != nil {
		logger.Fatal().Err(err).Msg("Failed to run migrations")
	}

	// Seed initial data - create default roles (admin, user, moderator) & permissions
	// Hanya dijalankan kalau data belum ada (idempotent)
	// Data yang di-seed:
	//  - Role: admin, user, moderator
	//  - Permissions: users:read, users:write, roles:manage, dll
	//  - Assign permissions ke roles sesuai hierarchy
	if err := database.SeedData(db); err != nil {
		logger.Warn().Err(err).Msg("Failed to seed data") // Warning saja, tidak fatal
	}

	// ========== STEP 4: Dependency Injection - Bottom-Up ==========
	// Ini adalah "pabrik" dimana semua object dibuat dan di-wire bersama
	// Pattern: Buat dependencies dari layer paling bawah (repository) ke atas (handler)

	// Layer 1: Repository - Data Access Layer (interact dengan database)
	// Repositories handle CRUD operations ke database menggunakan GORM
	userRepo := repository.NewUserRepository(db)
	roleRepo := repository.NewRoleRepository(db)
	permissionRepo := repository.NewPermissionRepository(db)
	refreshTokenRepo := repository.NewRefreshTokenRepository(db)
	activityRepo := repository.NewUserActivityRepository(db)
	socialRepo := repository.NewSocialAccountRepository(db)
	logger.Debug().Msg("Repositories initialized")

	// Service: JWT Token Service (bukan layer, tapi utility service)
	// Service ini handle:
	//  - Generate access token (short-lived: 15-30 menit)
	//  - Generate refresh token (long-lived: 7-30 hari)
	//  - Validate JWT signature & expiry
	//  - Extract claims dari token (user_id, username, email)
	tokenService := jwt.NewTokenService(
		cfg.JWT.SecretKey,          // Secret key untuk sign JWT (HARUS di-encrypt di production!)
		cfg.JWT.AccessTokenExpiry,  // Expiry access token (short-lived, misal 15 menit)
		cfg.JWT.RefreshTokenExpiry, // Expiry refresh token (long-lived, misal 7 hari)
	)

	// =========== Initialize Redis Client ==========
	logger.Info().Msg("Initializing Redis connection...")
	redisClient, err := redisClient.NewClient(
		cfg.Redis.Host,
		cfg.Redis.Port,
		cfg.Redis.Password,
		cfg.Redis.DB,
		cfg.Redis.PoolSize,
	)

	if err != nil {
		logger.Fatal().Err(err).Msg("Failed to connect to Redis")
	}

	defer redisClient.Close()

	// Test Redis connection
	if err := redisClient.Ping(context.Background()); err != nil {
		logger.Fatal().Err(err).Msg("Redis ping failed")
	}
	logger.Info().Msg("Redis connection established")

	// Layer 2: Usecase - Business Logic Layer
	// Inject repositories & services yang sudah dibuat ke usecase
	// Usecases berisi SEMUA business logic, tidak tau tentang HTTP/database detail
	authUsecase := usecase.NewAuthUsecase(
		userRepo,         // ← Dependency injection
		refreshTokenRepo, // ← Dependency injection
		activityRepo,     // ← Dependency injection
		roleRepo,         // ← Dependency injection
		tokenService,     // ← Dependency injection
		redisClient,      // ← Redis client untuk token blacklist
		cfg,              // ← Dependency injection
	)

	roleUsecase := usecase.NewRoleUsecase(
		roleRepo,       // ← Dependency injection
		userRepo,       // ← Dependency injection
		permissionRepo, // ← Dependency injection
	)
	permissionUsecase := usecase.NewPermissionUsecase(
		permissionRepo,
		roleRepo,
	)
	socialUsecase := usecase.NewSocialUseCase(socialRepo, redisClient, cfg)
	logger.Debug().Msg("Usecases initialized")

	// Layer 3: Handler - HTTP Layer (Gin handlers)
	// Inject usecases yang sudah dibuat ke handler
	// Handlers handle HTTP request/response, validation, status codes
	authHandler := handler.NewAuthHandler(authUsecase, cfg, redisClient)  // ← Dependency injection
	roleHandler := handler.NewRoleHandler(roleUsecase)                    // ← Dependency injection
	permissionHandler := handler.NewPermissionHandler(permissionUsecase)  // ← Dependency injection
	internalHandler := handler.NewInternalHandler(authUsecase)            // ← Dependency injection
	socialHandler := handler.NewSocialHandler(socialUsecase, cfg)         // ← Dependency injection

	// Layer 4: Router - HTTP Router & Middleware Setup
	// Inject handlers yang sudah dibuat ke router
	// Router setup routes (/auth, /roles, /internal, /social) dan middleware (CORS, Auth, etc)
	router := handler.NewRouter(
		authHandler,       // ← Dependency injection
		roleHandler,       // ← Dependency injection
		permissionHandler, // ← Dependency injection
		internalHandler,   // ← Dependency injection
		socialHandler,     // ← Dependency injection
		tokenService,      // ← Untuk middleware authentication
		userRepo,          // ← Untuk middleware user lookup
		roleRepo,          // ← Untuk middleware role checking
		redisClient,
		cfg, // ← Untuk CORS dan config lain
	)

	// ========== STEP 5: Router Setup ==========
	// Setup semua routes (/auth, /roles, /internal) dengan middleware
	// Routes yang disetup:
	//  - Public: POST /auth/register, POST /auth/token, POST /auth/refresh
	//  - Protected: GET /auth/me, PUT /auth/profile, POST /auth/logout, dll
	//  - Admin: POST /roles, PUT /roles/:id, DELETE /roles/:id, dll
	engine := router.Setup()

	// ========== STEP 6: HTTP Server Configuration ==========
	// Create HTTP server dengan timeout configuration untuk security & stability
	// Timeout configuration prevent:
	//  - Slowloris attack (client send request very slowly)
	//  - Keep-alive connection abuse
	//  - Memory leak dari connection yang nggak ditutup
	srv := &http.Server{
		Addr:         ":" + cfg.Server.Port, // Port dari config (default: 8000)
		Handler:      engine,                // Gin router handler
		ReadTimeout:  15 * time.Second,      // Max waktu read request (prevent slow client attack)
		WriteTimeout: 15 * time.Second,      // Max waktu write response
		IdleTimeout:  60 * time.Second,      // Max waktu idle connection (keep-alive)
	}

	// ========== STEP 7: Start Server (Non-Blocking) ==========
	// Start server di goroutine terpisah agar tidak blocking main thread
	// Main thread akan wait for shutdown signal (SIGINT/SIGTERM)
	// Kenapa di goroutine?
	//  - Kalau ListenAndServe() dipanggil langsung, code di bawahnya tidak akan pernah dieksekusi
	//  - Dengan goroutine, main thread bisa lanjut ke signal handling
	go func() {
		logger.Info().Str("port", cfg.Server.Port).Msg("Server is running")
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Fatal().Err(err).Msg("Failed to start server")
		}
	}()

	// ========== STEP 8: Graceful Shutdown ==========
	// Wait for interrupt signal (Ctrl+C atau kill command)
	// Channel quit akan receive signal dari OS (SIGINT = Ctrl+C, SIGTERM = kill)
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM) // Listen untuk SIGINT (Ctrl+C) dan SIGTERM (kill)
	<-quit                                               // Block sampai dapat signal (blocking operation)

	logger.Info().Msg("Shutting down server...")

	// Graceful shutdown: kasih waktu 10 detik untuk:
	//  1. Finish ongoing requests (tidak langsung kill request yang sedang berjalan)
	//  2. Close database connections dengan proper cleanup
	//  3. Cleanup resources (file handles, goroutines, dll)
	//  4. Send response ke client yang sedang menunggu
	// Kalau lebih dari 10 detik, server akan di-force shutdown (kill)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel() // Cleanup context setelah shutdown selesai

	// Shutdown server dengan context timeout
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error().Err(err).Msg("Server forced to shutdown") // Timeout, force shutdown
	}

	logger.Info().Msg("Server exited properly") // Clean exit (semua request selesai, resources cleaned up)
}
