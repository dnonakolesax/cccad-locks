//nolint:recvcheck // easyjson generates both value and pointer receiver methods for these DTOs.
package model

//easyjson:json
type CreateWorkspaceRequest struct {
	Name        string `json:"name"`
	Description string `json:"description,omitempty"`
}

//easyjson:json
type UpdateWorkspaceRequest struct {
	Name        *string `json:"name,omitempty"`
	Description *string `json:"description,omitempty"`
}

//easyjson:json
type Workspace struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Description     string `json:"description,omitempty"`
	CreatedByUserID string `json:"createdByUserId"`
	CreatedAt       string `json:"createdAt"`
	UpdatedAt       string `json:"updatedAt"`
}

//easyjson:json
type WorkspaceList struct {
	Workspaces []Workspace `json:"workspaces"`
}
