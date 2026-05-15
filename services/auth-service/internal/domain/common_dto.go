package domain

// ==================== Common DTOs ====================

// ErrorResponse for error responses
type ErrorResponse struct {
	Detail string `json:"detail"`
}

// MessageResponse for success responses
type MessageResponse struct {
	Detail string `json:"detail"`
}

// HealthResponse for health check
type HealthResponse struct {
	Status   string `json:"status"`
	Service  string `json:"service"`
	Internal bool   `json:"internal,omitempty"`
}
