package usecase

import (
	"context"
	"errors"
	"fmt"

	"auth-service/internal/domain"
	"auth-service/internal/repository"

	"github.com/google/uuid"
)

// roleUsecase implements RoleUsecase interface
// Handle business logic untuk role, permission, dan user-role assignment
type roleUsecase struct {
	roleRepo       repository.RoleRepository       // Repository untuk CRUD roles
	userRepo       repository.UserRepository       // Repository untuk user operations
	permissionRepo repository.PermissionRepository // Repository untuk permissions
}

// NewRoleUsecase create instance baru dengan dependency injection
func NewRoleUsecase(
	roleRepo repository.RoleRepository,
	userRepo repository.UserRepository,
	permissionRepo repository.PermissionRepository,
) RoleUsecase {
	return &roleUsecase{
		roleRepo:       roleRepo,
		userRepo:       userRepo,
		permissionRepo: permissionRepo,
	}
}

// ===== Role CRUD =====

// GetAllRoles get semua roles, bisa include inactive roles
func (u *roleUsecase) GetAllRoles(ctx context.Context, includeInactive bool) ([]domain.RoleResponse, error) {
	roles, err := u.roleRepo.FindAll(ctx, includeInactive)
	if err != nil {
		return nil, err
	}
	return u.toRoleResponses(roles), nil
}

// GetRoleByID get role berdasarkan ID
func (u *roleUsecase) GetRoleByID(ctx context.Context, id string) (*domain.RoleResponse, error) {
	// Parse ID string to UUID
	roleUUID, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("invalid role ID: %w", err)
	}

	role, err := u.roleRepo.FindByID(ctx, roleUUID)
	if err != nil {
		return nil, err
	}
	return u.toRoleResponse(role), nil
}

// GetRoleByName get role berdasarkan nama (admin, user, moderator, dll)
func (u *roleUsecase) GetRoleByName(ctx context.Context, name string) (*domain.RoleResponse, error) {
	role, err := u.roleRepo.FindByName(ctx, name)
	if err != nil {
		return nil, err
	}
	return u.toRoleResponse(role), nil
}

// CreateRole create role baru dengan validasi duplikasi nama
func (u *roleUsecase) CreateRole(ctx context.Context, req *domain.RoleCreate) (*domain.RoleResponse, error) {
	// Cek duplikasi nama role
	_, err := u.roleRepo.FindByName(ctx, req.Name)
	if err == nil {
		// Role found - already exists
		return nil, domain.ErrRoleAlreadyExists
	}
	if !errors.Is(err, domain.ErrRoleNotFound) {
		// Database error (not "not found")
		return nil, err
	}

	// Create role baru (default is_active = true)
	role := &domain.Role{
		Name:        req.Name,
		Description: req.Description,
		IsActive:    true,
	}

	if err := u.roleRepo.Create(ctx, role); err != nil {
		return nil, fmt.Errorf("failed to create role: %w", err)
	}

	return u.toRoleResponse(role), nil
}

// UpdateRole update role (partial update: hanya field yang diisi yang diupdate)
// Validasi konflik nama jika nama diubah
func (u *roleUsecase) UpdateRole(ctx context.Context, id string, req *domain.RoleUpdate) (*domain.RoleResponse, error) {
	// Parse ID string to UUID
	roleUUID, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("invalid role ID: %w", err)
	}

	role, err := u.roleRepo.FindByID(ctx, roleUUID)
	if err != nil {
		if errors.Is(err, domain.ErrRoleNotFound) {
			return nil, domain.ErrRoleNotFound
		}
		return nil, err
	}

	// Update nama jika ada & cek konflik
	if req.Name != "" {
		existing, err := u.roleRepo.FindByName(ctx, req.Name)
		if err == nil && existing.ID != roleUUID {
			// Different role with same name exists
			return nil, domain.ErrRoleAlreadyExists
		}
		// Ignore ErrRoleNotFound (name available)
		role.Name = req.Name
	}

	if req.Description != "" {
		role.Description = req.Description
	}

	if req.IsActive != nil {
		role.IsActive = *req.IsActive
	}

	if err := u.roleRepo.Update(ctx, role); err != nil {
		return nil, fmt.Errorf("failed to update role: %w", err)
	}

	return u.toRoleResponse(role), nil
}

