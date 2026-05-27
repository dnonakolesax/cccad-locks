//nolint:recvcheck // easyjson generates both value and pointer receiver methods for these DTOs.
package model

import "encoding/json"

//go:generate easyjson -all realtime_messages.go

//easyjson:json
type ClientRealtimeMessage struct {
	Type       string          `json:"type"`
	RequestID  string          `json:"requestId,omitempty"`
	SketchID   string          `json:"sketchId"`
	ClientTime string          `json:"clientTime,omitempty"`
	Payload    json.RawMessage `json:"payload,omitempty"`
}

//easyjson:json
type ServerRealtimeMessage struct {
	Type       string          `json:"type"`
	EventID    string          `json:"eventId,omitempty"`
	RequestID  string          `json:"requestId,omitempty"`
	SketchID   string          `json:"sketchId"`
	ServerTime string          `json:"serverTime"`
	Payload    json.RawMessage `json:"payload,omitempty"`
}

//easyjson:json
type RealtimeErrorPayload struct {
	Code    string          `json:"code"`
	Message string          `json:"message"`
	Details json.RawMessage `json:"details,omitempty"`
}

//easyjson:json
type OpenRealtimeSessionRequest struct {
	SketchID    string `json:"sketchId"`
	UserID      string `json:"userId"`
	UserName    string `json:"userName,omitempty"`
	ClientID    string `json:"clientId,omitempty"`
	RemoteAddr  string `json:"remoteAddr,omitempty"`
	UserAgent   string `json:"userAgent,omitempty"`
	AccessToken string `json:"-"`
}

//easyjson:json
type UserPresenceSummary struct {
	UserID      string `json:"userId"`
	DisplayName string `json:"displayName"`
	Role        string `json:"role"`
	ClientID    string `json:"clientId"`
}

//easyjson:json
type SessionJoinPayload struct {
	LastSeenVersion          int64  `json:"lastSeenVersion"`
	ClientID                 string `json:"clientId"`
	SupportedProtocolVersion int    `json:"supportedProtocolVersion"`
}

//easyjson:json
type SessionJoinedPayload struct {
	ProtocolVersion     int                   `json:"protocolVersion"`
	CurrentVersion      int64                 `json:"currentVersion"`
	User                UserPresenceSummary   `json:"user"`
	ActiveUsers         []UserPresenceSummary `json:"activeUsers"`
	MissingOpsAvailable bool                  `json:"missingOpsAvailable"`
}

//easyjson:json
type SessionUserLeftPayload struct {
	UserID   string `json:"userId"`
	ClientID string `json:"clientId"`
	Reason   string `json:"reason"`
}

//easyjson:json
type SessionAccessRevokedPayload struct {
	Message string `json:"message"`
}

//easyjson:json
type SessionPingPayload struct {
	ClientVersion int64 `json:"clientVersion"`
}

//easyjson:json
type SessionPongPayload struct {
	CurrentVersion int64 `json:"currentVersion"`
}

//easyjson:json
type CursorPayload struct {
	X        float64          `json:"x"`
	Y        float64          `json:"y"`
	Viewport *ViewportPayload `json:"viewport,omitempty"`
}

//easyjson:json
type ViewportPayload struct {
	Scale   float64 `json:"scale"`
	OffsetX float64 `json:"offsetX"`
	OffsetY float64 `json:"offsetY"`
}

//easyjson:json
type SelectionPayload struct {
	EntityIDs     []string `json:"entityIds,omitempty"`
	ConstraintIDs []string `json:"constraintIds,omitempty"`
	DimensionIDs  []string `json:"dimensionIds,omitempty"`
}

//easyjson:json
type HoverPayload struct {
	Kind string `json:"kind"`
	ID   string `json:"id"`
}

//easyjson:json
type ToolPayload struct {
	Tool string `json:"tool"`
}

//easyjson:json
type IntentDraftPayload struct {
	DraftID           string          `json:"draftId"`
	ActorUserID       string          `json:"actorUserId"`
	ClientID          string          `json:"clientId"`
	BaseVersion       int64           `json:"baseVersion"`
	Tool              string          `json:"tool"`
	OperationType     string          `json:"operationType,omitempty"`
	Phase             string          `json:"phase"`
	Sequence          int64           `json:"sequence,omitempty"`
	SelectedEntityIDs []string        `json:"selectedEntityIds,omitempty"`
	SelectedPointIDs  []string        `json:"selectedPointIds,omitempty"`
	HoverEntityID     string          `json:"hoverEntityId,omitempty"`
	HoverPointID      string          `json:"hoverPointId,omitempty"`
	CursorWorld       *PointPayload   `json:"cursorWorld,omitempty"`
	Anchors           []IntentAnchor  `json:"anchors,omitempty"`
	Preview           json.RawMessage `json:"preview,omitempty"`
	StyleHint         string          `json:"styleHint,omitempty"`
	TTLMS             int64           `json:"ttlMs,omitempty"`
	Metadata          json.RawMessage `json:"metadata,omitempty"`
}

