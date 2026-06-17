//nolint:recvcheck // easyjson generates both value and pointer receiver methods for these DTOs.
package model

import "github.com/mailru/easyjson"

//go:generate easyjson -all comment.go

//easyjson:json
type CadComment struct {
	ID              string              `json:"id"`
	WorkspaceID     string              `json:"workspaceId"`
	SketchID        *string             `json:"sketchId,omitempty"`
	PartID          *string             `json:"partId,omitempty"`
	ParentCommentID *string             `json:"parentCommentId,omitempty"`
	ThreadRootID    string              `json:"threadRootId"`
	ReplyDepth      int                 `json:"replyDepth"`
	ReplyCount      int                 `json:"replyCount,omitempty"`
	MessageType     string              `json:"messageType"`
	SystemEventType *string             `json:"systemEventType,omitempty"`
	EventPayload    easyjson.RawMessage `json:"eventPayload"`
	TargetType      string              `json:"targetType"`
	TargetID        string              `json:"targetId"`
	Kind            string              `json:"kind"`
	Status          string              `json:"status"`
	AuthorUserID    string              `json:"authorUserId"`
	AssigneeUserIDs []string            `json:"assigneeUserIds"`
	Body            string              `json:"body"`
	SketchVersion   *int64              `json:"sketchVersion,omitempty"`
	PartVersion     *int64              `json:"partVersion,omitempty"`
	Anchor          easyjson.RawMessage `json:"anchor,omitempty"`
	Metadata        easyjson.RawMessage `json:"metadata"`
	CreatedAt       string              `json:"createdAt"`
	UpdatedAt       string              `json:"updatedAt"`
	DeletedAt       *string             `json:"deletedAt,omitempty"`
}

//easyjson:json
type CreateCommentRequest struct {
	ParentCommentID *string             `json:"parentCommentId,omitempty"`
	SketchID        *string             `json:"sketchId,omitempty"`
	PartID          *string             `json:"partId,omitempty"`
	TargetType      string              `json:"targetType,omitempty"`
	TargetID        string              `json:"targetId,omitempty"`
	Kind            string              `json:"kind,omitempty"`
	Body            string              `json:"body"`
	AssigneeUserIDs []string            `json:"assigneeUserIds,omitempty"`
	Status          string              `json:"status,omitempty"`
	SketchVersion   *int64              `json:"sketchVersion,omitempty"`
	PartVersion     *int64              `json:"partVersion,omitempty"`
	Anchor          easyjson.RawMessage `json:"anchor,omitempty"`
	Metadata        easyjson.RawMessage `json:"metadata,omitempty"`
}

//easyjson:json
type UpdateCommentRequest struct {
	Body     *string             `json:"body,omitempty"`
	Anchor   easyjson.RawMessage `json:"anchor,omitempty"`
	Metadata easyjson.RawMessage `json:"metadata,omitempty"`
}

//easyjson:json
type ChangeCommentStatusRequest struct {
	Status string  `json:"status"`
	Reason *string `json:"reason,omitempty"`
}

//easyjson:json
type ReplaceCommentAssigneesRequest struct {
	AssigneeUserIDs []string `json:"assigneeUserIds"`
}

//easyjson:json
type CreateCommentReplyRequest struct {
	Body            string              `json:"body"`
	AssigneeUserIDs []string            `json:"assigneeUserIds,omitempty"`
	Anchor          easyjson.RawMessage `json:"anchor,omitempty"`
	Metadata        easyjson.RawMessage `json:"metadata,omitempty"`
}

//easyjson:json
type CommentListResponse struct {
	Items  []CadComment `json:"items"`
	Limit  int          `json:"limit"`
	Offset int          `json:"offset"`
	Total  int          `json:"total"`
}

type CommentListFilter struct {
	WorkspaceID     string
	SketchID        string
	PartID          string
	TargetType      string
	TargetID        string
	Kind            string
	Status          string
	MessageType     string
	SystemEventType string
	AssigneeUserID  string
	ParentCommentID string
	ThreadRootID    string
	RootsOnly       bool
	IncludeDeleted  bool
	IncludeSystem   bool
	MaxDepth        int
	Limit           int
	Offset          int
}

//easyjson:json
type ChangeCommentStatusResponse struct {
	Comment       CadComment `json:"comment"`
	SystemMessage CadComment `json:"systemMessage"`
}

//easyjson:json
type ChangeCommentAssigneesResponse struct {
	Comment       CadComment `json:"comment"`
	SystemMessage CadComment `json:"systemMessage"`
}

//easyjson:json
type CommentThreadResponse struct {
	Root  CadComment   `json:"root"`
	Items []CadComment `json:"items"`
}

//easyjson:json
type CommentStatusHistoryItem struct {
	ID              string  `json:"id"`
	CommentID       string  `json:"commentId"`
	OldStatus       *string `json:"oldStatus,omitempty"`
	NewStatus       string  `json:"newStatus"`
	ChangedByUserID string  `json:"changedByUserId"`
	ChangedAt       string  `json:"changedAt"`
	Reason          *string `json:"reason,omitempty"`
}

//easyjson:json
type CommentStatusHistoryResponse struct {
	Items []CommentStatusHistoryItem `json:"items"`
}

//easyjson:json
type CommentEditHistoryItem struct {
	ID             string `json:"id"`
	CommentID      string `json:"commentId"`
	OldBody        string `json:"oldBody"`
	NewBody        string `json:"newBody"`
	EditedByUserID string `json:"editedByUserId"`
	EditedAt       string `json:"editedAt"`
}

//easyjson:json
type CommentEditHistoryResponse struct {
	Items []CommentEditHistoryItem `json:"items"`
}

//easyjson:json
type CommentRealtimeEvent struct {
	Type        string              `json:"type"`
	EventID     string              `json:"eventId"`
	WorkspaceID string              `json:"workspaceId"`
	OccurredAt  string              `json:"occurredAt"`
	ActorUserID string              `json:"actorUserId"`
	Payload     easyjson.RawMessage `json:"payload"`
}

type CommentSubscription interface {
	ID() string
	DocumentID() string
	Events() <-chan CommentRealtimeEvent
	Close()
}
