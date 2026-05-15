package domain

import (
	"time"

	"github.com/google/uuid"
)

// RefreshToken represents the refresh token entity
type RefreshToken struct {
	ID         uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	TokenHash  string    `gorm:"uniqueIndex;not null;type:varchar(255)" json:"-"`
	UserID     uuid.UUID `gorm:"type:uuid;not null;index" json:"user_id"`
	CreatedAt  time.Time `gorm:"autoCreateTime" json:"created_at"`
	ExpiresAt  time.Time `gorm:"not null" json:"expires_at"`
	IsValid    bool      `gorm:"default:true" json:"is_valid"`
	DeviceInfo string    `gorm:"type:varchar(255)" json:"device_info"`
	IPAddress  string    `gorm:"type:varchar(45)" json:"ip_address"`
}

// TableName specifies the table name for RefreshToken
func (RefreshToken) TableName() string {
	return "refresh_tokens"
}
