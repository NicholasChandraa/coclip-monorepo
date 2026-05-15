package repository

import (
	"context"
	"errors"
	"time"

	"auth-service/internal/domain"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// userRepository adalah implementasi UserRepository interface
// Mengelola data user di database (CRUD, lookup, permission check)
type userRepository struct {
	db *gorm.DB
}

// NewUserRepository membuat instance baru userRepository
func NewUserRepository(db *gorm.DB) UserRepository {
	return &userRepository{db: db}
}

// ========== User CRUD ==========

// Create menyimpan user baru ke database
func (r *userRepository) Create(ctx context.Context, user *domain.User) error {
	return r.db.WithContext(ctx).Create(user).Error
}

// FindByID mencari user berdasarkan ID (UUID)
// Return ErrUserNotFound jika tidak ditemukan
func (r *userRepository) FindByID(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	var user domain.User
	err := r.db.WithContext(ctx).Where("id = ?", id).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// FindByUsername mencari user berdasarkan username
// Return ErrUserNotFound jika tidak ditemukan
func (r *userRepository) FindByUsername(ctx context.Context, username string) (*domain.User, error) {
	var user domain.User
	err := r.db.WithContext(ctx).Preload("Roles").Where("username = ?", username).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// FindByEmail mencari user berdasarkan email
// Return ErrUserNotFound jika tidak ditemukan
func (r *userRepository) FindByEmail(ctx context.Context, email string) (*domain.User, error) {
	var user domain.User
	err := r.db.WithContext(ctx).Preload("Roles").Where("email = ?", email).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// FindByUsernameOrEmail mencari user dengan username ATAU email
// Return ErrUserNotFound jika tidak ditemukan
func (r *userRepository) FindByUsernameOrEmail(ctx context.Context, identifier string) (*domain.User, error) {
	var user domain.User
	err := r.db.WithContext(ctx).Preload("Roles").Where("username = ? OR email = ?", identifier, identifier).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// Update mengupdate data user (full update dengan Save)
func (r *userRepository) Update(ctx context.Context, user *domain.User) error {
	return r.db.WithContext(ctx).Save(user).Error
}

// Delete menghapus user beserta semua relasinya
// Menggunakan transaction untuk menjaga data consistency
// user_roles harus dihapus manual karena many2many tidak punya ON DELETE CASCADE
// RefreshTokens dan Activities punya ON DELETE CASCADE di level GORM constraint
func (r *userRepository) Delete(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		// Hapus relasi user_roles dulu (many2many, no CASCADE)
		if err := tx.Where("user_id = ?", id).Delete(&domain.UserRole{}).Error; err != nil {
			return err
		}

		// Hapus user (CASCADE akan auto-delete refresh_tokens dan user_activities)
		return tx.Delete(&domain.User{}, "id = ?", id).Error
	})
}

// ========== Activity Tracking ==========

// UpdateLastLogin mengupdate last_login dan last_activity user
// Dipanggil setiap kali user berhasil login
func (r *userRepository) UpdateLastLogin(ctx context.Context, id uuid.UUID) error {
	now := time.Now()
	return r.db.WithContext(ctx).Model(&domain.User{}).Where("id = ?", id).Updates(map[string]any{
		"last_login":    now,
		"last_activity": now,
	}).Error
}

// UpdateLastActivity mengupdate waktu aktivitas terakhir user
// Dipanggil saat user melakukan action (API call, dll)
func (r *userRepository) UpdateLastActivity(ctx context.Context, id uuid.UUID) error {
	return r.db.WithContext(ctx).Model(&domain.User{}).Where("id = ?", id).Update("last_activity", time.Now()).Error
}

// ========== Role & Permission ==========

// GetUserWithRoles mengambil user beserta roles-nya
// Return ErrUserNotFound jika tidak ditemukan
func (r *userRepository) GetUserWithRoles(ctx context.Context, id uuid.UUID) (*domain.User, error) {
	var user domain.User
	err := r.db.WithContext(ctx).Preload("Roles").Where("id = ?", id).First(&user).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrUserNotFound
	}
	if err != nil {
		return nil, err
	}
	return &user, nil
}

// GetUserPermissions mengambil semua permission names yang dimiliki user
// Query melalui: user_roles -> role_permissions -> permissions
func (r *userRepository) GetUserPermissions(ctx context.Context, id uuid.UUID) ([]string, error) {
	var permissions []string
	err := r.db.WithContext(ctx).Raw(`
		SELECT DISTINCT p.name
		FROM permissions p
		JOIN role_permissions rp ON p.id = rp.permission_id
		JOIN user_roles ur ON rp.role_id = ur.role_id
		WHERE ur.user_id = ?
	`, id).Scan(&permissions).Error

	return permissions, err
}

// HasPermission mengecek apakah user punya permission tertentu
// Return true jika user memiliki permission tersebut via salah satu role-nya
func (r *userRepository) HasPermission(ctx context.Context, userID uuid.UUID, permissionName string) (bool, error) {
	var count int64

	err := r.db.WithContext(ctx).Raw(`
		SELECT COUNT(*)
		FROM permissions p
		JOIN role_permissions rp ON p.id = rp.permission_id
		JOIN user_roles ur ON rp.role_id = ur.role_id
		WHERE ur.user_id = ? AND p.name = ?
	`, userID, permissionName).Scan(&count).Error

	return count > 0, err
}
