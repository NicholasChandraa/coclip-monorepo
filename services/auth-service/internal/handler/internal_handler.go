package handler

import (
	"net/http"

	"auth-service/internal/domain"
	"auth-service/internal/usecase"

	"github.com/gin-gonic/gin"
)

// InternalHandler adalah handler untuk endpoint internal (service-to-service)
// Endpoint ini dipakai oleh service lain (misal: chat-service) untuk query data user
// Akses ke endpoint ini dilindungi oleh ServiceAuthMiddleware (bukan JWT user)
type InternalHandler struct {
	authUsecase usecase.AuthUsecase
}

// NewInternalHandler membuat instance baru InternalHandler
func NewInternalHandler(authUsecase usecase.AuthUsecase) *InternalHandler {
	return &InternalHandler{
		authUsecase: authUsecase,
	}
}


// HealthCheckInternal adalah handler untuk health check internal
// Endpoint: GET /internal/health-internal
// Dipakai untuk monitoring dan load balancer internal
func (h *InternalHandler) HealthCheckInternal(c *gin.Context) {
	c.JSON(http.StatusOK, domain.HealthResponse{
		Status:   "healthy",
		Service:  "auth-service",
		Internal: true,
	})
}