//easyjson:json
type IntentAnchor struct {
	Kind         string        `json:"kind"`
	PointID      string        `json:"pointId,omitempty"`
	EntityID     string        `json:"entityId,omitempty"`
	ConstraintID string        `json:"constraintId,omitempty"`
	DimensionID  string        `json:"dimensionId,omitempty"`
	Position     *PointPayload `json:"position,omitempty"`
}

//easyjson:json
type IntentDraftCancelPayload struct {
	DraftID     string `json:"draftId"`
	ActorUserID string `json:"actorUserId,omitempty"`
	ClientID    string `json:"clientId,omitempty"`
	Reason      string `json:"reason,omitempty"`
}

//easyjson:json
type IntentDraftEndedPayload struct {
	DraftID          string `json:"draftId"`
	ActorUserID      string `json:"actorUserId"`
	ClientID         string `json:"clientId,omitempty"`
	Reason           string `json:"reason"`
	CommittedOpID    string `json:"committedOpId,omitempty"`
	CommittedVersion int64  `json:"committedVersion,omitempty"`
}

//easyjson:json
type DragBeginPayload struct {
	EntityID    string `json:"entityId"`
	Kind        string `json:"kind"`
	BaseVersion int64  `json:"baseVersion"`
}

//easyjson:json
type DragBeginAcceptedPayload struct {
	LockID          string   `json:"lockId"`
	LockedEntityIDs []string `json:"lockedEntityIds,omitempty"`
	ComponentID     string   `json:"componentId,omitempty"`
}

//easyjson:json
type DragBeginRejectedPayload struct {
	Reason         string `json:"reason"`
	LockedByUserID string `json:"lockedByUserId,omitempty"`
	LockID         string `json:"lockId,omitempty"`
}

//easyjson:json
type DragPreviewPayload struct {
	LockID   string       `json:"lockId,omitempty"`
	EntityID string       `json:"entityId"`
	Target   PointPayload `json:"target"`
}

//easyjson:json
type DragCommitPayload struct {
	LockID      string          `json:"lockId,omitempty"`
	BaseVersion int64           `json:"baseVersion"`
	ClientOpID  string          `json:"clientOpId"`
	Op          json.RawMessage `json:"op"`
}

//easyjson:json
type DragCancelPayload struct {
	LockID   string `json:"lockId,omitempty"`
	EntityID string `json:"entityId"`
}

//easyjson:json
type OpSubmitPayload struct {
	BaseVersion int64           `json:"baseVersion"`
	ClientOpID  string          `json:"clientOpId"`
	Op          json.RawMessage `json:"op"`
}

//easyjson:json
type OpCommittedPayload struct {
	OpID                  string          `json:"opId"`
	Version               int64           `json:"version"`
	ActorUserID           string          `json:"actorUserId"`
	ClientOpID            string          `json:"clientOpId,omitempty"`
	Op                    json.RawMessage `json:"op"`
	Patch                 json.RawMessage `json:"patch"`
	SolveStatus           json.RawMessage `json:"solveStatus"`
	AffectedEntityIDs     []string        `json:"affectedEntityIds"`
	AffectedConstraintIDs []string        `json:"affectedConstraintIds,omitempty"`
	AffectedDimensionIDs  []string        `json:"affectedDimensionIds,omitempty"`
	AffectedComponentIDs  []string        `json:"affectedComponentIds,omitempty"`
	Authoritative         bool            `json:"authoritative"`
}

//easyjson:json
type OpRejectedPayload struct {
	ClientOpID     string          `json:"clientOpId,omitempty"`
	CurrentVersion int64           `json:"currentVersion"`
	Reason         string          `json:"reason"`
	Message        string          `json:"message,omitempty"`
	Diagnostics    json.RawMessage `json:"diagnostics,omitempty"`
}

//easyjson:json
type SyncResumePayload struct {
	ClientID           string                    `json:"clientId"`
	LastSeenVersion    int64                     `json:"lastSeenVersion"`
	LastAckedClientSeq int64                     `json:"lastAckedClientSeq,omitempty"`
	PendingOps         []PendingOfflineOperation `json:"pendingOps"`
}

//easyjson:json
type PendingOfflineOperation struct {
	ClientOpID  string          `json:"clientOpId"`
	ClientSeq   int64           `json:"clientSeq"`
	BaseVersion int64           `json:"baseVersion"`
	CreatedAt   string          `json:"createdAt,omitempty"`
	Op          json.RawMessage `json:"op"`
}

//easyjson:json
type SyncResumeResultPayload struct {
	Status         string                   `json:"status"`
	ClientID       string                   `json:"clientId"`
	FromVersion    int64                    `json:"fromVersion,omitempty"`
	CurrentVersion int64                    `json:"currentVersion"`
	MissedPatches  []CommittedPatchPayload  `json:"missedPatches,omitempty"`
	OpResults      []OfflineOperationResult `json:"opResults"`
	Snapshot       *StateSnapshotPayload    `json:"snapshot,omitempty"`
	Message        string                   `json:"message,omitempty"`
}

