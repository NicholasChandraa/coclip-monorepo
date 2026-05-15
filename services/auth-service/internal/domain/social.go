package domain

import (
	"time"

	"github.com/google/uuid"
)

// SocialAccount stores encrypted OAuth tokens for connected social platforms.
// Unique constraint: one row per user per platform.
type SocialAccount struct {
	ID               uuid.UUID `gorm:"type:uuid;primaryKey;default:gen_random_uuid()" json:"id"`
	UserID           uuid.UUID `gorm:"type:uuid;not null;uniqueIndex:idx_social_user_platform" json:"user_id"`
	Platform         string    `gorm:"type:varchar(50);not null;uniqueIndex:idx_social_user_platform" json:"platform"`
	AccessToken      string    `gorm:"type:text;not null" json:"-"`
	RefreshToken     string    `gorm:"type:text;not null" json:"-"`
	TokenExpiry      time.Time `gorm:"not null" json:"token_expiry"`
	Scope            string    `gorm:"type:text" json:"scope"`
	PlatformUserID   string    `gorm:"type:varchar(255)" json:"platform_user_id"`
	PlatformUsername string    `gorm:"type:varchar(255)" json:"platform_username"`
	CreatedAt        time.Time `gorm:"autoCreateTime" json:"created_at"`
	UpdatedAt        time.Time `gorm:"autoUpdateTime" json:"updated_at"`
}

// TableName returns the explicit table name for GORM.
func (SocialAccount) TableName() string { return "social_accounts" }

// OAuthStartResponse is returned when initiating an OAuth flow.
type OAuthStartResponse struct {
	URL string `json:"url"`
}

// SocialAccountResponse is the public-safe view of a connected account (no token fields).
type SocialAccountResponse struct {
	ID               string    `json:"id"`
	Platform         string    `json:"platform"`
	PlatformUserID   string    `json:"platform_user_id"`
	PlatformUsername string    `json:"platform_username"`
	ConnectedAt      time.Time `json:"connected_at"`
}

// InternalTokenResponse is returned by the internal token endpoint consumed by the engine.
type InternalTokenResponse struct {
	AccessToken      string    `json:"access_token"`
	TokenExpiry      time.Time `json:"token_expiry"`
	OpenID           string    `json:"open_id"`
	PlatformUsername string    `json:"platform_username"`
}
