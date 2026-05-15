package domain

import "github.com/google/uuid"

// Permission represents the permission entity
type Permission struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Name        string    `gorm:"uniqueIndex;not null;type:varchar(100)" json:"name"`
	Description string    `gorm:"type:text" json:"description"`
	Resource    string    `gorm:"not null;type:varchar(50);index" json:"resource"`
	Action      string    `gorm:"not null;type:varchar(50)" json:"action"`
}

// TableName specifies the table name for Permission
func (Permission) TableName() string {
	return "permissions"
}

// RolePermission junction table for many-to-many relationship between Role and Permission
type RolePermission struct {
	RoleID       uuid.UUID `gorm:"type:uuid;primaryKey"`
	PermissionID uuid.UUID `gorm:"type:uuid;primaryKey"`
}

// TableName specifies the table name for RolePermission
func (RolePermission) TableName() string {
	return "role_permissions"
}
