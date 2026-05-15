package domain

import (
	"time"

	"github.com/google/uuid"
)

// UserActivity represents the user activity entity
type UserActivity struct {
	ID          uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID      uuid.UUID `gorm:"type:uuid;not null;index" json:"user_id"`
	Action      string    `gorm:"not null;type:varchar(100);index" json:"action"`
	Description string    `gorm:"type:text" json:"description"`
	IPAddress   string    `gorm:"type:varchar(45)" json:"ip_address"`
	UserAgent   string    `gorm:"type:text" json:"user_agent"`
	DeviceInfo  string    `gorm:"type:varchar(255)" json:"device_info"`
	CreatedAt   time.Time `gorm:"autoCreateTime;index;idx_created_at,sort:desc" json:"created_at"`
}

// TableName specifies the table name for UserActivity
func (UserActivity) TableName() string {
	return "user_activities"
}
