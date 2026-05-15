package handler

import (
	"net/http"

	"auth-service/internal/domain"
	"auth-service/internal/usecase"
	"auth-service/pkg/logger"

	"github.com/gin-gonic/gin"
)

// PermissionHandler handles HTTP requests for permission management
type PermissionHandler struct {
	permissionUsecase usecase.PermissionUsecase
}

// NewPermissionHandler creates a new instance of PermissionHandler
func NewPermissionHandler(permissionUsecase usecase.PermissionUsecase) *PermissionHandler {
	return &PermissionHandler{
		permissionUsecase: permissionUsecase,
	}
}

// GetAllPermissions gets all permissions
// Endpoint: GET /permissions
// Query params: resource (optional)
func (h *PermissionHandler) GetAllPermissions(c *gin.Context) {
	log := logger.GetLoggerFromGinContext(c)
	resource := c.Query("resource")

	permissions, err := h.permissionUsecase.GetAllPermissions(c.Request.Context(), resource)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get all permissions")
		c.JSON(http.StatusInternalServerError, domain.ErrorResponse{
			Detail: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, permissions)
}

// GetPermissionByID gets a permission by ID
// Endpoint: GET /permissions/:id
func (h *PermissionHandler) GetPermissionByID(c *gin.Context) {
	log := logger.GetLoggerFromGinContext(c)
	id := c.Param("id")

	permission, err := h.permissionUsecase.GetPermissionByID(c.Request.Context(), id)
	if err != nil {
		log.Error().Err(err).Str("permission_id", id).Msg("Failed to get permission by ID")
		status := http.StatusInternalServerError
		if err == domain.ErrPermissionNotFound {
			status = http.StatusNotFound
		}
		c.JSON(status, domain.ErrorResponse{
			Detail: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, permission)
}

// CreatePermission creates a new permission
// Endpoint: POST /permissions
func (h *PermissionHandler) CreatePermission(c *gin.Context) {
	log := logger.GetLoggerFromGinContext(c)
	var req domain.PermissionCreate
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warn().Err(err).Msg("Invalid create permission request")
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{
			Detail: FormatValidationError(err),
		})
		return
	}

	permission, err := h.permissionUsecase.CreatePermission(c.Request.Context(), &req)
	if err != nil {
		log.Error().Err(err).Str("permission_name", req.Name).Msg("Failed to create permission")
		status := http.StatusInternalServerError
		if err == domain.ErrPermissionAlreadyExists {
			status = http.StatusConflict
		}
		c.JSON(status, domain.ErrorResponse{
			Detail: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, permission)
}

// UpdatePermission updates an existing permission
// Endpoint: PUT /permissions/:id
func (h *PermissionHandler) UpdatePermission(c *gin.Context) {
	log := logger.GetLoggerFromGinContext(c)
	id := c.Param("id")

	var req domain.PermissionUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warn().Err(err).Str("permission_id", id).Msg("Invalid update permission request")
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{
			Detail: FormatValidationError(err),
		})
		return
	}

	permission, err := h.permissionUsecase.UpdatePermission(c.Request.Context(), id, &req)
	if err != nil {
		log.Error().Err(err).Str("permission_id", id).Msg("Failed to update permission")
		status := http.StatusInternalServerError
		switch err {
		case domain.ErrPermissionNotFound:
			status = http.StatusNotFound
		case domain.ErrPermissionAlreadyExists:
			status = http.StatusConflict
		}
		c.JSON(status, domain.ErrorResponse{
			Detail: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, permission)
}

// DeletePermission deletes a permission
// Endpoint: DELETE /permissions/:id
func (h *PermissionHandler) DeletePermission(c *gin.Context) {
	log := logger.GetLoggerFromGinContext(c)
	id := c.Param("id")

	if err := h.permissionUsecase.DeletePermission(c.Request.Context(), id); err != nil {
		log.Error().Err(err).Str("permission_id", id).Msg("Failed to delete permission")
		status := http.StatusInternalServerError
		if err == domain.ErrPermissionNotFound {
			status = http.StatusNotFound
		}
		c.JSON(status, domain.ErrorResponse{
			Detail: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, domain.MessageResponse{
		Detail: "Permission deleted successfully",
	})
}
