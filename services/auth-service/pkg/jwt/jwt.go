// Package jwt menyediakan utilities untuk JWT (JSON Web Token) operations
// Menggunakan library golang-jwt/jwt untuk generate dan validate token
package jwt

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// Error variables untuk JWT operations
var (
	ErrInvalidToken = errors.New("invalid token")     // Token invalid/format salah
	ErrExpiredToken = errors.New("token has expired") // Token sudah expired
)

// Claims adalah struct untuk JWT payload
// Berisi informasi user yang di-embed di dalam token
type Claims struct {
	UserID               string   `json:"user_id"`  // ID user
	Username             string   `json:"username"` // Username untuk display
	Email                string   `json:"email"`    // Email user
	Roles                []string `json:"roles"`    // User roles (admin, moderator, user)
	jwt.RegisteredClaims          // Standard JWT claims (exp, iat, sub)
}

// TokenService handles generate & validate JWT tokens
// Service ini stateless, hanya menyimpan config (secret key & expiry)
type TokenService struct {
	secretKey          []byte        // Secret key untuk sign JWT (harus aman!)
	accessTokenExpiry  time.Duration // Expiry access token (misal: 15 menit)
	refreshTokenExpiry time.Duration // Expiry refresh token (misal: 7 hari)
}

// NewTokenService membuat instance baru TokenService
// Secret key harus disimpan aman di environment variable!
func NewTokenService(secretKey string, accessExpiry, refreshExpiry time.Duration) *TokenService {
	return &TokenService{
		secretKey:          []byte(secretKey),
		accessTokenExpiry:  accessExpiry,
		refreshTokenExpiry: refreshExpiry,
	}
}

// GenerateAccessToken membuat JWT access token baru
// Access token berisi user info (ID, username, email) dan valid untuk waktu singkat
// Return: token string, expiry time, dan error
func (s *TokenService) GenerateAccessToken(userID, username, email string, roles []string) (string, time.Time, error) {
	expiresAt := time.Now().Add(s.accessTokenExpiry)

	// Build claims dengan user info & expiry time
	claims := &Claims{
		UserID:   userID,
		Username: username,
		Email:    email,
		Roles:    roles,
		RegisteredClaims: jwt.RegisteredClaims{
			ExpiresAt: jwt.NewNumericDate(expiresAt),
			IssuedAt:  jwt.NewNumericDate(time.Now()),
			Subject:   userID,
			ID:        uuid.NewString(), // Add Entropy/Unique ID
		},
	}

	// Create token dengan algoritma HS256 (HMAC SHA256)
	token := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)

	// Sign token dengan secret key
	tokenString, err := token.SignedString(s.secretKey)

	return tokenString, expiresAt, err
}

// GenerateRefreshToken membuat refresh token dan hash-nya
// Refresh token hanya berisi user ID (tidak ada username/email)
// Token di-hash sebelum disimpan di database untuk keamanan
// Return: token (plain), hash (untuk database), expiry time, error
func (s *TokenService) GenerateRefreshToken(userID string) (token string, hash string, expiresAt time.Time, err error) {
	expiresAt = time.Now().Add(s.refreshTokenExpiry)

	// Refresh token hanya perlu subject (user ID) & expiry
	// Tambahkan ID (jti) random UUID supaya token selalu unique (even di detik yang sama)
	claims := &jwt.RegisteredClaims{
		ExpiresAt: jwt.NewNumericDate(expiresAt),
		IssuedAt:  jwt.NewNumericDate(time.Now()),
		Subject:   userID,
		ID:        uuid.NewString(), // Add Entropy
	}

	jwtToken := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	token, err = jwtToken.SignedString(s.secretKey)
	if err != nil {
		return "", "", time.Time{}, err
	}

	// Hash token dengan SHA256 untuk disimpan di database
	hash = HashToken(token)
	return token, hash, expiresAt, nil
}

// ValidateAccessToken memvalidasi access token dan return claims
// Cek signature, expiry, dan format token
func (s *TokenService) ValidateAccessToken(tokenString string) (*Claims, error) {
	token, err := jwt.ParseWithClaims(tokenString, &Claims{}, func(token *jwt.Token) (any, error) {
		// Validasi signing method harus HMAC
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return s.secretKey, nil
	})

	// Handle error parsing
	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return nil, ErrExpiredToken
		}
		return nil, ErrInvalidToken
	}

	// Extract claims dari token yang valid
	if claims, ok := token.Claims.(*Claims); ok && token.Valid {
		return claims, nil
	}

	return nil, ErrInvalidToken
}

// ValidateRefreshToken memvalidasi refresh token dan return user ID
// Hanya return user ID (subject claim) kalau token valid
func (s *TokenService) ValidateRefreshToken(tokenString string) (string, error) {
	token, err := jwt.Parse(tokenString, func(token *jwt.Token) (any, error) {
		if _, ok := token.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, ErrInvalidToken
		}
		return s.secretKey, nil
	})

	if err != nil {
		if errors.Is(err, jwt.ErrTokenExpired) {
			return "", ErrExpiredToken
		}
		return "", ErrInvalidToken
	}

	// Extract user ID dari subject claim
	if claims, ok := token.Claims.(jwt.MapClaims); ok && token.Valid {
		if sub, ok := claims["sub"].(string); ok {
			return sub, nil
		}
	}

	return "", ErrInvalidToken
}

// HashToken membuat SHA256 hash dari token
// Digunakan untuk menyimpan refresh token di database secara aman
// Kalau database breach, token asli tetap aman karena di-hash
func HashToken(token string) string {
	hash := sha256.Sum256([]byte(token))
	return hex.EncodeToString(hash[:])
}

// GetAccessTokenExpiry return durasi expiry access token
func (s *TokenService) GetAccessTokenExpiry() time.Duration {
	return s.accessTokenExpiry
}

// GetRefreshTokenExpiry return durasi expiry refresh token
func (s *TokenService) GetRefreshTokenExpiry() time.Duration {
	return s.refreshTokenExpiry
}
