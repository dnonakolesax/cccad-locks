//nolint:recvcheck // easyjson generates both value and pointer receiver methods for these DTOs.
package model

//easyjson:json
type Permission struct {
	SketchID        string  `json:"sketchId"`
	UserID          string  `json:"userId"`
	Role            string  `json:"role"`
	Source          string  `json:"source"`
	GrantedByUserID *string `json:"grantedByUserId"`
	CreatedAt       string  `json:"createdAt"`
	UpdatedAt       string  `json:"updatedAt"`
}

//easyjson:json
type PermissionList struct {
	SketchID    string       `json:"sketchId"`
	Permissions []Permission `json:"permissions"`
}

//easyjson:json
type SetPermissionRequest struct {
	Role string `json:"role"`
}

//easyjson:json
type ErrorEnvelope struct {
	Error ErrorObject `json:"error"`
}

//easyjson:json
type ErrorObject struct {
	Code    string                 `json:"code"`
	Message string                 `json:"message"`
	Details map[string]interface{} `json:"details,omitempty"`
}