//easyjson:json
type CommittedPatchPayload struct {
	Version               int64           `json:"version"`
	OpID                  string          `json:"opId,omitempty"`
	ActorUserID           string          `json:"actorUserId,omitempty"`
	ClientOpID            string          `json:"clientOpId,omitempty"`
	Patch                 json.RawMessage `json:"patch"`
	SolveStatus           json.RawMessage `json:"solveStatus,omitempty"`
	AffectedEntityIDs     []string        `json:"affectedEntityIds,omitempty"`
	AffectedConstraintIDs []string        `json:"affectedConstraintIds,omitempty"`
	AffectedDimensionIDs  []string        `json:"affectedDimensionIds,omitempty"`
	AffectedComponentIDs  []string        `json:"affectedComponentIds,omitempty"`
	Authoritative         bool            `json:"authoritative"`
}

//easyjson:json
type OfflineOperationResult struct {
	ClientOpID            string          `json:"clientOpId"`
	ClientSeq             int64           `json:"clientSeq"`
	Status                string          `json:"status"`
	CommittedVersion      int64           `json:"committedVersion,omitempty"`
	CurrentVersion        int64           `json:"currentVersion,omitempty"`
	OpID                  string          `json:"opId,omitempty"`
	Patch                 json.RawMessage `json:"patch,omitempty"`
	SolveStatus           json.RawMessage `json:"solveStatus,omitempty"`
	Reason                string          `json:"reason,omitempty"`
	Message               string          `json:"message,omitempty"`
	Diagnostics           json.RawMessage `json:"diagnostics,omitempty"`
	AffectedEntityIDs     []string        `json:"affectedEntityIds,omitempty"`
	AffectedConstraintIDs []string        `json:"affectedConstraintIds,omitempty"`
	AffectedDimensionIDs  []string        `json:"affectedDimensionIds,omitempty"`
	AffectedComponentIDs  []string        `json:"affectedComponentIds,omitempty"`
	Authoritative         bool            `json:"authoritative"`
}

//easyjson:json
type LockScopePayload struct {
	Type         string `json:"type"`
	EntityID     string `json:"entityId,omitempty"`
	ConstraintID string `json:"constraintId,omitempty"`
	DimensionID  string `json:"dimensionId,omitempty"`
	ComponentID  string `json:"componentId,omitempty"`
}

//easyjson:json
type LockAcquirePayload struct {
	Scope LockScopePayload `json:"scope"`
	Mode  string           `json:"mode"`
	TTLMS int64            `json:"ttlMs"`
}

//easyjson:json
type LockAcquiredPayload struct {
	LockID    string           `json:"lockId"`
	UserID    string           `json:"userId"`
	Scope     LockScopePayload `json:"scope"`
	EntityIDs []string         `json:"entityIds,omitempty"`
	ExpiresAt string           `json:"expiresAt"`
}

//easyjson:json
type LockRejectedPayload struct {
	Reason         string `json:"reason"`
	LockedByUserID string `json:"lockedByUserId,omitempty"`
	LockID         string `json:"lockId,omitempty"`
}

//easyjson:json
type LockRefreshPayload struct {
	LockID string `json:"lockId"`
	TTLMS  int64  `json:"ttlMs"`
}

//easyjson:json
type LockRefreshedPayload struct {
	LockID    string `json:"lockId"`
	ExpiresAt string `json:"expiresAt"`
}

//easyjson:json
type LockReleasePayload struct {
	LockID string `json:"lockId"`
}

//easyjson:json
type LockReleasedPayload struct {
	LockID string `json:"lockId"`
	Reason string `json:"reason"`
	UserID string `json:"userId,omitempty"`
}

//easyjson:json
type PermissionUpdatedPayload struct {
	TargetUserID    string `json:"targetUserId"`
	Role            string `json:"role"`
	ChangedByUserID string `json:"changedByUserId"`
}

//easyjson:json
type PermissionRevokedPayload struct {
	TargetUserID    string `json:"targetUserId"`
	ChangedByUserID string `json:"changedByUserId"`
}

//easyjson:json
type ConflictCreatedPayload struct {
	ConflictID          string            `json:"conflictId"`
	ConflictType        string            `json:"conflictType"`
	Status              string            `json:"status"`
	AffectedEntityIDs   []string          `json:"affectedEntityIds"`
	CausedByOps         []string          `json:"causedByOps"`
	Message             string            `json:"message"`
	PossibleResolutions []json.RawMessage `json:"possibleResolutions,omitempty"`
}

//easyjson:json
type ConflictResolvedPayload struct {
	ConflictID       string `json:"conflictId"`
	ResolvedByUserID string `json:"resolvedByUserId"`
	ResolutionOpID   string `json:"resolutionOpId,omitempty"`
}

//easyjson:json
type StateResyncRequiredPayload struct {
	CurrentVersion    int64  `json:"currentVersion"`
	Reason            string `json:"reason"`
	RecommendedAction string `json:"recommendedAction"`
}

//easyjson:json
type StateSnapshotPayload struct {
	Version  int64           `json:"version"`
	Document json.RawMessage `json:"document"`
}

//easyjson:json
type StatePatchPayload struct {
	Version int64           `json:"version"`
	Patch   json.RawMessage `json:"patch"`
}

//easyjson:json
type PointPayload struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}