// DeleteRole hapus role berdasarkan ID
// System roles (admin, user) protected - tidak bisa dihapus
func (u *roleUsecase) DeleteRole(ctx context.Context, id string) error {
	// Parse ID string to UUID
	roleUUID, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("invalid role ID: %w", err)
	}

	role, err := u.roleRepo.FindByID(ctx, roleUUID)
	if err != nil {
		if errors.Is(err, domain.ErrRoleNotFound) {
			return domain.ErrRoleNotFound
		}
		return err
	}

	// Proteksi system roles
	if role.Name == "admin" || role.Name == "user" {
		return domain.ErrRoleInUse
	}

	return u.roleRepo.Delete(ctx, roleUUID)
}

// ===== Permission Management =====

// GetAllPermissions get semua permissions, optional filter by resource
func (u *roleUsecase) GetAllPermissions(ctx context.Context, resource string) ([]domain.PermissionResponse, error) {
	permissions, err := u.permissionRepo.FindAll(ctx, resource)
	if err != nil {
		return nil, err
	}
	return u.toPermissionResponses(permissions), nil
}

// AddPermissionsToRole tambah permissions ke role
func (u *roleUsecase) AddPermissionsToRole(ctx context.Context, roleID string, permissions []string) error {
	// Parse roleID string to UUID
	roleUUID, err := uuid.Parse(roleID)
	if err != nil {
		return fmt.Errorf("invalid role ID: %w", err)
	}

	_, err = u.roleRepo.FindByID(ctx, roleUUID)
	if err != nil {
		if errors.Is(err, domain.ErrRoleNotFound) {
			return domain.ErrRoleNotFound
		}
		return err
	}
	return u.roleRepo.AddPermissions(ctx, roleUUID, permissions)
}

// RemovePermissionsFromRole hapus permissions dari role
func (u *roleUsecase) RemovePermissionsFromRole(ctx context.Context, roleID string, permissions []string) error {
	// Parse roleID string to UUID
	roleUUID, err := uuid.Parse(roleID)
	if err != nil {
		return fmt.Errorf("invalid role ID: %w", err)
	}

	_, err = u.roleRepo.FindByID(ctx, roleUUID)
	if err != nil {
		if errors.Is(err, domain.ErrRoleNotFound) {
			return domain.ErrRoleNotFound
		}
		return err
	}
	return u.roleRepo.RemovePermissions(ctx, roleUUID, permissions)
}

// ===== User-Role Assignment =====

// AssignRoleToUser assign role ke user by role name
func (u *roleUsecase) AssignRoleToUser(ctx context.Context, userID string, roleName string) error {
	// Parse userID string to UUID
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("invalid user ID: %w", err)
	}

	_, err = u.userRepo.FindByID(ctx, userUUID)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			return domain.ErrUserNotFound
		}
		return err
	}

	role, err := u.roleRepo.FindByName(ctx, roleName)
	if err != nil {
		if errors.Is(err, domain.ErrRoleNotFound) {
			return domain.ErrRoleNotFound
		}
		return err
	}

	return u.roleRepo.AssignToUser(ctx, userUUID, role.ID)
}

// RemoveRoleFromUser hapus role dari user
func (u *roleUsecase) RemoveRoleFromUser(ctx context.Context, userID string, roleName string) error {
	// Parse userID string to UUID
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return fmt.Errorf("invalid user ID: %w", err)
	}

	_, err = u.userRepo.FindByID(ctx, userUUID)
	if err != nil {
		if errors.Is(err, domain.ErrUserNotFound) {
			return domain.ErrUserNotFound
		}
		return err
	}

	role, err := u.roleRepo.FindByName(ctx, roleName)
	if err != nil {
		if errors.Is(err, domain.ErrRoleNotFound) {
			return domain.ErrRoleNotFound
		}
		return err
	}

	return u.roleRepo.RemoveFromUser(ctx, userUUID, role.ID)
}

