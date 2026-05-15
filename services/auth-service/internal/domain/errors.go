package domain

import "errors"

// Repository layer errors
// These errors represent data access failures and missing records
var (
	// User errors
	ErrUserNotFound          = errors.New("user not found")
	ErrUsernameAlreadyExists = errors.New("username already exists")
	ErrEmailAlreadyExists    = errors.New("email already exists")

	// Role errors
	ErrRoleNotFound      = errors.New("role not found")
	ErrRoleAlreadyExists = errors.New("role already exists")
	ErrRoleInUse         = errors.New("role is in use by users")

	// Permission errors
	ErrPermissionNotFound      = errors.New("permission not found")
	ErrPermissionAlreadyExists = errors.New("permission already exists")

	// Token errors
	ErrRefreshTokenNotFound = errors.New("refresh token not found")
	ErrRefreshTokenInvalid  = errors.New("refresh token is invalid")
	ErrRefreshTokenExpired  = errors.New("refresh token has expired")

	// Database errors
	ErrDatabaseConnection = errors.New("database connection failed")
	ErrDatabaseQuery      = errors.New("database query failed")
)

// Usecase layer errors
// These errors represent business logic violations
var (
	// Authentication errors
	ErrInvalidCredentials = errors.New("invalid username or password")
	ErrAccountInactive    = errors.New("account is inactive")
	ErrUnauthorized       = errors.New("unauthorized access")
	ErrForbidden          = errors.New("forbidden: insufficient permissions")

	// Validation errors
	ErrInvalidInput    = errors.New("invalid input data")
	ErrPasswordTooWeak = errors.New("password does not meet requirements")
	ErrInvalidEmail    = errors.New("invalid email format")

	// Business logic errors
	ErrPasswordMismatch  = errors.New("current password is incorrect")
	ErrNoChanges         = errors.New("no changes detected")
	ErrSamePassword      = errors.New("new password must be different from current password")
	ErrCannotDeleteSelf  = errors.New("cannot delete your own account")
)
