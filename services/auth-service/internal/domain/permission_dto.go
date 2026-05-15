package domain

// PermissionCreate for permission creation
type PermissionCreate struct {
	Name        string `json:"name" binding:"required,min=3,max=100"` // Format: resource:action
	Description string `json:"description"`
	Resource    string `json:"resource" binding:"required,max=50"`
	Action      string `json:"action" binding:"required,max=50"`
}

// PermissionUpdate for permission update
type PermissionUpdate struct {
	Name        string `json:"name,omitempty"`
	Description string `json:"description,omitempty"`
	Resource    string `json:"resource,omitempty"`
	Action      string `json:"action,omitempty"`
}