// GetUsersWithRole get semua user yang punya role tertentu
func (u *roleUsecase) GetUsersWithRole(ctx context.Context, roleName string) ([]domain.UserWithRolesResponse, error) {
	users, err := u.roleRepo.GetUsersWithRoles(ctx, roleName)
	if err != nil {
		return nil, err
	}

	// Transform ke DTO dengan roles
	responses := make([]domain.UserWithRolesResponse, len(users))
	for i, user := range users {
		// Extract role names dari user.Roles
		roleNames := make([]string, len(user.Roles))
		for j, role := range user.Roles {
			roleNames[j] = role.Name
		}

		responses[i] = domain.UserWithRolesResponse{
			UserID:    user.ID.String(), // Convert UUID to string for JSON
			Username:  user.Username,
			Email:     user.Email,
			FullName:  user.FullName,
			IsActive:  user.IsActive,
			Roles:     roleNames,
			LastLogin: user.LastLogin,
		}
	}
	return responses, nil
}

// GetUserRoles get semua roles user
func (u *roleUsecase) GetUserRoles(ctx context.Context, userID string) ([]domain.RoleResponse, error) {
	// Parse userID string to UUID
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	roles, err := u.roleRepo.GetUserRoles(ctx, userUUID)
	if err != nil {
		return nil, err
	}
	return u.toRoleResponses(roles), nil
}

// GetUserPermissions get semua permission names user (aggregated dari roles)
func (u *roleUsecase) GetUserPermissions(ctx context.Context, userID string) ([]string, error) {
	// Parse userID string to UUID
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return nil, fmt.Errorf("invalid user ID: %w", err)
	}

	return u.userRepo.GetUserPermissions(ctx, userUUID)
}

// CheckUserPermission cek apakah user punya permission tertentu
func (u *roleUsecase) CheckUserPermission(ctx context.Context, userID string, permission string) (bool, error) {
	// Parse userID string to UUID
	userUUID, err := uuid.Parse(userID)
	if err != nil {
		return false, fmt.Errorf("invalid user ID: %w", err)
	}

	return u.userRepo.HasPermission(ctx, userUUID, permission)
}

// ===== Analytics =====

// GetRoleStatistics get statistik roles (total, active, dll)
func (u *roleUsecase) GetRoleStatistics(ctx context.Context) (*domain.RoleStatistics, error) {
	return u.roleRepo.GetRoleStatistics(ctx)
}

// ===== Helper Functions =====

// toRoleResponse convert domain.Role → domain.RoleResponse (DTO)
func (u *roleUsecase) toRoleResponse(role *domain.Role) *domain.RoleResponse {
	resp := &domain.RoleResponse{
		ID:          role.ID.String(), // Convert UUID to string for JSON
		Name:        role.Name,
		Description: role.Description,
		IsActive:    role.IsActive,
	}

	if len(role.Permissions) > 0 {
		resp.Permissions = u.toPermissionResponses(role.Permissions)
	}

	return resp
}

// toRoleResponses batch convert domain.Role → domain.RoleResponse
func (u *roleUsecase) toRoleResponses(roles []domain.Role) []domain.RoleResponse {
	responses := make([]domain.RoleResponse, len(roles))
	for i, role := range roles {
		responses[i] = *u.toRoleResponse(&role)
	}
	return responses
}

// toPermissionResponses convert domain.Permission → domain.PermissionResponse
func (u *roleUsecase) toPermissionResponses(permissions []domain.Permission) []domain.PermissionResponse {
	responses := make([]domain.PermissionResponse, len(permissions))
	for i, p := range permissions {
		responses[i] = domain.PermissionResponse{
			ID:          p.ID.String(), // Convert UUID to string for JSON
			Name:        p.Name,
			Resource:    p.Resource,
			Action:      p.Action,
			Description: p.Description,
		}
	}
	return responses
}
