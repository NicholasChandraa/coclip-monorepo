package middleware

import (
	"auth-service/pkg/logger"

	"github.com/gin-gonic/gin"
	"github.com/google/uuid"
)

// RequestIDMiddleware generates a unique request ID for each incoming request
// Request ID ini akan:
// 1. Di-return ke client via header "X-Request-ID"
// 2. Inject logger instance yang sudah include request ID otomatis
//
// Kegunaan: Untuk tracing/debugging - semua log dari 1 request bisa di-track dengan request ID yang sama
func RequestIDMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		// 1. Generate UUID v4 sebagai request ID
		requestID := uuid.New().String()

		// 2. Return request ID ke client via response header
		c.Header("X-Request-ID", requestID)

		// 3. Buat logger instance dengan request ID built-in
		// Logger ini sudah "dibumbui" dengan request ID, jadi semua log statement
		// yang pakai logger ini otomatis include request ID tanpa perlu set manual
		reqLogger := logger.Log.With().
			Str("request_id", requestID).
			Caller().  // Add caller info untuk clickable file location
			Logger()

		//4. Inject logger instance ke context
		// Semua handler/usecase bisa ambil logger ini dan otomatis dapat request ID
		c.Set("logger", &reqLogger)

		// 5. Log request started dengan request ID
		reqLogger.Info().
			Str("method", c.Request.Method).
			Str("path", c.Request.URL.Path).
			Str("client_ip", c.ClientIP()).
			Msg("Request started")

		// 6. Process request
		c.Next()

		// 7. Log request completed dengan status code
		reqLogger.Info().
			Int("status_code", c.Writer.Status()).
			Msg("Request completed")
	}
}
