package repository

import (
	"context"
	"time"

	"auth-service/internal/domain"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

// userActivityRepository adalah implementasi UserActivityRepository interface
// Mengelola audit log aktivitas user (login, logout, password change, dll)
type userActivityRepository struct {
	db *gorm.DB
}

// NewUserActivityRepository membuat instance baru userActivityRepository
func NewUserActivityRepository(db *gorm.DB) UserActivityRepository {
	return &userActivityRepository{db: db}
}

// Create menyimpan activity baru ke database
// Dipanggil setiap kali ada action yang perlu di-log (login, logout, dll)
func (r *userActivityRepository) Create(ctx context.Context, activity *domain.UserActivity) error {
	return r.db.WithContext(ctx).Create(activity).Error
}

// FindByUserID mengambil activities user dengan filter dan pagination
// Query bisa difilter by action, date range, dan pagination
func (r *userActivityRepository) FindByUserID(ctx context.Context, userID uuid.UUID, query *domain.ActivityQuery) ([]domain.UserActivity, error) {
	var activities []domain.UserActivity

	db := r.db.WithContext(ctx).Where("user_id = ?", userID)

	// Apply filters jika ada
	if query != nil {
		if query.Action != "" {
			db = db.Where("action = ?", query.Action)
		}
		if query.StartDate != nil {
			db = db.Where("created_at >= ?", query.StartDate)
		}
		if query.EndDate != nil {
			db = db.Where("created_at <= ?", query.EndDate)
		}

		// Pagination
		if query.Limit > 0 {
			db = db.Limit(query.Limit)
		}
		if query.Offset > 0 {
			db = db.Offset(query.Offset)
		}
	}

	err := db.Order("created_at DESC").Find(&activities).Error
	return activities, err
}

// GetRecentByUserID mengambil N aktivitas terbaru user
// Shortcut untuk FindByUserID dengan sorting terbaru
func (r *userActivityRepository) GetRecentByUserID(ctx context.Context, userID uuid.UUID, limit int) ([]domain.UserActivity, error) {
	var activities []domain.UserActivity

	err := r.db.WithContext(ctx).
		Where("user_id = ?", userID).
		Order("created_at DESC").
		Limit(limit).
		Find(&activities).Error

	return activities, err
}

// GetSummary menghasilkan ringkasan statistik aktivitas user
// Data yang dikumpulkan: total aktivitas, breakdown per action, hari teraktif,
// rata-rata per hari, jumlah IP unik, dan jumlah device unik
func (r *userActivityRepository) GetSummary(ctx context.Context, userID uuid.UUID, days int) (*domain.ActivitySummary, error) {
	summary := &domain.ActivitySummary{
		ActionBreakdown:    make(map[string]int),
		AnalysisPeriodDays: days,
	}

	startDate := time.Now().AddDate(0, 0, -days)

	// Hitung total aktivitas dalam periode
	var totalCount int64
	r.db.WithContext(ctx).
		Model(&domain.UserActivity{}).
		Where("user_id = ? AND created_at >= ?", userID, startDate).
		Count(&totalCount)

	summary.TotalActivities = int(totalCount)

	// Hitung breakdown per jenis action (login, logout, dll)
	var actionResults []struct {
		Action string
		Count  int64
	}
	r.db.WithContext(ctx).
		Model(&domain.UserActivity{}).
		Select("action, count(*) as count").
		Where("user_id = ? AND created_at >= ?", userID, startDate).
		Group("action").
		Scan(&actionResults)

	for _, result := range actionResults {
		summary.ActionBreakdown[result.Action] = int(result.Count)
	}

	// Cari hari dengan aktivitas terbanyak
	var mostActiveDay struct {
		Day   string
		Count int64
	}
	r.db.WithContext(ctx).
		Model(&domain.UserActivity{}).
		Select("DATE(created_at) as day, count(*) as count").
		Where("user_id = ? AND created_at >= ?", userID, startDate).
		Group("day").
		Order("count DESC").
		Limit(1).
		Scan(&mostActiveDay)

	summary.MostActiveDay = mostActiveDay.Day

	// Hitung rata-rata aktivitas per hari
	if days > 0 {
		summary.AveragePerDay = float64(totalCount) / float64(days)
	}

	// Hitung jumlah IP address unik (untuk deteksi multi-location access)
	var uniqueIPs int64
	r.db.WithContext(ctx).
		Model(&domain.UserActivity{}).
		Where("user_id = ? AND created_at >= ?", userID, startDate).
		Distinct("ip_address").
		Count(&uniqueIPs)

	summary.UniqueIPAddresses = int(uniqueIPs)

	// Hitung jumlah device unik
	var uniqueDevices int64
	r.db.WithContext(ctx).
		Model(&domain.UserActivity{}).
		Where("user_id = ? AND created_at >= ? AND device_info != ''", userID, startDate).
		Distinct("device_info").
		Count(&uniqueDevices)

	summary.UniqueDevices = int(uniqueDevices)

	return summary, nil
}

// DeleteOlderThan menghapus aktivitas yang lebih lama dari waktu tertentu
// Digunakan untuk cleanup data lama (bisa dijalankan via cron job)
func (r *userActivityRepository) DeleteOlderThan(ctx context.Context, before time.Time) error {
	return r.db.WithContext(ctx).
		Where("created_at < ?", before).
		Delete(&domain.UserActivity{}).Error
}
