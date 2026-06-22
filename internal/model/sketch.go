//nolint:recvcheck // easyjson generates both value and pointer receiver methods for these DTOs.
package model

import "github.com/mailru/easyjson"

//go:generate easyjson -all sketch.go

//easyjson:json
type CreateSketchRequest struct {
	Name  string       `json:"name"`
	Unit  string       `json:"unit,omitempty"`
	Plane *SketchPlane `json:"plane"`
}

//easyjson:json
type UpdateSketchMetadataRequest struct {
	Name *string `json:"name,omitempty"`
	Unit *string `json:"unit,omitempty"`
}

//easyjson:json
type SketchPlane struct {
	Origin Vector3 `json:"origin"`
	Normal Vector3 `json:"normal"`
	XAxis  Vector3 `json:"xAxis"`
}

//easyjson:json
type Vector3 struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

//easyjson:json
type SketchMetadata struct {
	ID              string      `json:"id"`
	WorkspaceID     string      `json:"workspaceId"`
	Name            string      `json:"name"`
	CreatedByUserID string      `json:"createdByUserId"`
	Unit            string      `json:"unit"`
	Plane           SketchPlane `json:"plane"`
	Version         int64       `json:"version"`
	CreatedAt       string      `json:"createdAt"`
	UpdatedAt       string      `json:"updatedAt"`
}

//easyjson:json
type AvailableSketch struct {
	ID              string      `json:"id"`
	WorkspaceID     string      `json:"workspaceId"`
	Name            string      `json:"name"`
	CreatedByUserID string      `json:"createdByUserId"`
	Unit            string      `json:"unit"`
	Plane           SketchPlane `json:"plane"`
	Version         int64       `json:"version"`
	Role            string      `json:"role"`
	CreatedAt       string      `json:"createdAt"`
	UpdatedAt       string      `json:"updatedAt"`
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
	Plane           SketchPlane                    `json:"plane"`
	Version         int64                          `json:"version"`
	Entities        map[string]easyjson.RawMessage `json:"entities"`
	Constraints     map[string]easyjson.RawMessage `json:"constraints"`
	Dimensions      map[string]easyjson.RawMessage `json:"dimensions"`
	Groups          map[string]easyjson.RawMessage `json:"groups"`
	SolveStatus     easyjson.RawMessage            `json:"solveStatus"`
	Profiles        []easyjson.RawMessage          `json:"profiles"`
	Conflicts       []easyjson.RawMessage          `json:"conflicts,omitempty"`
}

//easyjson:json
type SketchSnapshot struct {
	SketchID             string              `json:"sketchId"`
	Version              int64               `json:"version"`
	GraphState           easyjson.RawMessage `json:"graphState"`
	MaterializedGeometry easyjson.RawMessage `json:"materializedGeometry"`
	SolveStatus          easyjson.RawMessage `json:"solveStatus"`
	CreatedAt            string              `json:"createdAt"`
}

//easyjson:json
type DeletedSketchEntityGeometry struct {
	SketchID             string              `json:"sketchId"`
	EntityID             string              `json:"entityId"`
	Version              int64               `json:"version"`
	Entity               easyjson.RawMessage `json:"entity"`
	MaterializedGeometry easyjson.RawMessage `json:"materializedGeometry"`
}
