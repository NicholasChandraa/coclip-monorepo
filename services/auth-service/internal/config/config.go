// Package config berisi konfigurasi aplikasi yang di-load dari environment variables
// Semua konfigurasi di-centralize di sini untuk memudahkan management
package config

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/joho/godotenv"
)

// Config adalah struct utama yang menampung semua konfigurasi aplikasi
// Struct ini di-pass ke berbagai layer (handler, usecase, dll) via dependency injection
type Config struct {
	Server   ServerConfig   // Konfigurasi HTTP server
	Database DatabaseConfig // Konfigurasi koneksi database
	Redis    RedisConfig    // Konfigurasi koneksi Redis
	JWT      JWTConfig      // Konfigurasi JWT token
	Cookie   CookieConfig   // Konfigurasi cookie untuk refresh token
	CORS     CORSConfig     // Konfigurasi CORS untuk cross-origin requests
	Service  ServiceConfig  // Konfigurasi untuk internal service communication
	Social   SocialConfig   // Konfigurasi OAuth untuk social media integrations
	Logger   LoggerConfig   // Konfigurasi logging
}

// ServerConfig berisi konfigurasi HTTP server
type ServerConfig struct {
	Port string // Port server (default: 8005)
	Mode string // Gin mode: debug, release, test (default: release)
}

// DatabaseConfig berisi konfigurasi koneksi PostgreSQL
type DatabaseConfig struct {
	Host     string // Database host (default: localhost)
	Port     string // Database port (default: 5432)
	User     string // Database user (default: postgres)
	Password string // Database password
	DBName   string // Nama database (default: auth_db)
	SSLMode  string // SSL mode: disable, require, verify-full (default: disable)
}

// RedisConfig berisi konfigurasi koneksi Redis
type RedisConfig struct {
	Host     string `yaml:"host"`
	Port     string `yaml:"port"`
	Password string `yaml:"password"`
	DB       string `yaml:"db"`
	PoolSize string `yaml:"pool_size"`
}

// JWTConfig berisi konfigurasi JWT token
type JWTConfig struct {
	SecretKey          string        // Secret key untuk sign JWT (WAJIB diganti di production!)
	AccessTokenExpiry  time.Duration // Durasi access token (default: 30 menit)
	RefreshTokenExpiry time.Duration // Durasi refresh token (default: 7 hari)
}

// CookieConfig berisi konfigurasi cookie untuk refresh token
type CookieConfig struct {
	Domain   string // Domain cookie (kosong = current domain)
	Secure   bool   // Secure flag: true = hanya HTTPS (default: true)
	SameSite string // SameSite: Strict, Lax, None (default: Lax)
	Path     string // Path cookie (default: /)
}

// CORSConfig berisi konfigurasi CORS (Cross-Origin Resource Sharing)
type CORSConfig struct {
	AllowedOrigins   []string // Daftar origin yang diizinkan (default: http://localhost:3000)
	AllowCredentials bool     // Allow credentials (cookies) di cross-origin requests
}

// ServiceConfig berisi konfigurasi untuk komunikasi antar service
type ServiceConfig struct {
	ServiceToken string // Token untuk autentikasi internal service-to-service
}

// YouTubeConfig holds Google OAuth2 credentials for YouTube Data API access
type YouTubeConfig struct {
	ClientID     string // Google OAuth2 client ID
	ClientSecret string // Google OAuth2 client secret
	RedirectURI  string // OAuth2 callback URI registered in Google Cloud Console
}

// TikTokConfig holds TikTok OAuth2 credentials for Content Posting API access
type TikTokConfig struct {
	ClientKey    string // TikTok OAuth2 client key
	ClientSecret string // TikTok OAuth2 client secret
	RedirectURI  string // OAuth2 callback URI registered in TikTok Developers Portal
}

// SocialConfig holds social media OAuth configuration for third-party integrations
type SocialConfig struct {
	YouTube       YouTubeConfig
	TikTok        TikTokConfig
	EncryptionKey string // 64-char hex = 32 bytes for AES-256 token encryption at rest
	FrontendURL   string // Base URL of the Next.js frontend for post-OAuth redirects
}

// LoggerConfig berisi konfigurasi logging
type LoggerConfig struct {
	Level string // Log level: debug, info, warn, error (default: info)
}

