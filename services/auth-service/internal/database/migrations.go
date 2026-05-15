package database

import (
	"auth-service/internal/domain"
	"auth-service/pkg/logger"

	"golang.org/x/crypto/bcrypt"
	"gorm.io/gorm"
)

// AutoMigrate menjalankan database migrations menggunakan GORM AutoMigrate
// GORM akan otomatis create/update tables berdasarkan struct definitions di domain
//
// Tables yang dibuat:
//   - users: data user (username, email, password hash, dll)
//   - roles: daftar role (admin, moderator, user)
//   - permissions: daftar permission (user.read, user.write, dll)
//   - refresh_tokens: refresh token untuk token rotation
//   - user_activities: audit log aktivitas user
//   - social_accounts: OAuth provider accounts linked to users (Google, GitHub, etc.)
//   - user_roles: pivot table user <-> roles (many-to-many)
//   - role_permissions: pivot table roles <-> permissions (many-to-many)
func AutoMigrate(db *gorm.DB) error {
	logger.Info().Msg("Running database migrations...")

	// Migrate semua entities (GORM akan create table jika belum ada)
	if err := db.AutoMigrate(
		&domain.User{},
		&domain.Role{},
		&domain.Permission{},
		&domain.RefreshToken{},
		&domain.UserActivity{},
		&domain.SocialAccount{},
	); err != nil {
		return err
	}

	logger.Info().Msg("Database migrations completed")
	return nil
}

// SeedData mengisi data awal (roles, permissions) ke database
// Fungsi ini idempotent - hanya insert jika data belum ada
//
// Data yang di-seed:
//   - Roles: admin, moderator, user
//   - Permissions: user.read, user.write, user.delete, role.read, role.write, role.delete, activity.read
//   - Role-Permission mapping:
//   - admin: semua permissions
//   - user: user.read, activity.read
func SeedData(db *gorm.DB) error {
	logger.Info().Msg("Seeding initial data...")

	// ========== Seed Roles ==========
	roles := []domain.Role{
		{Name: "admin", Description: "Administrator with full access", IsActive: true},
		{Name: "moderator", Description: "Moderator with limited admin access", IsActive: true},
		{Name: "user", Description: "Standard user role", IsActive: true},
	}

	for _, role := range roles {
		var existing domain.Role
		result := db.Where("name = ?", role.Name).First(&existing)
		if result.Error == gorm.ErrRecordNotFound {
			// Role belum ada, create baru
			if err := db.Create(&role).Error; err != nil {
				logger.Error().Err(err).Str("role", role.Name).Msg("Failed to create role")
			} else {
				logger.Debug().Str("role", role.Name).Msg("Created role")
			}
		}
	}

	// ========== Seed Permissions ==========
	// Format nama: resource.action
	permissions := []domain.Permission{
		{Name: "user.read", Resource: "user", Action: "read", Description: "Read user data"},
		{Name: "user.write", Resource: "user", Action: "write", Description: "Create/Update user data"},
		{Name: "user.delete", Resource: "user", Action: "delete", Description: "Delete user"},
		{Name: "role.read", Resource: "role", Action: "read", Description: "View roles"},
		{Name: "role.write", Resource: "role", Action: "write", Description: "Create/Update roles"},
		{Name: "role.delete", Resource: "role", Action: "delete", Description: "Delete roles"},
		{Name: "activity.read", Resource: "activity", Action: "read", Description: "View activity logs"},
	}

	for _, perm := range permissions {
		var existing domain.Permission
		result := db.Where("name = ?", perm.Name).First(&existing)
		if result.Error == gorm.ErrRecordNotFound {
			// Permission belum ada, create baru
			if err := db.Create(&perm).Error; err != nil {
				logger.Error().Err(err).Str("permission", perm.Name).Msg("Failed to create permission")
			} else {
				logger.Debug().Str("permission", perm.Name).Msg("Created permission")
			}
		}
	}

	// ========== Assign Permissions ke Roles ==========

	// Admin role: assign SEMUA permissions
	var adminRole domain.Role
	if err := db.Where("name = ?", "admin").First(&adminRole).Error; err == nil {
		var allPermissions []domain.Permission
		db.Find(&allPermissions)

		for _, perm := range allPermissions {
			var existing domain.RolePermission
			result := db.Where("role_id = ? AND permission_id = ?", adminRole.ID, perm.ID).First(&existing)
			if result.Error == gorm.ErrRecordNotFound {
				db.Create(&domain.RolePermission{
					RoleID:       adminRole.ID,
					PermissionID: perm.ID,
				})
			}
		}
		logger.Debug().Msg("Assigned all permissions to admin role")
	}

	// User role: assign permission dasar saja
	var userRole domain.Role
	if err := db.Where("name = ?", "user").First(&userRole).Error; err == nil {
		basicPermissions := []string{"user.read", "activity.read"}
		for _, permName := range basicPermissions {
			var perm domain.Permission
			if db.Where("name = ?", permName).First(&perm).Error == nil {
				var existing domain.RolePermission
				result := db.Where("role_id = ? AND permission_id = ?", userRole.ID, perm.ID).First(&existing)
				if result.Error == gorm.ErrRecordNotFound {
					db.Create(&domain.RolePermission{
						RoleID:       userRole.ID,
						PermissionID: perm.ID,
					})
				}
			}
		}
		logger.Debug().Msg("Assigned basic permissions to user role")
	}

	// ========== Seed Default Admin User ==========
	// Create admin user jika belum ada (untuk bootstrap aplikasi)
	var adminUser domain.User
	result := db.Where("username = ?", "admin").First(&adminUser)
	if result.Error == gorm.ErrRecordNotFound {
		// Hash password untuk admin (default: "admin123")
		// IMPORTANT: Ganti password ini setelah first login!
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte("admin123"), bcrypt.DefaultCost)
		if err != nil {
			logger.Error().Err(err).Msg("Failed to hash admin password")
			return nil
		}

		adminUser = domain.User{
			Username:       "admin",
			Email:          "admin@example.com",
			HashedPassword: string(hashedPassword),
			FullName:       "System Administrator",
			IsActive:       true,
		}

		if err := db.Create(&adminUser).Error; err != nil {
			logger.Error().Err(err).Msg("Failed to create admin user")
		} else {
			logger.Info().Msg("Created default admin user (username: admin, password: admin123)")

			// Assign role admin ke admin user
			var adminRole domain.Role
			if db.Where("name = ?", "admin").First(&adminRole).Error == nil {
				db.Create(&domain.UserRole{
					UserID: adminUser.ID,
					RoleID: adminRole.ID,
				})
				logger.Info().Msg("Assigned admin role to default admin user")
			}
		}
	}

	logger.Info().Msg("Seeding completed")
	return nil
}
