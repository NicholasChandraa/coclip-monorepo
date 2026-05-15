package repository

import (
	"context"
	"time"

	"auth-service/internal/domain"

	"github.com/google/uuid"
	"gorm.io/gorm"
	"gorm.io/gorm/clause"
)

// SocialAccountRepository handles persistence of OAuth social accounts.
type SocialAccountRepository interface {
	Upsert(ctx context.Context, account *domain.SocialAccount) error
	FindByUserAndPlatform(ctx context.Context, userID uuid.UUID, platform string) (*domain.SocialAccount, error)
	FindAllByUser(ctx context.Context, userID uuid.UUID) ([]domain.SocialAccount, error)
	DeleteByUserAndPlatform(ctx context.Context, userID uuid.UUID, platform string) error
	UpdateTokens(ctx context.Context, id uuid.UUID, encAccess, encRefresh string, expiry time.Time) error
}

type socialAccountRepository struct {
	db *gorm.DB
}

// NewSocialAccountRepository creates a new GORM-backed SocialAccountRepository.
func NewSocialAccountRepository(db *gorm.DB) SocialAccountRepository {
	return &socialAccountRepository{db: db}
}

// Upsert inserts a new SocialAccount or updates existing columns on (user_id, platform) conflict.
func (r *socialAccountRepository) Upsert(ctx context.Context, account *domain.SocialAccount) error {
	return r.db.WithContext(ctx).
		Clauses(clause.OnConflict{
			Columns: []clause.Column{{Name: "user_id"}, {Name: "platform"}},
			DoUpdates: clause.AssignmentColumns([]string{
				"access_token", "refresh_token", "token_expiry",
				"scope", "platform_user_id", "platform_username", "updated_at",
			}),
		}).
		Create(account).Error
}

// FindByUserAndPlatform retrieves a single SocialAccount by (user_id, platform).
// Returns gorm.ErrRecordNotFound wrapped in the error chain if no row exists.
func (r *socialAccountRepository) FindByUserAndPlatform(ctx context.Context, userID uuid.UUID, platform string) (*domain.SocialAccount, error) {
	var account domain.SocialAccount
	err := r.db.WithContext(ctx).
		Where("user_id = ? AND platform = ?", userID, platform).
		First(&account).Error
	if err != nil {
		return nil, err
	}
	return &account, nil
}

// FindAllByUser returns all connected social accounts for a given user.
func (r *socialAccountRepository) FindAllByUser(ctx context.Context, userID uuid.UUID) ([]domain.SocialAccount, error) {
	var accounts []domain.SocialAccount
	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Find(&accounts).Error
	return accounts, err
}

// DeleteByUserAndPlatform removes the social account row matching (user_id, platform).
func (r *socialAccountRepository) DeleteByUserAndPlatform(ctx context.Context, userID uuid.UUID, platform string) error {
	return r.db.WithContext(ctx).
		Where("user_id = ? AND platform = ?", userID, platform).
		Delete(&domain.SocialAccount{}).Error
}

// UpdateTokens replaces the stored encrypted tokens and expiry for the given account ID.
func (r *socialAccountRepository) UpdateTokens(ctx context.Context, id uuid.UUID, encAccess, encRefresh string, expiry time.Time) error {
	return r.db.WithContext(ctx).
		Model(&domain.SocialAccount{}).
		Where("id = ?", id).
		Updates(map[string]any{
			"access_token":  encAccess,
			"refresh_token": encRefresh,
			"token_expiry":  expiry,
		}).Error
}
