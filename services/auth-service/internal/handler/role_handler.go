package handler

import (
	"net/http"
	"strconv"

	"auth-service/internal/domain"
	"auth-service/internal/middleware"
	"auth-service/internal/usecase"
	"auth-service/pkg/logger"

	"github.com/gin-gonic/gin"
)

// RoleHandler adalah handler untuk manajemen roles dan permissions
// Fitur utama:
//   - CRUD roles (admin only)
//   - Manage permissions per role
//   - Assign/remove role ke user
//   - Query roles & permissions user
type RoleHandler struct {
	roleUsecase usecase.RoleUsecase
}

// NewRoleHandler membuat instance baru RoleHandler
func NewRoleHandler(roleUsecase usecase.RoleUsecase) *RoleHandler {
	return &RoleHandler{
		roleUsecase: roleUsecase,
	}
}

// ========== Role CRUD ==========

// GetAllRoles adalah handler untuk mengambil semua roles
// Endpoint: GET /roles/
// Query params: include_inactive (default: false)
func (h *RoleHandler) GetAllRoles(c *gin.Context) {
	log := logger.GetLoggerFromGinContext(c)
	includeInactive, _ := strconv.ParseBool(c.DefaultQuery("include_inactive", "false"))

	roles, err := h.roleUsecase.GetAllRoles(c.Request.Context(), includeInactive)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get all roles")
		c.JSON(http.StatusInternalServerError, domain.ErrorResponse{
			Detail: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, roles)
}

// GetRoleByID adalah handler untuk mengambil role berdasarkan ID
// Endpoint: GET /roles/:role_id
func (h *RoleHandler) GetRoleByID(c *gin.Context) {
	log := logger.GetLoggerFromGinContext(c)
	roleID := c.Param("role_id") // UUID as string

	role, err := h.roleUsecase.GetRoleByID(c.Request.Context(), roleID)
	if err != nil {
		log.Error().Err(err).Str("role_id", roleID).Msg("Failed to get role by ID")
		status := http.StatusInternalServerError
		if err == domain.ErrRoleNotFound {
			status = http.StatusNotFound
		}
		c.JSON(status, domain.ErrorResponse{
			Detail: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, role)
}

// GetRoleByName adalah handler untuk mengambil role berdasarkan nama
// Endpoint: GET /roles/name/:role_name
func (h *RoleHandler) GetRoleByName(c *gin.Context) {
	log := logger.GetLoggerFromGinContext(c)
	roleName := c.Param("role_name")

	role, err := h.roleUsecase.GetRoleByName(c.Request.Context(), roleName)
	if err != nil {
		log.Error().Err(err).Str("role_name", roleName).Msg("Failed to get role by name")
		status := http.StatusInternalServerError
		if err == domain.ErrRoleNotFound {
			status = http.StatusNotFound
		}
		c.JSON(status, domain.ErrorResponse{
			Detail: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, role)
}

// CreateRole adalah handler untuk membuat role baru
// Endpoint: POST /roles/
// Request Body: JSON dengan name dan description
func (h *RoleHandler) CreateRole(c *gin.Context) {
	log := logger.GetLoggerFromGinContext(c)
	var req domain.RoleCreate
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warn().Err(err).Msg("Invalid create role request")
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{
			Detail: FormatValidationError(err),
		})
		return
	}

	role, err := h.roleUsecase.CreateRole(c.Request.Context(), &req)
	if err != nil {
		log.Error().Err(err).Str("role_name", req.Name).Msg("Failed to create role")
		status := http.StatusInternalServerError
		if err == domain.ErrRoleAlreadyExists {
			status = http.StatusConflict // Role dengan nama yang sama sudah ada
		}
		c.JSON(status, domain.ErrorResponse{
			Detail: err.Error(),
		})
		return
	}

	c.JSON(http.StatusCreated, role)
}

// UpdateRole adalah handler untuk update role
// Endpoint: PUT /roles/:role_id
func (h *RoleHandler) UpdateRole(c *gin.Context) {
	log := logger.GetLoggerFromGinContext(c)
	roleID := c.Param("role_id") // UUID as string

	var req domain.RoleUpdate
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warn().Err(err).Str("role_id", roleID).Msg("Invalid update role request")
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{
			Detail: FormatValidationError(err),
		})
		return
	}

	role, err := h.roleUsecase.UpdateRole(c.Request.Context(), roleID, &req)
	if err != nil {
		log.Error().Err(err).Str("role_id", roleID).Msg("Failed to update role")
		status := http.StatusInternalServerError
		switch err {
		case domain.ErrRoleNotFound:
			status = http.StatusNotFound
		case domain.ErrRoleAlreadyExists:
			status = http.StatusConflict
		}
		c.JSON(status, domain.ErrorResponse{
			Detail: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, role)
}

// DeleteRole adalah handler untuk menghapus role
// Endpoint: DELETE /roles/:role_id
// Note: Role default (admin, user, moderator) tidak bisa dihapus
func (h *RoleHandler) DeleteRole(c *gin.Context) {
	log := logger.GetLoggerFromGinContext(c)
	roleID := c.Param("role_id") // UUID as string

	if err := h.roleUsecase.DeleteRole(c.Request.Context(), roleID); err != nil {
		log.Error().Err(err).Str("role_id", roleID).Msg("Failed to delete role")
		status := http.StatusInternalServerError
		switch err {
		case domain.ErrRoleNotFound:
			status = http.StatusNotFound
		case domain.ErrRoleInUse:
			status = http.StatusForbidden // Role default tidak bisa dihapus
		}
		c.JSON(status, domain.ErrorResponse{
			Detail: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, domain.MessageResponse{
		Detail: "Role deleted successfully",
	})
}

// ========== Permission Management ==========

// GetAllPermissions adalah handler untuk mengambil semua permissions
// Endpoint: GET /roles/permissions
// Query params: resource (optional filter by resource)
func (h *RoleHandler) GetAllPermissions(c *gin.Context) {
	log := logger.GetLoggerFromGinContext(c)
	resource := c.Query("resource") // Filter by resource (user, role, activity)

	permissions, err := h.roleUsecase.GetAllPermissions(c.Request.Context(), resource)
	if err != nil {
		log.Error().Err(err).Msg("Failed to get all permissions")
		c.JSON(http.StatusInternalServerError, domain.ErrorResponse{
			Detail: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, permissions)
}

// AddPermissionsToRole adalah handler untuk menambah permissions ke role
// Endpoint: POST /roles/:role_id/permissions
// Request Body: JSON dengan array permission_names
func (h *RoleHandler) AddPermissionsToRole(c *gin.Context) {
	log := logger.GetLoggerFromGinContext(c)
	roleID := c.Param("role_id") // UUID as string

	var req domain.AddPermissionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warn().Err(err).Str("role_id", roleID).Msg("Invalid add permissions request")
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{
			Detail: FormatValidationError(err),
		})
		return
	}

	if err := h.roleUsecase.AddPermissionsToRole(c.Request.Context(), roleID, req.PermissionNames); err != nil {
		log.Error().Err(err).Str("role_id", roleID).Msg("Failed to add permissions to role")
		status := http.StatusInternalServerError
		if err == domain.ErrRoleNotFound {
			status = http.StatusNotFound
		}
		c.JSON(status, domain.ErrorResponse{
			Detail: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, domain.MessageResponse{
		Detail: "Permissions added successfully",
	})
}

// RemovePermissionsFromRole adalah handler untuk menghapus permissions dari role
// Endpoint: DELETE /roles/:role_id/permissions
// Request Body: JSON dengan array permission_names
func (h *RoleHandler) RemovePermissionsFromRole(c *gin.Context) {
	log := logger.GetLoggerFromGinContext(c)
	roleID := c.Param("role_id") // UUID as string

	var req domain.RemovePermissionsRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warn().Err(err).Str("role_id", roleID).Msg("Invalid remove permissions request")
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{
			Detail: FormatValidationError(err),
		})
		return
	}

	if err := h.roleUsecase.RemovePermissionsFromRole(c.Request.Context(), roleID, req.PermissionNames); err != nil {
		log.Error().Err(err).Str("role_id", roleID).Msg("Failed to remove permissions from role")
		status := http.StatusInternalServerError
		if err == domain.ErrRoleNotFound {
			status = http.StatusNotFound
		}
		c.JSON(status, domain.ErrorResponse{
			Detail: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, domain.MessageResponse{
		Detail: "Permissions removed successfully",
	})
}

// ========== User-Role Assignment ==========

// AssignRoleToUser adalah handler untuk assign role ke user
// Endpoint: POST /roles/assign
// Request Body: JSON dengan user_id dan role_name
func (h *RoleHandler) AssignRoleToUser(c *gin.Context) {
	log := logger.GetLoggerFromGinContext(c)
	var req domain.AssignRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warn().Err(err).Msg("Invalid assign role request")
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{
			Detail: FormatValidationError(err),
		})
		return
	}

	if err := h.roleUsecase.AssignRoleToUser(c.Request.Context(), req.UserID, req.RoleName); err != nil {
		log.Error().Err(err).Str("user_id", req.UserID).Str("role_name", req.RoleName).Msg("Failed to assign role to user")
		status := http.StatusInternalServerError
		if err == domain.ErrUserNotFound || err == domain.ErrRoleNotFound {
			status = http.StatusNotFound
		}
		c.JSON(status, domain.ErrorResponse{
			Detail: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, domain.MessageResponse{
		Detail: "Role assigned successfully",
	})
}

// RemoveRoleFromUser adalah handler untuk menghapus role dari user
// Endpoint: DELETE /roles/remove
// Request Body: JSON dengan user_id dan role_name
func (h *RoleHandler) RemoveRoleFromUser(c *gin.Context) {
	log := logger.GetLoggerFromGinContext(c)
	var req domain.RemoveRoleRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		log.Warn().Err(err).Msg("Invalid remove role request")
		c.JSON(http.StatusBadRequest, domain.ErrorResponse{
			Detail: FormatValidationError(err),
		})
		return
	}

	if err := h.roleUsecase.RemoveRoleFromUser(c.Request.Context(), req.UserID, req.RoleName); err != nil {
		log.Error().Err(err).Str("user_id", req.UserID).Str("role_name", req.RoleName).Msg("Failed to remove role from user")
		status := http.StatusInternalServerError
		if err == domain.ErrUserNotFound || err == domain.ErrRoleNotFound {
			status = http.StatusNotFound
		}
		c.JSON(status, domain.ErrorResponse{
			Detail: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, domain.MessageResponse{
		Detail: "Role removed successfully",
	})
}

// GetUsersWithRole adalah handler untuk mengambil daftar user dengan role tertentu
// Endpoint: GET /roles/role/:role_name/users
func (h *RoleHandler) GetUsersWithRole(c *gin.Context) {
	log := logger.GetLoggerFromGinContext(c)
	roleName := c.Param("role_name")

	users, err := h.roleUsecase.GetUsersWithRole(c.Request.Context(), roleName)
	if err != nil {
		log.Error().Err(err).Str("role_name", roleName).Msg("Failed to get users with role")
		c.JSON(http.StatusInternalServerError, domain.ErrorResponse{
			Detail: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, users)
}

// ========== User Roles & Permissions Query ==========

// GetUserRoles adalah handler untuk mengambil roles user tertentu (admin only)
// Endpoint: GET /roles/user/:user_id
func (h *RoleHandler) GetUserRoles(c *gin.Context) {
	log := logger.GetLoggerFromGinContext(c)
	userID := c.Param("user_id")

	roles, err := h.roleUsecase.GetUserRoles(c.Request.Context(), userID)
	if err != nil {
		log.Error().Err(err).Str("user_id", userID).Msg("Failed to get user roles")
		c.JSON(http.StatusInternalServerError, domain.ErrorResponse{
			Detail: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, roles)
}

// GetUserPermissions adalah handler untuk mengambil permissions user tertentu (admin only)
// Endpoint: GET /roles/user/:user_id/permissions
func (h *RoleHandler) GetUserPermissions(c *gin.Context) {
	log := logger.GetLoggerFromGinContext(c)
	userID := c.Param("user_id")

	permissions, err := h.roleUsecase.GetUserPermissions(c.Request.Context(), userID)
	if err != nil {
		log.Error().Err(err).Str("user_id", userID).Msg("Failed to get user permissions")
		c.JSON(http.StatusInternalServerError, domain.ErrorResponse{
			Detail: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, permissions)
}

// CheckUserPermission adalah handler untuk cek apakah user punya permission tertentu
// Endpoint: GET /roles/user/:user_id/permissions/:permission_name
// Response: { "has_permission": true/false }
func (h *RoleHandler) CheckUserPermission(c *gin.Context) {
	log := logger.GetLoggerFromGinContext(c)
	userID := c.Param("user_id")
	permissionName := c.Param("permission_name")

	hasPermission, err := h.roleUsecase.CheckUserPermission(c.Request.Context(), userID, permissionName)
	if err != nil {
		log.Error().Err(err).Str("user_id", userID).Str("permission", permissionName).Msg("Failed to check user permission")
		c.JSON(http.StatusInternalServerError, domain.ErrorResponse{
			Detail: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"has_permission": hasPermission,
	})
}

// ========== Analytics ==========

// GetRoleStatistics adalah handler untuk mengambil statistik roles
// Endpoint: GET /roles/analytics/statistics
// Return: jumlah user per role, total roles, dll
func (h *RoleHandler) GetRoleStatistics(c *gin.Context) {
	log := logger.GetLoggerFromGinContext(c)
	stats, err := h.roleUsecase.GetRoleStatistics(c.Request.Context())
	if err != nil {
		log.Error().Err(err).Msg("Failed to get role statistics")
		c.JSON(http.StatusInternalServerError, domain.ErrorResponse{
			Detail: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, stats)
}

// ========== Current User (Self) ==========
// Endpoint untuk user mengakses roles/permissions miliknya sendiri

// GetMyRoles adalah handler untuk mengambil roles user yang sedang login
// Endpoint: GET /roles/me/roles
func (h *RoleHandler) GetMyRoles(c *gin.Context) {
	log := logger.GetLoggerFromGinContext(c)
	userID := middleware.GetUserID(c)

	roles, err := h.roleUsecase.GetUserRoles(c.Request.Context(), userID)
	if err != nil {
		log.Error().Err(err).Str("user_id", userID).Msg("Failed to get my roles")
		c.JSON(http.StatusInternalServerError, domain.ErrorResponse{
			Detail: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, roles)
}

// GetMyPermissions adalah handler untuk mengambil permissions user yang sedang login
// Endpoint: GET /roles/me/permissions
func (h *RoleHandler) GetMyPermissions(c *gin.Context) {
	log := logger.GetLoggerFromGinContext(c)
	userID := middleware.GetUserID(c)

	permissions, err := h.roleUsecase.GetUserPermissions(c.Request.Context(), userID)
	if err != nil {
		log.Error().Err(err).Str("user_id", userID).Msg("Failed to get my permissions")
		c.JSON(http.StatusInternalServerError, domain.ErrorResponse{
			Detail: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, permissions)
}

// CheckMyPermission adalah handler untuk cek apakah user yang login punya permission tertentu
// Endpoint: GET /roles/me/permissions/:permission_name
// Response: { "has_permission": true/false }
func (h *RoleHandler) CheckMyPermission(c *gin.Context) {
	log := logger.GetLoggerFromGinContext(c)
	userID := middleware.GetUserID(c)
	permissionName := c.Param("permission_name")

	hasPermission, err := h.roleUsecase.CheckUserPermission(c.Request.Context(), userID, permissionName)
	if err != nil {
		log.Error().Err(err).Str("user_id", userID).Str("permission", permissionName).Msg("Failed to check my permission")
		c.JSON(http.StatusInternalServerError, domain.ErrorResponse{
			Detail: err.Error(),
		})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"has_permission": hasPermission,
	})
}
