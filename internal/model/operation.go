//nolint:recvcheck // easyjson generates both value and pointer receiver methods for these DTOs.
package model

import "github.com/mailru/easyjson"

//easyjson:json
type SubmitOperationRequest struct {
	BaseVersion int64               `json:"baseVersion"`
	ClientOpID  string              `json:"clientOpId"`
	LockID      *string             `json:"lockId,omitempty"`
	Op          easyjson.RawMessage `json:"op"`
}

//easyjson:json
type SubmitOperationResponse struct {
	Accepted         bool                  `json:"accepted"`
	Duplicate        bool                  `json:"duplicate,omitempty"`
	OpID             *string               `json:"opId,omitempty"`
	Version          *int64                `json:"version,omitempty"`
	CurrentVersion   int64                 `json:"currentVersion"`
	Patch            easyjson.RawMessage   `json:"patch,omitempty"`
	SolveStatus      easyjson.RawMessage   `json:"solveStatus,omitempty"`
	ChangedEntityIDs []string              `json:"changedEntityIds,omitempty"`
	Conflicts        []easyjson.RawMessage `json:"conflicts,omitempty"`
	Rejection        *OperationRejection   `json:"rejection,omitempty"`
}

//easyjson:json
type SketchOperationPage struct {
	SketchID             string               `json:"sketchId"`
	FromVersionExclusive int64                `json:"fromVersionExclusive"`
	ToVersion            int64                `json:"toVersion"`
	Ops                  []CommittedOperation `json:"ops"`
}

//easyjson:json
type CommittedOperation struct {
	ID          string              `json:"id"`
	SketchID    string              `json:"sketchId"`
	Version     int64               `json:"version"`
	ActorUserID string              `json:"actorUserId"`
	ClientOpID  *string             `json:"clientOpId,omitempty"`
	CreatedAt   string              `json:"createdAt"`
	Payload     easyjson.RawMessage `json:"payload"`
}

//easyjson:json
type OperationRejection struct {
	Reason      string                `json:"reason"`
	Message     string                `json:"message"`
	Diagnostics []easyjson.RawMessage `json:"diagnostics,omitempty"`
}
