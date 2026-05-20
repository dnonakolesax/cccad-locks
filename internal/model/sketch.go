//nolint:recvcheck // easyjson generates both value and pointer receiver methods for these DTOs.
package model

import "github.com/mailru/easyjson"

//easyjson:json
type CreateSketchRequest struct {
	Name string `json:"name"`
	Unit string `json:"unit,omitempty"`
}

//easyjson:json
type UpdateSketchMetadataRequest struct {
	Name *string `json:"name,omitempty"`
	Unit *string `json:"unit,omitempty"`
}

//easyjson:json
type SketchMetadata struct {
	ID              string `json:"id"`
	WorkspaceID     string `json:"workspaceId"`
	Name            string `json:"name"`
	CreatedByUserID string `json:"createdByUserId"`
	Unit            string `json:"unit"`
	Version         int64  `json:"version"`
	CreatedAt       string `json:"createdAt"`
	UpdatedAt       string `json:"updatedAt"`
}

//easyjson:json
type AvailableSketch struct {
	ID              string `json:"id"`
	WorkspaceID     string `json:"workspaceId"`
	Name            string `json:"name"`
	CreatedByUserID string `json:"createdByUserId"`
	Unit            string `json:"unit"`
	Version         int64  `json:"version"`
	Role            string `json:"role"`
	CreatedAt       string `json:"createdAt"`
	UpdatedAt       string `json:"updatedAt"`
}

//easyjson:json
type AvailableSketchList struct {
	Sketches []AvailableSketch `json:"sketches"`
}

//easyjson:json
type SketchDocument struct {
	ID              string                         `json:"id"`
	WorkspaceID     string                         `json:"workspaceId"`
	Name            string                         `json:"name"`
	CreatedByUserID string                         `json:"createdByUserId"`
	Unit            string                         `json:"unit"`
	Version         int64                          `json:"version"`
	Entities        map[string]easyjson.RawMessage `json:"entities"`
	Constraints     map[string]easyjson.RawMessage `json:"constraints"`
	Dimensions      map[string]easyjson.RawMessage `json:"dimensions"`
	Groups          map[string]easyjson.RawMessage `json:"groups"`
	SolveStatus     easyjson.RawMessage            `json:"solveStatus"`
	Conflicts       []easyjson.RawMessage          `json:"conflicts,omitempty"`
}
