//nolint:recvcheck // easyjson generates both value and pointer receiver methods for these DTOs.
package model

import "github.com/mailru/easyjson"

//easyjson:json
type AcquireLockRequest struct {
	Scope easyjson.RawMessage `json:"scope"`
	Mode  string              `json:"mode"`
	TTLMS int                 `json:"ttlMs,omitempty"`
}

//easyjson:json
type AcquireLockResponse struct {
	Granted  bool          `json:"granted"`
	Lock     *SketchLock   `json:"lock,omitempty"`
	Conflict *LockConflict `json:"conflict,omitempty"`
}

//easyjson:json
type RefreshLockRequest struct {
	TTLMS int `json:"ttlMs,omitempty"`
}

//easyjson:json
type SketchLock struct {
	ID          string              `json:"id"`
	SketchID    string              `json:"sketchId"`
	OwnerUserID string              `json:"ownerUserId"`
	Scope       easyjson.RawMessage `json:"scope"`
	Mode        string              `json:"mode"`
	ExpiresAt   string              `json:"expiresAt"`
}

//easyjson:json
type LockConflict struct {
	HolderUserID string              `json:"holderUserId"`
	LockID       string              `json:"lockId"`
	Scope        easyjson.RawMessage `json:"scope"`
}
