package repository

import (
	"context"
	"errors"
	"time"

	"auth-service/internal/domain"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// refreshTokenRepository adalah implementasi RefreshTokenRepository interface
// Mengelola refresh tokens untuk token rotation dan session management
type refreshTokenRepository struct {
	db *gorm.DB
}

// NewRefreshTokenRepository membuat instance baru refreshTokenRepository
func NewRefreshTokenRepository(db *gorm.DB) RefreshTokenRepository {
	return &refreshTokenRepository{db: db}
}

// Create menyimpan refresh token baru ke database
// Token disimpan dalam bentuk hash untuk keamanan
func (r *refreshTokenRepository) Create(ctx context.Context, token *domain.RefreshToken) error {
	return r.db.WithContext(ctx).Create(token).Error
}

// FindByHash mencari refresh token berdasarkan hash
// Return ErrRefreshTokenNotFound jika tidak ditemukan atau expired/invalid
func (r *refreshTokenRepository) FindByHash(ctx context.Context, hash string) (*domain.RefreshToken, error) {
	var token domain.RefreshToken
	err := r.db.WithContext(ctx).Where("token_hash = ? AND is_valid = ? AND expires_at > ?", hash, true, time.Now()).First(&token).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, domain.ErrRefreshTokenNotFound
	}
	if err != nil {
		return nil, err
	}
	return &token, nil
}

// Invalidate menandai refresh token sebagai tidak valid (untuk token rotation)
// Dipanggil setelah token dipakai untuk refresh
func (r *refreshTokenRepository) Invalidate(ctx context.Context, hash string) error {
	return r.db.WithContext(ctx).Model(&domain.RefreshToken{}).Where("token_hash = ?", hash).Update("is_valid", false).Error
}

// InvalidateAllForUser menginvalidasi semua refresh token user (untuk logout all devices)
func (r *refreshTokenRepository) InvalidateAllForUser(ctx context.Context, userID uuid.UUID) error {
	return r.db.WithContext(ctx).Model(&domain.RefreshToken{}).Where("user_id = ?", userID).Update("is_valid", false).Error
}

// DeleteExpired menghapus token yang sudah expired atau invalid (untuk cleanup)
// Bisa dijalankan secara periodik via cron job
func (r *refreshTokenRepository) DeleteExpired(ctx context.Context) error {
	return r.db.WithContext(ctx).Where("expires_at < ? OR is_valid = ?", time.Now(), false).Delete(&domain.RefreshToken{}).Error
}
