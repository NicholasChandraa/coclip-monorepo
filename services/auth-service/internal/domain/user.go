package domain

import (
	"time"

	"github.com/google/uuid"
)

// User represents the user entity
type User struct {
	ID             uuid.UUID  `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	Username       string     `gorm:"uniqueIndex;not null;type:varchar(255)" json:"username"`
	Email          string     `gorm:"uniqueIndex;not null;type:varchar(255)" json:"email"`
	HashedPassword string     `gorm:"not null;type:varchar(255)" json:"-"`
	FullName       string     `gorm:"type:varchar(255)" json:"full_name"`
	IsActive       bool       `gorm:"default:true" json:"is_active"`
	CreatedAt      time.Time  `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt      time.Time  `gorm:"autoUpdateTime" json:"updated_at"`
	LastLogin      *time.Time `json:"last_login,omitempty"`
	LastActivity   *time.Time `json:"last_activity,omitempty"`

	// Relationships
	Roles         []Role         `gorm:"many2many:user_roles" json:"roles,omitempty"`
	RefreshTokens []RefreshToken `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"-"`
	Activities    []UserActivity `gorm:"foreignKey:UserID;constraint:OnDelete:CASCADE" json:"-"`
}

// TableName specifies the table name for User
func (User) TableName() string {
	return "users"
}
