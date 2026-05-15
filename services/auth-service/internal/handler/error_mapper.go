package handler

import (
	"auth-service/internal/domain"
	"auth-service/pkg/logger"
	"errors"
	"net/http"
)

// HTTPError represents an HTTP error response
type HTTPError struct {
	Status  int
	Code    string
	Message string
}

// MapError maps domain/usecase errors to HTTP error responses
// This centralizes error-to-HTTP mapping logic for consistency
func MapError(err error) HTTPError {
	switch {
	// User errors (404)
	case errors.Is(err, domain.ErrUserNotFound):
		return HTTPError{
			Status:  http.StatusNotFound,
			Code:    "user_not_found",
			Message: "User not found",
		}

	// Username/Email duplicate errors (409 Conflict)
	case errors.Is(err, domain.ErrUsernameAlreadyExists):
		return HTTPError{
			Status:  http.StatusConflict,
			Code:    "username_already_exists",
			Message: "Username already exists",
		}
	case errors.Is(err, domain.ErrEmailAlreadyExists):
		return HTTPError{
			Status:  http.StatusConflict,
			Code:    "email_already_exists",
			Message: "Email already exists",
		}

	// Role errors
	case errors.Is(err, domain.ErrRoleNotFound):
		return HTTPError{
			Status:  http.StatusNotFound,
			Code:    "role_not_found",
			Message: "Role not found",
		}
	case errors.Is(err, domain.ErrRoleAlreadyExists):
		return HTTPError{
			Status:  http.StatusConflict,
			Code:    "role_already_exists",
			Message: "Role already exists",
		}
	case errors.Is(err, domain.ErrRoleInUse):
		return HTTPError{
			Status:  http.StatusConflict,
			Code:    "role_in_use",
			Message: "Cannot delete role that is assigned to users",
		}

	// Permission errors
	case errors.Is(err, domain.ErrPermissionNotFound):
		return HTTPError{
			Status:  http.StatusNotFound,
			Code:    "permission_not_found",
			Message: "Permission not found",
		}
	case errors.Is(err, domain.ErrPermissionAlreadyExists):
		return HTTPError{
			Status:  http.StatusConflict,
			Code:    "permission_already_exists",
			Message: "Permission already exists",
		}

	// Token errors (401 Unauthorized)
	case errors.Is(err, domain.ErrRefreshTokenNotFound):
		return HTTPError{
			Status:  http.StatusUnauthorized,
			Code:    "refresh_token_not_found",
			Message: "Refresh token not found",
		}
	case errors.Is(err, domain.ErrRefreshTokenInvalid):
		return HTTPError{
			Status:  http.StatusUnauthorized,
			Code:    "refresh_token_invalid",
			Message: "Refresh token is invalid",
		}
	case errors.Is(err, domain.ErrRefreshTokenExpired):
		return HTTPError{
			Status:  http.StatusUnauthorized,
			Code:    "refresh_token_expired",
			Message: "Refresh token has expired",
		}

	// Authentication errors (401)
	case errors.Is(err, domain.ErrInvalidCredentials):
		return HTTPError{
			Status:  http.StatusUnauthorized,
			Code:    "invalid_credentials",
			Message: "Invalid username or password",
		}

	// Account status errors
	case errors.Is(err, domain.ErrAccountInactive):
		return HTTPError{
			Status:  http.StatusForbidden,
			Code:    "account_inactive",
			Message: "Account is inactive",
		}

	// Authorization errors (403 Forbidden)
	case errors.Is(err, domain.ErrUnauthorized):
		return HTTPError{
			Status:  http.StatusForbidden,
			Code:    "unauthorized",
			Message: "Unauthorized access",
		}
	case errors.Is(err, domain.ErrForbidden):
		return HTTPError{
			Status:  http.StatusForbidden,
			Code:    "forbidden",
			Message: "Forbidden: insufficient permissions",
		}

	// Validation errors (400 Bad Request)
	case errors.Is(err, domain.ErrInvalidInput):
		return HTTPError{
			Status:  http.StatusBadRequest,
			Code:    "invalid_input",
			Message: "Invalid input data",
		}
	case errors.Is(err, domain.ErrPasswordTooWeak):
		return HTTPError{
			Status:  http.StatusBadRequest,
			Code:    "password_too_weak",
			Message: "Password does not meet requirements",
		}
	case errors.Is(err, domain.ErrInvalidEmail):
		return HTTPError{
			Status:  http.StatusBadRequest,
			Code:    "invalid_email",
			Message: "Invalid email format",
		}
	case errors.Is(err, domain.ErrPasswordMismatch):
		return HTTPError{
			Status:  http.StatusBadRequest,
			Code:    "password_mismatch",
			Message: "Current password is incorrect",
		}
	case errors.Is(err, domain.ErrSamePassword):
		return HTTPError{
			Status:  http.StatusBadRequest,
			Code:    "same_password",
			Message: "New password must be different from current password",
		}
	case errors.Is(err, domain.ErrNoChanges):
		return HTTPError{
			Status:  http.StatusBadRequest,
			Code:    "no_changes",
			Message: "No changes detected",
		}

	// Database errors (500 Internal Server Error)
	case errors.Is(err, domain.ErrDatabaseConnection):
		return HTTPError{
			Status:  http.StatusInternalServerError,
			Code:    "database_connection_failed",
			Message: "Database connection failed",
		}
	case errors.Is(err, domain.ErrDatabaseQuery):
		return HTTPError{
			Status:  http.StatusInternalServerError,
			Code:    "database_query_failed",
			Message: "Database query failed",
		}

	// Self-deactivation prevention (400 Bad Request)
	case err != nil && err.Error() == "cannot deactivate your own account":
		return HTTPError{
			Status:  http.StatusBadRequest,
			Code:    "cannot_deactivate_self",
			Message: "You cannot deactivate your own account",
		}

	// Self-deletion prevention (400 Bad Request)
	case errors.Is(err, domain.ErrCannotDeleteSelf):
		return HTTPError{
			Status:  http.StatusBadRequest,
			Code:    "cannot_delete_self",
			Message: "You cannot delete your own account",
		}

	// Default: unexpected error
	default:

		// Log unexpected errors for debugging
		logger.Error().Err(err).Msg("Unhandled error in error mapper")

		return HTTPError{
			Status:  http.StatusInternalServerError,
			Code:    "internal_error",
			Message: "An unexpected error occurred",
		}
	}
}