// Load membaca konfigurasi dari environment variables dan file .env
// Semua config punya default value, jadi aplikasi tetap bisa jalan tanpa .env file
//
// Environment variables yang dibaca:
//   - SERVER_PORT, GIN_MODE
//   - DB_HOST, DB_PORT, DB_USER, DB_PASSWORD, DB_NAME, DB_SSLMODE
//   - JWT_SECRET_KEY, JWT_ACCESS_TOKEN_EXPIRY_MINUTES, JWT_REFRESH_TOKEN_EXPIRY_DAYS
//   - COOKIE_DOMAIN, COOKIE_SECURE, COOKIE_SAMESITE
//   - CORS_ALLOWED_ORIGINS
//   - SERVICE_TOKEN
//   - LOG_LEVEL
func Load() (*Config, error) {
	// Load .env file jika ada (error diabaikan jika file tidak ada)
	_ = godotenv.Load()

	// Parse nilai numerik dari environment
	accessExpiry, _ := strconv.Atoi(getEnv("JWT_ACCESS_TOKEN_EXPIRY_MINUTES", "30"))
	refreshExpiry, _ := strconv.Atoi(getEnv("JWT_REFRESH_TOKEN_EXPIRY_DAYS", "7"))
	cookieSecure, _ := strconv.ParseBool(getEnv("COOKIE_SECURE", "true"))

	return &Config{
		// Server config
		Server: ServerConfig{
			Port: getEnv("SERVER_PORT", "8005"),
			Mode: getEnv("GIN_MODE", "release"),
		},

		// Database config (PostgreSQL)
		Database: DatabaseConfig{
			Host:     getEnv("DB_HOST", "localhost"),
			Port:     getEnv("DB_PORT", "5432"),
			User:     getEnv("DB_USER", "postgres"),
			Password: getEnv("DB_PASSWORD", "postgres"),
			DBName:   getEnv("DB_NAME", "auth_db"),
			SSLMode:  getEnv("DB_SSLMODE", "disable"),
		},

		// Redis Config
		Redis: RedisConfig{
			Host:     getEnv("REDIS_HOST", "localhost"),
			Port:     getEnv("REDIS_PORT", "6379"),
			Password: getEnv("REDIS_PASSWORD", ""),
			DB:       getEnv("REDIS_DB", "0"),
			PoolSize: getEnv("REDIS_POOL_SIZE", "10"),
		},

		// JWT config
		JWT: JWTConfig{
			SecretKey:          getEnv("JWT_SECRET_KEY", "your-super-secret-key-change-in-production"),
			AccessTokenExpiry:  time.Duration(accessExpiry) * time.Minute,
			RefreshTokenExpiry: time.Duration(refreshExpiry) * 24 * time.Hour,
		},

		// Cookie config (untuk refresh token)
		Cookie: CookieConfig{
			Domain:   getEnv("COOKIE_DOMAIN", ""),
			Secure:   cookieSecure,
			SameSite: getEnv("COOKIE_SAMESITE", "Lax"),
			Path:     "/",
		},

		// CORS config
		CORS: CORSConfig{
			AllowedOrigins:   parseCSV(getEnv("CORS_ALLOWED_ORIGINS", "http://localhost:3000")),
			AllowCredentials: true,
		},

		// Internal service config
		Service: ServiceConfig{
			ServiceToken: getEnv("SERVICE_TOKEN", "internal-service-token"),
		},

		// Social OAuth config
		Social: SocialConfig{
			YouTube: YouTubeConfig{
				ClientID:     getEnv("YOUTUBE_CLIENT_ID", ""),
				ClientSecret: getEnv("YOUTUBE_CLIENT_SECRET", ""),
				RedirectURI:  getEnv("YOUTUBE_REDIRECT_URI", "http://localhost:8005/api/v1/social/auth/youtube/callback"),
			},
			TikTok: TikTokConfig{
				ClientKey:    getEnv("TIKTOK_CLIENT_KEY", ""),
				ClientSecret: getEnv("TIKTOK_CLIENT_SECRET", ""),
				RedirectURI:  getEnv("TIKTOK_REDIRECT_URI", "http://localhost:8005/api/v1/social/auth/tiktok/callback"),
			},
			EncryptionKey: getEnv("SOCIAL_TOKEN_ENCRYPTION_KEY", ""),
			FrontendURL:   getEnv("FRONTEND_URL", "http://localhost:3000"),
		},

		// Logger config
		Logger: LoggerConfig{
			Level: getEnv("LOG_LEVEL", "info"),
		},
	}, nil
}

// getEnv mengambil nilai environment variable dengan fallback ke default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// parseCSV mem-parse comma-separated values menjadi []string
func parseCSV(value string) []string {
	if value == "" {
		return []string{}
	}

	parts := strings.Split(value, ",")
	result := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			result = append(result, trimmed)
		}
	}

	return result
}
