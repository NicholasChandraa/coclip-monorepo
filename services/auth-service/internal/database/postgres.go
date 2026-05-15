// Package database berisi fungsi untuk koneksi dan setup database PostgreSQL
package database

import (
	"auth-service/internal/config"
	"fmt"
	"time"

	"gorm.io/driver/postgres"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// NewPostgresDB membuat koneksi baru ke PostgreSQL database menggunakan GORM
// Return *gorm.DB yang sudah dikonfigurasi dengan connection pool
//
// Connection pool settings:
//   - MaxIdleConns: 10 (koneksi idle yang di-keep)
//   - MaxOpenConns: 100 (max koneksi yang bisa dibuka)
//   - ConnMaxLifetime: 1 jam (lifetime setiap koneksi)
func NewPostgresDB(cfg *config.DatabaseConfig) (*gorm.DB, error) {
	// Build DSN (Data Source Name) untuk PostgreSQL
	dsn := fmt.Sprintf(
		"host=%s port=%s user=%s password=%s dbname=%s sslmode=%s",
		cfg.Host,
		cfg.Port,
		cfg.User,
		cfg.Password,
		cfg.DBName,
		cfg.SSLMode,
	)

	// Konfigurasi GORM
	gormConfig := &gorm.Config{
		Logger: logger.Default.LogMode(logger.Info), // Log semua SQL queries
	}

	// Buka koneksi ke database
	db, err := gorm.Open(postgres.Open(dsn), gormConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to connect to database: %w", err)
	}

	// Konfigurasi connection pool untuk performa optimal
	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get database instance: %w", err)
	}

	sqlDB.SetMaxIdleConns(10)           // Jumlah koneksi idle di pool
	sqlDB.SetMaxOpenConns(100)          // Max koneksi yang bisa dibuka bersamaan
	sqlDB.SetConnMaxLifetime(time.Hour) // Koneksi di-recycle setelah 1 jam

	return db, nil
}
