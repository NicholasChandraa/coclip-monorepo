package domain

import "github.com/google/uuid"

// Role represents the role entity
type Role struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Name        string    `gorm:"uniqueIndex;not null;type:varchar(100)" json:"name"`
	Description string    `gorm:"type:text" json:"description"`
	IsActive    bool      `gorm:"default:true" json:"is_active"`

	// Relationships
	Permissions []Permission `gorm:"many2many:role_permissions" json:"permissions,omitempty"`
}

// TableName specifies the table name for Role
func (Role) TableName() string {
	return "roles"
}

// UserRole junction table for many-to-many relationship between User and Role
type UserRole struct {
	UserID uuid.UUID `gorm:"type:uuid;primaryKey"`
	RoleID uuid.UUID `gorm:"type:uuid;primaryKey"`
}

// TableName specifies the table name for UserRole
func (UserRole) TableName() string {
	return "user_roles"
}
