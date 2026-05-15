package domain

// ====================== Role DTOs ======================

// RoleCreate for role creation
type RoleCreate struct {
	Name        string `json:"name" binding:"required,min=2,max=100"`
	Description string `json:"description"`
}

// RoleUpdate for role update
type RoleUpdate struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	IsActive    *bool  `json:"is_active,omitempty"`
}

// RoleResponse for role endpoints
type RoleResponse struct {
	ID          string               `json:"id"`
	Name        string               `json:"name"`
	Description string               `json:"description"`
	IsActive    bool                 `json:"is_active"`
	Permissions []PermissionResponse `json:"permissions,omitempty"`
}

// PermissionResponse for permission endpoints
type PermissionResponse struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Resource    string `json:"resource"`
	Action      string `json:"action"`
	Description string `json:"description"`
}

// RoleStatistics for analytics
type RoleStatistics struct {
	TotalRoles     int64           `json:"total_roles"`
	ActiveRoles    int64           `json:"active_roles"`
	RolesBreakdown []RoleBreakdown `json:"roles_breakdown"`
}

// RoleBreakdown for role user count
type RoleBreakdown struct {
	RoleName  string `json:"role_name"`
	UserCount int    `json:"user_count"`
}

// AddPermissionsRequest for adding permissions to role
type AddPermissionsRequest struct {
	PermissionNames []string `json:"permission_names" binding:"required,min=1"`
}

// AssignRoleRequest for assigning role to user
type AssignRoleRequest struct {
	UserID   string `json:"user_id" binding:"required"`
	RoleName string `json:"role_name" binding:"required"`
}

// RemoveRoleRequest for removing role from user
type RemoveRoleRequest struct {
	UserID   string `json:"user_id" binding:"required"`
	RoleName string `json:"role_name" binding:"required"`
}

// RemovePermissionsRequest for removing permissions from role
type RemovePermissionsRequest struct {
	PermissionNames []string `json:"permission_names" binding:"required,min=1"`
}
