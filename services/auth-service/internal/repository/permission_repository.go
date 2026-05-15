package repository

import (
	"context"
	"errors"

	"auth-service/internal/domain"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// permissionRepository adalah implementasi PermissionRepository interface
// Mengelola data permissions di database
type permissionRepository struct {
	db *gorm.DB
}

// NewPermissionRepository membuat instance baru permissionRepository
func NewPermissionRepository(db *gorm.DB) PermissionRepository {
	return &permissionRepository{db: db}
}

// Create membuat permission baru
func (r *permissionRepository) Create(ctx context.Context, permission *domain.Permission) error {
	return r.db.WithContext(ctx).Create(permission).Error
}

// FindByID mencari permission berdasarkan ID
func (r *permissionRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Permission, error) {
	var permission domain.Permission
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&permission).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrPermissionNotFound
	}
	if err != nil {
		return nil, err
	}
	return &permission, nil
}

// FindAll mengambil semua permissions, bisa difilter by resource
// Jika resource kosong, return semua permissions
func (r *permissionRepository) FindAll(ctx context.Context, resource string) ([]domain.Permission, error) {
	var permissions []domain.Permission
	query := r.db.WithContext(ctx)

	// Filter by resource jika diberikan
	if resource != "" {
		query = query.Where("resource = ?", resource)
	}

	err := query.Find(&permissions).Error
	return permissions, err
}

// FindByName mencari permission berdasarkan nama (misal: "user.read")
// Return ErrPermissionNotFound jika tidak ditemukan
func (r *permissionRepository) FindByName(ctx context.Context, name string) (*domain.Permission, error) {
	var permission domain.Permission
	err := r.db.WithContext(ctx).Where("name = ?", name).First(&permission).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrPermissionNotFound
	}
	if err != nil {
		return nil, err
	}
	return &permission, nil
}

// FindByNames mencari multiple permissions sekaligus berdasarkan nama
// Berguna untuk batch operations (assign multiple permissions ke role)
func (r *permissionRepository) FindByNames(ctx context.Context, names []string) ([]domain.Permission, error) {
	var permissions []domain.Permission
	err := r.db.WithContext(ctx).Where("name IN ?", names).Find(&permissions).Error
	return permissions, err
}

// Update mengupdate data permission
func (r *permissionRepository) Update(ctx context.Context, permission *domain.Permission) error {
	return r.db.WithContext(ctx).Save(permission).Error
}

// Delete menghapus permission dan relasinya dengan role
func (r *permissionRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Hapus dari role_permissions
		if err := tx.Where("permission_id = ?", id).Delete(&domain.RolePermission{}).Error; err != nil {
			return err
		}

		// Hapus permission
		return tx.Delete(&domain.Permission{}, id).Error
	})
}
