package domain

import "time"

// ======================== Auth DTOs ========================

// UserCreate for registration
type UserCreate struct {
	Email    string `json:"email" binding:"required,email,min=3"`
	Username string `json:"username" binding:"required,min=3,max=50"`
	Password string `json:"password" binding:"required,min=8"`
	FullName string `json:"full_name" binding:"omitempty,min=3"`
}

// LoginRequest for login
type LoginRequest struct {
	Username string `json:"username" form:"username" binding:"required"`
	Password string `json:"password" form:"password" binding:"required"`
}

// TokenResponse for token endpoints
type TokenResponse struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token"`
	TokenType    string `json:"token_type"`
	ExpiresIn    int64  `json:"expires_in"`
}

// UserResponse for user endpoints
type UserResponse struct {
	ID        string     `json:"id"`
	Email     string     `json:"email"`
	Username  string     `json:"username"`
	FullName  string     `json:"full_name"`
	IsActive  bool       `json:"is_active"`
	CreatedAt time.Time  `json:"created_at"`
	UpdatedAt time.Time  `json:"updated_at"`
	LastLogin *time.Time `json:"last_login,omitempty"`
}

// UserWithPermissionsResponse for /me endpoint
type UserWithPermissionsResponse struct {
	UserResponse
	Permissions []string `json:"permissions"`
	Roles       []string `json:"roles"`
}

// UserWithRolesResponse for role endpoints
type UserWithRolesResponse struct {
	UserID    string     `json:"user_id"`
	Username  string     `json:"username"`
	Email     string     `json:"email"`
	FullName  string     `json:"full_name"`
	IsActive  bool       `json:"is_active"`
	Roles     []string   `json:"roles"`
	LastLogin *time.Time `json:"last_login,omitempty"`
}

// UserUpdate for profile update
type UserUpdate struct {
	Username string `json:"username,omitempty" binding:"omitempty,min=3,max=50"`
	FullName string `json:"full_name,omitempty"`
	Email    string `json:"email,omitempty" binding:"omitempty,email"`
}

// ChangePasswordRequest for password change
type ChangePasswordRequest struct {
	CurrentPassword string `json:"current_password" binding:"required"`
	NewPassword     string `json:"new_password" binding:"required,min=8"`
}

// UpdateUserStatusRequest for admin to update user active status
type UpdateUserStatusRequest struct {
	IsActive bool `json:"is_active"`
}
