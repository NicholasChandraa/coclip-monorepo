package middleware

import (
	"net/http"

	"auth-service/internal/config"
	"auth-service/internal/domain"

	"github.com/gin-gonic/gin"
)

// ServiceTokenMiddleware adalah middleware untuk autentikasi internal service-to-service
// Berbeda dengan AuthMiddleware yang pakai JWT user, middleware ini pakai static token
//
// Use case:
//   - Chat-service perlu query thinking mode user ke auth-service
//   - Service lain perlu data user tanpa harus punya JWT user
//
// Authentication:
//   - Header: X-Service-Token
//   - Value: harus match dengan SERVICE_TOKEN di environment
//
// Response jika gagal:
//   - 401 Unauthorized: token tidak ada atau tidak valid
func ServiceTokenMiddleware(cfg *config.Config) gin.HandlerFunc {
	return func(c *gin.Context) {
		// Ambil token dari header X-Service-Token
		serviceToken := c.GetHeader("X-Service-Token")

		// Cek apakah header ada
		if serviceToken == "" {
			c.JSON(http.StatusUnauthorized, domain.ErrorResponse{
				Detail: "X-Service-Token header is required",
			})
			c.Abort()
			return
		}

		// Validasi token dengan yang di config
		if serviceToken != cfg.Service.ServiceToken {
			c.JSON(http.StatusUnauthorized, domain.ErrorResponse{
				Detail: "Invalid service token",
			})
			c.Abort()
			return
		}

		// Token valid, lanjut ke handler
		c.Next()
	}
}
