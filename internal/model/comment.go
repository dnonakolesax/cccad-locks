//nolint:recvcheck // easyjson generates both value and pointer receiver methods for these DTOs.
package model

import "github.com/mailru/easyjson"

//go:generate easyjson -all comment.go

//easyjson:json
type CadComment struct {
	ID              string              `json:"id"`
	WorkspaceID     string              `json:"workspaceId"`
	DocumentID      string              `json:"documentId"`
	PartID          *string             `json:"partId,omitempty"`
	TargetType      string              `json:"targetType"`
	TargetID        string              `json:"targetId"`
	Kind            string              `json:"kind"`
	Status          string              `json:"status"`
	AuthorID        string              `json:"authorId"`
	AssigneeIDs     []string            `json:"assigneeIds"`
	Body            string              `json:"body"`
	DocumentVersion *int64              `json:"documentVersion,omitempty"`
	PartVersion     *int64              `json:"partVersion,omitempty"`
	Anchor          easyjson.RawMessage `json:"anchor,omitempty"`
	Metadata        easyjson.RawMessage `json:"metadata"`
	CreatedAt       string              `json:"createdAt"`
	UpdatedAt       string              `json:"updatedAt"`
	DeletedAt       *string             `json:"deletedAt,omitempty"`
}

//easyjson:json
type CreateCommentRequest struct {
	PartID          *string             `json:"partId,omitempty"`
	TargetType      string              `json:"targetType"`
	TargetID        string              `json:"targetId"`
	Kind            string              `json:"kind,omitempty"`
	Body            string              `json:"body"`
	AssigneeIDs     []string            `json:"assigneeIds,omitempty"`
	Status          string              `json:"status,omitempty"`
	DocumentVersion *int64              `json:"documentVersion,omitempty"`
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
	AssigneeIDs []string `json:"assigneeIds"`
}

//easyjson:json
type CommentListResponse struct {
	Items  []CadComment `json:"items"`
	Limit  int          `json:"limit"`
	Offset int          `json:"offset"`
	Total  int          `json:"total"`
}

type CommentListFilter struct {
	DocumentID     string
	PartID         string
	TargetType     string
	TargetID       string
	Kind           string
	Status         string
	AssigneeID     string
	IncludeDeleted bool
	Limit          int
	Offset         int
}

//easyjson:json
type CommentStatusHistoryItem struct {
	ID        string  `json:"id"`
	CommentID string  `json:"commentId"`
	OldStatus *string `json:"oldStatus,omitempty"`
	NewStatus string  `json:"newStatus"`
	ChangedBy string  `json:"changedBy"`
	ChangedAt string  `json:"changedAt"`
	Reason    *string `json:"reason,omitempty"`
}

//easyjson:json
type CommentStatusHistoryResponse struct {
	Items []CommentStatusHistoryItem `json:"items"`
}

//easyjson:json
type CommentEditHistoryItem struct {
	ID        string `json:"id"`
	CommentID string `json:"commentId"`
	OldBody   string `json:"oldBody"`
	NewBody   string `json:"newBody"`
	EditedBy  string `json:"editedBy"`
	EditedAt  string `json:"editedAt"`
}

//easyjson:json
type CommentEditHistoryResponse struct {
	Items []CommentEditHistoryItem `json:"items"`
}
