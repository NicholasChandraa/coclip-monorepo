package middleware

import (
	"net/http"
	"strings"

	"auth-service/internal/config"
	"auth-service/pkg/logger"

	"github.com/gin-gonic/gin"
)

// CORSMiddleware adalah middleware untuk handle Cross-Origin Resource Sharing (CORS)
// ... (comments kept as is) ...
func CORSMiddleware(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		origin := c.Request.Header.Get("Origin")

		// Debug logging for CORS
		if origin != "" {
			logger.Debug().
				Str("origin", origin).
				Strs("allowed_origins", cfg.CORS.AllowedOrigins).
				Msg("Checking CORS")
		}

		// Cek apakah origin ada di whitelist
		allowed := false
		for _, allowedOrigin := range cfg.CORS.AllowedOrigins {
			// Exact match atau wildcard "*"
			if allowedOrigin == "*" || allowedOrigin == origin {
				allowed = true
				break
			}

			// Support wildcard subdomain (misal: *.example.com)
			if strings.HasPrefix(allowedOrigin, "*.") {
				domain := strings.TrimPrefix(allowedOrigin, "*")
				if strings.HasSuffix(origin, domain) {
					allowed = true
					break
				}
			}
		}

		if origin != "" && !allowed {
			logger.Warn().
				Str("origin", origin).
				Strs("allowed_origins", cfg.CORS.AllowedOrigins).
				Msg("CORS blocked origin")
		}

		// Set Allow-Origin header jika origin diizinkan
		if allowed && origin != "" {
			c.Header("Access-Control-Allow-Origin", origin)
		}

		// Allow credentials (cookies) - penting untuk refresh token di cookie
		c.Header("Access-Control-Allow-Credentials", "true")

		// Headers yang diizinkan dari client
		c.Header("Access-Control-Allow-Headers",
			"Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, "+
				"Authorization, Accept, Origin, Cache-Control, X-Requested-With, X-Service-Token, X-Skip-Refresh")

		// HTTP methods yang diizinkan
		c.Header("Access-Control-Allow-Methods", "GET, POST, PUT, PATCH, DELETE, OPTIONS")

		// Cache preflight response selama 24 jam (86400 detik)
		c.Header("Access-Control-Max-Age", "86400")

		// Handle preflight OPTIONS request
		// Browser kirim OPTIONS dulu sebelum actual request untuk cek CORS
		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}
