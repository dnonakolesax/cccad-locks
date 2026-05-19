//nolint:recvcheck // easyjson generates both value and pointer receiver methods for these DTOs.
package model

import "github.com/mailru/easyjson"

//easyjson:json
type SolvePreviewRequest struct {
	BaseVersion int64               `json:"baseVersion"`
	Intent      easyjson.RawMessage `json:"intent"`
	Options     easyjson.RawMessage `json:"options,omitempty"`
}

//easyjson:json
type SolvePreviewResponse struct {
	Status            SolveStatusInfo     `json:"status"`
	Patch             easyjson.RawMessage `json:"patch"`
	AffectedEntityIDs []string            `json:"affectedEntityIds"`
	Diagnostics       []SolverDiagnostic  `json:"diagnostics"`
}

//easyjson:json
type AnalyzeSketchResponse struct {
	Status           SolveStatusInfo       `json:"status"`
	DegreesOfFreedom int                   `json:"degreesOfFreedom"`
	Components       []ConstraintComponent `json:"components"`
	Diagnostics      []SolverDiagnostic    `json:"diagnostics"`
}

//easyjson:json
type SolveStatusInfo struct {
	Status           string  `json:"status"`
	DegreesOfFreedom int     `json:"degreesOfFreedom,omitempty"`
	Message          *string `json:"message,omitempty"`
}

//easyjson:json
type SolverDiagnostic struct {
	Level         string   `json:"level"`
	Code          string   `json:"code"`
	Message       string   `json:"message"`
	EntityIDs     []string `json:"entityIds,omitempty"`
	ConstraintIDs []string `json:"constraintIds,omitempty"`
	DimensionIDs  []string `json:"dimensionIds,omitempty"`
}

//easyjson:json
type ConstraintComponent struct {
	ID               string   `json:"id"`
	EntityIDs        []string `json:"entityIds"`
	ConstraintIDs    []string `json:"constraintIds"`
	DegreesOfFreedom int      `json:"degreesOfFreedom"`
}

//easyjson:json
type SketchConflict struct {
	ID                string              `json:"id"`
	Type              string              `json:"type"`
	Status            string              `json:"status"`
	AffectedEntityIDs []string            `json:"affectedEntityIds"`
	CausedByOpIDs     []string            `json:"causedByOpIds"`
	Message           string              `json:"message,omitempty"`
	Payload           easyjson.RawMessage `json:"payload,omitempty"`
}
