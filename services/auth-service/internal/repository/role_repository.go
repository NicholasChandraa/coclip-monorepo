package repository

import (
	"context"
	"errors"

	"auth-service/internal/domain"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// roleRepository adalah implementasi RoleRepository interface
// Mengelola roles, user-role assignments, dan role-permission mappings
type roleRepository struct {
	db *gorm.DB
}

// NewRoleRepository membuat instance baru roleRepository
func NewRoleRepository(db *gorm.DB) RoleRepository {
	return &roleRepository{db: db}
}

// ========== Role CRUD ==========

// Create membuat role baru di database
func (r *roleRepository) Create(ctx context.Context, role *domain.Role) error {
	return r.db.WithContext(ctx).Create(role).Error
}

// FindByID mencari role berdasarkan ID, termasuk permissions-nya
// Return ErrRoleNotFound jika tidak ditemukan
func (r *roleRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.Role, error) {
	var role domain.Role
	err := r.db.WithContext(ctx).Preload("Permissions").Where("id = ?", id).First(&role).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrRoleNotFound
	}
	if err != nil {
		return nil, err
	}
	return &role, nil
}

// FindByName mencari role berdasarkan nama, termasuk permissions-nya
// Return ErrRoleNotFound jika tidak ditemukan
func (r *roleRepository) FindByName(ctx context.Context, name string) (*domain.Role, error) {
	var role domain.Role
	err := r.db.WithContext(ctx).Preload("Permissions").Where("name = ?", name).First(&role).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrRoleNotFound
	}
	if err != nil {
		return nil, err
	}
	return &role, nil
}

// FindAll mengambil semua roles, bisa include/exclude inactive roles
func (r *roleRepository) FindAll(ctx context.Context, includeInactive bool) ([]domain.Role, error) {
	var roles []domain.Role
	query := r.db.WithContext(ctx).Preload("Permissions")

	if !includeInactive {
		query = query.Where("is_active = ?", true)
	}

	err := query.Find(&roles).Error
	return roles, err
}

// Update mengupdate data role
func (r *roleRepository) Update(ctx context.Context, role *domain.Role) error {
	return r.db.WithContext(ctx).Save(role).Error
}

// Delete menghapus role beserta semua relasinya (user_roles, role_permissions)
// Menggunakan transaction untuk menjaga data consistency
func (r *roleRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Hapus relasi role dari users dulu
		if err := tx.Where("role_id = ?", id).Delete(&domain.UserRole{}).Error; err != nil {
			return err
		}

		// Hapus relasi permissions dari role
		if err := tx.Where("role_id = ?", id).Delete(&domain.RolePermission{}).Error; err != nil {
			return err
		}

		// Baru hapus role-nya
		return tx.Delete(&domain.Role{}, id).Error
	})
}

// ========== User-Role Assignment ==========

// AssignToUser assign role ke user (replace existing role)
// User hanya bisa punya 1 role pada saat ini (single role per user)
func (r *roleRepository) AssignToUser(ctx context.Context, userID uuid.UUID, roleID uuid.UUID) error {
	// Hapus semua role existing dari user dulu
	if err := r.db.WithContext(ctx).Where("user_id = ?", userID).Delete(&domain.UserRole{}).Error; err != nil {
		return err
	}

	// Assign role baru
	userRole := domain.UserRole{
		UserID: userID,
		RoleID: roleID,
	}

	return r.db.WithContext(ctx).Create(&userRole).Error
}

// RemoveFromUser menghapus role tertentu dari user
func (r *roleRepository) RemoveFromUser(ctx context.Context, userID uuid.UUID, roleID uuid.UUID) error {
	return r.db.WithContext(ctx).Where("user_id = ? AND role_id = ?", userID, roleID).Delete(&domain.UserRole{}).Error
}

// GetUserRoles mengambil semua roles yang dimiliki user
func (r *roleRepository) GetUserRoles(ctx context.Context, userID uuid.UUID) ([]domain.Role, error) {
	var roles []domain.Role
	err := r.db.WithContext(ctx).Raw(`
		SELECT r.* FROM roles r
		JOIN user_roles ur ON r.id = ur.role_id
		WHERE ur.user_id = ?
	`, userID).Scan(&roles).Error
	return roles, err
}

// GetUsersWithRoles mengambil semua users yang punya role tertentu
// Menggunakan Preload untuk load roles setiap user
func (r *roleRepository) GetUsersWithRoles(ctx context.Context, roleName string) ([]domain.User, error) {
	var users []domain.User
	err := r.db.WithContext(ctx).
		Preload("Roles").
		Joins("JOIN user_roles ur ON users.id = ur.user_id").
		Joins("JOIN roles r ON ur.role_id = r.id").
		Where("r.name = ?", roleName).
		Find(&users).Error
	return users, err
}

// ========== Role-Permission Management ==========

// AddPermissions menambahkan permissions ke role
// Menggunakan FirstOrCreate untuk menghindari duplicate error
func (r *roleRepository) AddPermissions(ctx context.Context, roleID uuid.UUID, permissionNames []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		for _, name := range permissionNames {
			var permission domain.Permission
			if err := tx.Where("name = ?", name).First(&permission).Error; err != nil {
				// Skip jika permission tidak ditemukan
				continue
			}

			// Buat record role_permission
			rp := domain.RolePermission{
				RoleID:       roleID,
				PermissionID: permission.ID,
			}

			// Pakai FirstOrCreate untuk avoid duplicate errors
			result := tx.Where("role_id = ? AND permission_id = ?", roleID, permission.ID).
				FirstOrCreate(&rp)

			if result.Error != nil {
				return result.Error
			}
		}

		return nil
	})
}

// RemovePermissions menghapus permissions dari role
func (r *roleRepository) RemovePermissions(ctx context.Context, roleID uuid.UUID, permissionNames []string) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Cari permission IDs dari nama-nama yang diberikan
		var permissionIDs []uuid.UUID
		if err := tx.Model(&domain.Permission{}).Where("name IN ?", permissionNames).Pluck("id", &permissionIDs).Error; err != nil {
			return err
		}

		// Hapus dari role_permissions
		return tx.Where("role_id = ? AND permission_id IN ?", roleID, permissionIDs).Delete(&domain.RolePermission{}).Error
	})
}

// ========== Analytics ==========

// GetRoleStatistics mengambil statistik roles (total, active, breakdown per role)
func (r *roleRepository) GetRoleStatistics(ctx context.Context) (*domain.RoleStatistics, error) {
	var stats domain.RoleStatistics

	// Total roles
	r.db.WithContext(ctx).Model(&domain.Role{}).Count(&stats.TotalRoles)

	// Active roles
	r.db.WithContext(ctx).Model(&domain.Role{}).Where("is_active = ?", true).Count(&stats.ActiveRoles)

	// Breakdown: jumlah user per role
	r.db.WithContext(ctx).Raw(`
		SELECT r.name as role_name, COUNT(ur.user_id) as user_count
		FROM roles r
		LEFT JOIN user_roles ur ON r.id = ur.role_id
		GROUP BY r.id, r.name
		ORDER BY user_count DESC
	`).Scan(&stats.RolesBreakdown)

	return &stats, nil
}
