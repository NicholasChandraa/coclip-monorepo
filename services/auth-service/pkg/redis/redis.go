package redis

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"auth-service/pkg/logger"

	"github.com/redis/go-redis/v9"
)

// Client adalah wrapper untuk Redis client dengan methods yang dibutuhkan
type Client struct {
	rdb *redis.Client
}

// NewClient membuat Redis client baru dengan connection pooling
// Parameters:
//   - host: Redis server host (e.g., "localhost")
//   - port: Redis server port (e.g., "6379")
//   - password: Redis password (kosong jika tidak ada)
//   - db: Redis database number (0-15)
//   - poolSize: Maximum number of socket connections
func NewClient(host string, port string, password string, dbStr string, poolSizeStr string) (*Client, error) {
	// Konversi db string -> int
	dbInt, err := strconv.Atoi(dbStr)
	if err != nil {
		logger.Error().Err(err).Str("db", dbStr).Msg("Failed to parse Redis db parameter")
		return nil, fmt.Errorf("parameter db harus angka %w", err)
	}

	// Konversi pool size string -> int
	poolSizeInt, err := strconv.Atoi(poolSizeStr)
	if err != nil {
		logger.Error().Err(err).Str("pool_size", poolSizeStr).Msg("Failed to parse Redis pool_size parameter")
		return nil, fmt.Errorf("parameter pool size harus angka %w", err)
	}

	// Buat Redis client dengan options
	rdb := redis.NewClient(&redis.Options{
		Addr:         fmt.Sprintf("%s:%s", host, port), // Format: "localhost:6379"
		Password:     password,                         // Password: kosong untuk dev
		DB:           dbInt,                            // Database number
		PoolSize:     poolSizeInt,                      // Connection pool size
		MinIdleConns: 5,                                // Minimum idle connections
		DialTimeout:  5 * time.Second,                  // Timeout untuk initial connection
		ReadTimeout:  3 * time.Second,                  // Timeout untuk read operations
		WriteTimeout: 3 * time.Second,                  // Timeout untuk write operations
	})

	// Tesst connection dengan Ping
	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	if err := rdb.Ping(ctx).Err(); err != nil {
		logger.Error().Err(err).Str("host", host).Str("port", port).Msg("Failed to connect to Redis")
		return nil, fmt.Errorf("failed to connect to Redis: %w", err)
	}

	logger.Info().Str("host", host).Str("port", port).Int("db", dbInt).Msg("Successfully connected to Redis")
	return &Client{rdb: rdb}, nil
}

// Ping checks if Redis server is alive
func (c *Client) Ping(ctx context.Context) error {
	return c.rdb.Ping(ctx).Err()
}

// Close menutup semua connections ke Redis
func (c *Client) Close() error {
	return c.rdb.Close()
}

// Get mengambil value dari Redis berdasarkan key
// Returns empty string jika key tidak ada
func (c *Client) Get(ctx context.Context, key string) (string, error) {
	val, err := c.rdb.Get(ctx, key).Result()
	if err == redis.Nil {
		// Key tidak ditemukan, return empty string
		return "", nil
	}
	if err != nil {
		logger.Error().Err(err).Str("key", key).Msg("Failed to get key from Redis")
	}
	return val, err
}

// Set menyimpan key-value pair ke Redis dengan TTL (Time to Live)
// TTL 0 berarti ga ada expired
func (c *Client) Set(
	ctx context.Context,
	key string,
	value any,
	ttl time.Duration,
) error {
	err := c.rdb.Set(ctx, key, value, ttl).Err()
	if err != nil {
		logger.Error().Err(err).Str("key", key).Dur("ttl", ttl).Msg("Failed to set key in Redis")
	}
	return err
}

// Del menghapus satu atau lebih keys dari Redis
func (c *Client) Del(ctx context.Context, keys ...string) error {
	err := c.rdb.Del(ctx, keys...).Err()
	if err != nil {
		logger.Error().Err(err).Strs("keys", keys).Msg("Failed to delete keys from Redis")
	}
	return err
}

// Exists mengecek apakah key ada di Redis
// Returns true jika key exists, false kalau tidak ada
func (c *Client) Exists(ctx context.Context, key string) (bool, error) {
	count, err := c.rdb.Exists(ctx, key).Result()
	if err != nil {
		logger.Error().Err(err).Str("key", key).Msg("Failed to check key existence in Redis")
	}
	return count > 0, err
}

// Incr increment integer value of key by 1
// Jika key belum ada, akan di set ke 0 dulu baru di increment
// Berguna untuk rate limiting (counter)
func (c *Client) Incr(ctx context.Context, key string) (int64, error) {
	val, err := c.rdb.Incr(ctx, key).Result()
	if err != nil {
		logger.Error().Err(err).Str("key", key).Msg("Failed to increment key in Redis")
	}
	return val, err
}

// Expire set TTL (expiration) untuk key yang sudah ada
// Berguna untuk set expiration setelah Incr
func (c *Client) Expire(ctx context.Context, key string, ttl time.Duration) error {
	err := c.rdb.Expire(ctx, key, ttl).Err()
	if err != nil {
		logger.Error().Err(err).Str("key", key).Dur("ttl", ttl).Msg("Failed to set expiration in Redis")
	}
	return err
}

// GetRawClient returns underlying Redis client
// Use with caution - only for advanced operations
func (c *Client) GetRawClient() *redis.Client {
	return c.rdb
}
