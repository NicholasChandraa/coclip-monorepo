package redis

import (
	"auth-service/pkg/logger"
	"context"
	"encoding/json"
	"fmt"
	"time"
)

// UserSession adalah struct untuk session cache
// Struct ini lebih kecil/ringan dari full domain.User (tidak perlu semua field)
// Includes roles AND permissions untuk complete RBAC support
type UserSession struct {
	ID          string   `json:"id"`
	Username    string   `json:"username"`
	Email       string   `json:"email"`
	FullName    string   `json:"full_name"`
	IsActive    bool     `json:"is_active"`
	Roles       []string `json:"roles"`       // Role names (e.g., ["admin", "user"])
	Permissions []string `json:"permissions"` // Permission names (e.g., ["user.read", "user.write"])
	CachedAt    int64    `json:"cached_at"`   // Unix timestamp untuk tracking
}

// SessionTTL adalah durasi cache session
const SessionTTL = 15 * time.Minute // (15 menit)

// GetUserSession mengambil user session dari Redis cache
// Return (session, true) jiika cache hit
// Return (nil, false) jika caches miss (key tidaka ada)
func (c *Client) GetUserSession(ctx context.Context, userID string) (*UserSession, bool) {
	// Build cache key
	key := fmt.Sprintf("user:session:%s", userID)

	// Get value dari Redis
	val, err := c.Get(ctx, key)
	if err != nil || val == "" {
		// Cache miss atau error
		return nil, false
	}

	// Unmarshal JSON ke UserSession struct
	var session UserSession
	if err := json.Unmarshal([]byte(val), &session); err != nil {
		// Invalid JSON - delete corrups cache
		logger.Warn().Err(err).Str("user_id", userID).Msg("Corrupt session cache, deleting")
		_ = c.Del(ctx, key)
		return nil, false
	}

	return &session, true
}

// SetUserSession menyimpan user session ke Redis cache
func (c *Client) SetUserSession(ctx context.Context, session *UserSession) error {
	// Build cache key
	key := fmt.Sprintf("user:session:%s", session.ID)

	// Set cached_at timestamp
	session.CachedAt = time.Now().Unix() // mencatat waktu sekarang dalam integer

	// Marshal session ke JSON
	val, err := json.Marshal(session)
	if err != nil {
		logger.Error().Err(err).Str("user_id", session.ID).Msg("Failed to marshal session")
		return fmt.Errorf("failed to marshal session: %w", err)
	}

	// Save to Redis dengan TTL 15 menit
	return c.Set(ctx, key, val, SessionTTL)
}

// DeleteUserSession menghapus user session dari cache
// Dipakai saat logout, profile update, role change, dll
func (c *Client) DeleteUserSession(ctx context.Context, userID string) error {
	key := fmt.Sprintf("user:session:%s", userID)
	return c.Del(ctx, key)
}

// BlacklistToken menambahkan refresh token ke blacklist
// Dipakai saat logout
// TTL otomatis sesuai dengan token expiry (token expired = auto-cleanup)
func (c *Client) BlacklistToken(ctx context.Context, tokenHash string, ttl time.Duration) error {
	// Build cache key dengan prefix "blacklist:"
	key := fmt.Sprintf("blacklist:token:%s", tokenHash)

	// Value ga penting (cuma buutuh key exists atau tidak)
	// Set "1" sebagai placeholder value
	return c.Set(ctx, key, "1", ttl)
}

// IsTokenBlacklisted mengecek apakah token sudah di-blacklist
// Return true jika token di blacklist (tidak boleh dipakai)
// Return false jika token NOT blacklist (token valid, boleh dikapai)
func (c *Client) IsTokenBlacklisted(ctx context.Context, tokenHash string) bool {
	key := fmt.Sprintf("blacklist:token:%s", tokenHash)

	// Check apakah key exists di Redis
	val, err := c.Get(ctx, key)
	if err != nil || val == "" {
		// Key tidak ada = token NOT blacklisted
		return false
	}

	// Key exists = token di blacklistede
	return true
}
