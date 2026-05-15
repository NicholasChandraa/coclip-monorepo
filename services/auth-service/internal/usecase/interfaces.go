package usecase

import (
	"context"

	"auth-service/internal/domain"
)

// AuthUsecase interface defines methods for authentication operations
type AuthUsecase interface {
	Register(ctx context.Context, req *domain.UserCreate, ip, userAgent, deviceInfo string) (*domain.UserResponse, error)
	Login(ctx context.Context, req *domain.LoginRequest, ip, userAgent, deviceInfo string) (*domain.TokenResponse, string, error)
	RefreshToken(ctx context.Context, refreshToken, ip, userAgent, deviceInfo string) (*domain.TokenResponse, string, error)
	Logout(ctx context.Context, userID, refreshToken string) error
	LogoutAll(ctx context.Context, userID string) error
	GetCurrentUser(ctx context.Context, userID string) (*domain.UserWithPermissionsResponse, error)
	GetUserByID(ctx context.Context, userID string) (*domain.UserResponse, error)
	UpdateProfile(ctx context.Context, userID string, req *domain.UserUpdate, ip, userAgent, deviceInfo string) (*domain.UserResponse, error)

	ChangePassword(ctx context.Context, userID string, req *domain.ChangePasswordRequest, ip, userAgent, deviceInfo string) error
	GetActivities(ctx context.Context, userID string, query *domain.ActivityQuery) ([]domain.ActivityResponse, error)
	GetActivitySummary(ctx context.Context, userID string, days int) (*domain.ActivitySummary, error)
	GetRecentActivities(ctx context.Context, userID string, limit int) ([]domain.ActivityResponse, error)
	UpdateUserStatus(ctx context.Context, targetUserID string, isActive bool, adminID, ip, userAgent, deviceInfo string) error
	DeleteUser(ctx context.Context, targetUserID, adminID, ip, userAgent, deviceInfo string) error
}

// RoleUsecase interface defines methods for role management operations
type RoleUsecase interface {
	GetAllRoles(ctx context.Context, includeInactive bool) ([]domain.RoleResponse, error)
	GetRoleByID(ctx context.Context, id string) (*domain.RoleResponse, error)
	GetRoleByName(ctx context.Context, name string) (*domain.RoleResponse, error)
	CreateRole(ctx context.Context, req *domain.RoleCreate) (*domain.RoleResponse, error)
	UpdateRole(ctx context.Context, id string, req *domain.RoleUpdate) (*domain.RoleResponse, error)
	DeleteRole(ctx context.Context, id string) error
	GetAllPermissions(ctx context.Context, resource string) ([]domain.PermissionResponse, error)
	AddPermissionsToRole(ctx context.Context, roleID string, permissions []string) error
	RemovePermissionsFromRole(ctx context.Context, roleID string, permissions []string) error
	AssignRoleToUser(ctx context.Context, userID string, roleName string) error
	RemoveRoleFromUser(ctx context.Context, userID string, roleName string) error
	GetUsersWithRole(ctx context.Context, roleName string) ([]domain.UserWithRolesResponse, error)
	GetUserRoles(ctx context.Context, userID string) ([]domain.RoleResponse, error)
	GetUserPermissions(ctx context.Context, userID string) ([]string, error)
	CheckUserPermission(ctx context.Context, userID string, permission string) (bool, error)
	GetRoleStatistics(ctx context.Context) (*domain.RoleStatistics, error)
}

// PermissionUsecase interface defines methods for permission management operations
type PermissionUsecase interface {
	GetAllPermissions(ctx context.Context, resource string) ([]domain.PermissionResponse, error)
	GetPermissionByID(ctx context.Context, id string) (*domain.PermissionResponse, error)
	CreatePermission(ctx context.Context, req *domain.PermissionCreate) (*domain.PermissionResponse, error)
	UpdatePermission(ctx context.Context, id string, req *domain.PermissionUpdate) (*domain.PermissionResponse, error)
	DeletePermission(ctx context.Context, id string) error
}
