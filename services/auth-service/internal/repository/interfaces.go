package repository

import (
	"context"
	"time"

	"auth-service/internal/domain"

	"github.com/google/uuid"
)

// UserRepository interface for user operations
type UserRepository interface {
	Create(ctx context.Context, user *domain.User) error
	FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error)
	FindByUsername(ctx context.Context, username string) (*domain.User, error)
	FindByEmail(ctx context.Context, email string) (*domain.User, error)
	FindByUsernameOrEmail(ctx context.Context, identifier string) (*domain.User, error)
	Update(ctx context.Context, user *domain.User) error
	Delete(ctx context.Context, id uuid.UUID) error
	UpdateLastLogin(ctx context.Context, id uuid.UUID) error
	UpdateLastActivity(ctx context.Context, id uuid.UUID) error
	GetUserWithRoles(ctx context.Context, id uuid.UUID) (*domain.User, error)
	GetUserPermissions(ctx context.Context, id uuid.UUID) ([]string, error)
	HasPermission(ctx context.Context, userID uuid.UUID, permissionName string) (bool, error)
}

// RoleRepository interface for role operations
type RoleRepository interface {
	Create(ctx context.Context, role *domain.Role) error
	FindByID(ctx context.Context, id uuid.UUID) (*domain.Role, error)
	FindByName(ctx context.Context, name string) (*domain.Role, error)
	FindAll(ctx context.Context, includeInactive bool) ([]domain.Role, error)
	Update(ctx context.Context, role *domain.Role) error
	Delete(ctx context.Context, id uuid.UUID) error
	AssignToUser(ctx context.Context, userID uuid.UUID, roleID uuid.UUID) error
	RemoveFromUser(ctx context.Context, userID uuid.UUID, roleID uuid.UUID) error
	GetUserRoles(ctx context.Context, userID uuid.UUID) ([]domain.Role, error)
	GetUsersWithRoles(ctx context.Context, roleName string) ([]domain.User, error)
	AddPermissions(ctx context.Context, roleID uuid.UUID, permissionNames []string) error
	RemovePermissions(ctx context.Context, roleID uuid.UUID, permissionNames []string) error
	GetRoleStatistics(ctx context.Context) (*domain.RoleStatistics, error)
}

// PermissionRepository interface for permission operations
type PermissionRepository interface {
	Create(ctx context.Context, permission *domain.Permission) error
	FindByID(ctx context.Context, id uuid.UUID) (*domain.Permission, error)
	FindAll(ctx context.Context, resource string) ([]domain.Permission, error)
	FindByName(ctx context.Context, name string) (*domain.Permission, error)
	FindByNames(ctx context.Context, names []string) ([]domain.Permission, error)
	Update(ctx context.Context, permission *domain.Permission) error
	Delete(ctx context.Context, id uuid.UUID) error
}

// RefreshTokenRepository interface for refresh token operations
type RefreshTokenRepository interface {
	Create(ctx context.Context, token *domain.RefreshToken) error
	FindByHash(ctx context.Context, hash string) (*domain.RefreshToken, error)
	Invalidate(ctx context.Context, hash string) error
	InvalidateAllForUser(ctx context.Context, userID uuid.UUID) error
	DeleteExpired(ctx context.Context) error
}

// UserActivityRepository interface for user activity operations
type UserActivityRepository interface {
	Create(ctx context.Context, activity *domain.UserActivity) error
	FindByUserID(ctx context.Context, userID uuid.UUID, query *domain.ActivityQuery) ([]domain.UserActivity, error)
	GetRecentByUserID(ctx context.Context, userID uuid.UUID, limit int) ([]domain.UserActivity, error)
	GetSummary(ctx context.Context, userID uuid.UUID, days int) (*domain.ActivitySummary, error)
	DeleteOlderThan(ctx context.Context, before time.Time) error
}
