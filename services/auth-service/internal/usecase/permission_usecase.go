package usecase

import (
	"context"
	"errors"
	"fmt"

	"auth-service/internal/domain"
	"auth-service/internal/repository"

	"github.com/google/uuid"
)

// permissionUsecase implements PermissionUsecase interface
// Handle business logic for permissions
type permissionUsecase struct {
	permissionRepo repository.PermissionRepository
	roleRepo       repository.RoleRepository // Needed for dependency checks on delete?
}

// NewPermissionUsecase create new instance with dependency injection
func NewPermissionUsecase(
	permissionRepo repository.PermissionRepository,
	roleRepo repository.RoleRepository,
) PermissionUsecase {
	return &permissionUsecase{
		permissionRepo: permissionRepo,
		roleRepo:       roleRepo, // Inject if needed for future validation
	}
}

// GetAllPermissions get all permissions, optional filtered by resource
func (u *permissionUsecase) GetAllPermissions(ctx context.Context, resource string) ([]domain.PermissionResponse, error) {
	permissions, err := u.permissionRepo.FindAll(ctx, resource)
	if err != nil {
		return nil, err
	}
	return u.toPermissionResponses(permissions), nil
}

// GetPermissionByID get permission by ID
func (u *permissionUsecase) GetPermissionByID(ctx context.Context, id string) (*domain.PermissionResponse, error) {
	// Parse ID string to UUID
	permUUID, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("invalid permission ID: %w", err)
	}

	permission, err := u.permissionRepo.FindByID(ctx, permUUID)
	if err != nil {
		return nil, err
	}
	return u.toPermissionResponse(permission), nil
}

// CreatePermission create new permission
func (u *permissionUsecase) CreatePermission(ctx context.Context, req *domain.PermissionCreate) (*domain.PermissionResponse, error) {
	// Check duplicate name
	_, err := u.permissionRepo.FindByName(ctx, req.Name)
	if err == nil {
		return nil, domain.ErrPermissionAlreadyExists
	}
	if !errors.Is(err, domain.ErrPermissionNotFound) {
		return nil, err
	}

	permission := &domain.Permission{
		Name:        req.Name,
		Description: req.Description,
		Resource:    req.Resource,
		Action:      req.Action,
	}

	if err := u.permissionRepo.Create(ctx, permission); err != nil {
		return nil, fmt.Errorf("failed to create permission: %w", err)
	}

	return u.toPermissionResponse(permission), nil
}

// UpdatePermission update existing permission
func (u *permissionUsecase) UpdatePermission(ctx context.Context, id string, req *domain.PermissionUpdate) (*domain.PermissionResponse, error) {
	// Parse ID string to UUID
	permUUID, err := uuid.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("invalid permission ID: %w", err)
	}

	permission, err := u.permissionRepo.FindByID(ctx, permUUID)
	if err != nil {
		if errors.Is(err, domain.ErrPermissionNotFound) {
			return nil, domain.ErrPermissionNotFound
		}
		return nil, err
	}

	// If name changes, check collision
	if req.Name != "" && req.Name != permission.Name {
		existing, err := u.permissionRepo.FindByName(ctx, req.Name)
		if err == nil && existing.ID != permUUID {
			return nil, domain.ErrPermissionAlreadyExists
		}
		permission.Name = req.Name
	}

	if req.Description != "" {
		permission.Description = req.Description
	}
	if req.Resource != "" {
		permission.Resource = req.Resource
	}
	if req.Action != "" {
		permission.Action = req.Action
	}

	if err := u.permissionRepo.Update(ctx, permission); err != nil {
		return nil, fmt.Errorf("failed to update permission: %w", err)
	}

	return u.toPermissionResponse(permission), nil
}

// DeletePermission delete permission
func (u *permissionUsecase) DeletePermission(ctx context.Context, id string) error {
	// Parse ID string to UUID
	permUUID, err := uuid.Parse(id)
	if err != nil {
		return fmt.Errorf("invalid permission ID: %w", err)
	}

	// Check if exists
	_, err = u.permissionRepo.FindByID(ctx, permUUID)
	if err != nil {
		if errors.Is(err, domain.ErrPermissionNotFound) {
			return domain.ErrPermissionNotFound
		}
		return err
	}

	// NOTE: We could check if permission is assigned to roles here if we want to prevent deletion
	// But repository Delete implementation already handles cascading or cleanup (Delete RolePermission)
	// So we proceed with deletion.

	return u.permissionRepo.Delete(ctx, permUUID)
}

// Helpers

func (u *permissionUsecase) toPermissionResponse(p *domain.Permission) *domain.PermissionResponse {
	return &domain.PermissionResponse{
		ID:          p.ID.String(),
		Name:        p.Name,
		Resource:    p.Resource,
		Action:      p.Action,
		Description: p.Description,
	}
}

func (u *permissionUsecase) toPermissionResponses(permissions []domain.Permission) []domain.PermissionResponse {
	responses := make([]domain.PermissionResponse, len(permissions))
	for i, p := range permissions {
		responses[i] = *u.toPermissionResponse(&p)
	}
	return responses
}
