package comments

import (
	"context"
	"encoding/json"
	"errors"
	"strings"

	"github.com/dnonakolesax/cccad-locks/internal/auth"
	"github.com/dnonakolesax/cccad-locks/internal/model"
	"github.com/mailru/easyjson"
)

const (
	defaultKind   = "comment"
	defaultStatus = "open"
	defaultLimit  = 50
	maxLimit      = 200
)

type Repository interface {
	List(ctx context.Context, filter model.CommentListFilter, userID string) (*model.CommentListResponse, error)
	Get(ctx context.Context, commentID string, userID string) (*model.CadComment, error)
	Create(ctx context.Context, workspaceID string, request *model.CreateCommentRequest, actorUserID string) (*model.CadComment, error)
	Update(ctx context.Context, commentID string, request *model.UpdateCommentRequest, actorUserID string) (*model.CadComment, error)
	Delete(ctx context.Context, commentID string, actorUserID string) error
	ChangeStatus(ctx context.Context, commentID string, request *model.ChangeCommentStatusRequest, actorUserID string) (*model.CadComment, error)
	ReplaceAssignees(ctx context.Context, commentID string, request *model.ReplaceCommentAssigneesRequest, actorUserID string) (*model.CadComment, error)
	StatusHistory(ctx context.Context, commentID string, userID string) ([]model.CommentStatusHistoryItem, error)
	EditHistory(ctx context.Context, commentID string, userID string) ([]model.CommentEditHistoryItem, error)
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) List(ctx context.Context, filter model.CommentListFilter) (*model.CommentListResponse, error) {
	filter.WorkspaceID = strings.TrimSpace(filter.WorkspaceID)
	if filter.WorkspaceID == "" {
		return nil, errors.New("workspaceID is required")
	}
	filter.SketchID = strings.TrimSpace(filter.SketchID)
	filter.PartID = strings.TrimSpace(filter.PartID)
	filter.TargetType = strings.TrimSpace(filter.TargetType)
	filter.TargetID = strings.TrimSpace(filter.TargetID)
	filter.Kind = strings.TrimSpace(filter.Kind)
	filter.Status = strings.TrimSpace(filter.Status)
	filter.AssigneeUserID = strings.TrimSpace(filter.AssigneeUserID)
	if filter.Limit <= 0 {
		filter.Limit = defaultLimit
	}
	if filter.Limit > maxLimit {
		filter.Limit = maxLimit
	}
	if filter.Offset < 0 {
		filter.Offset = 0
	}
	if filter.TargetType != "" && !isValidTargetType(filter.TargetType) {
		return nil, errors.New("targetType is invalid")
	}
	if filter.Kind != "" && !isValidKind(filter.Kind) {
		return nil, errors.New("kind is invalid")
	}
	if filter.Status != "" && !isValidStatus(filter.Status) {
		return nil, errors.New("status is invalid")
	}

	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, errors.New("authenticated user id is required")
	}

	return s.repo.List(ctx, filter, userID)
}

func (s *Service) Get(ctx context.Context, commentID string) (*model.CadComment, error) {
	commentID = strings.TrimSpace(commentID)
	if commentID == "" {
		return nil, errors.New("commentID is required")
	}
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, errors.New("authenticated user id is required")
	}
	return s.repo.Get(ctx, commentID, userID)
}

func (s *Service) Create(
	ctx context.Context,
	workspaceID string,
	request *model.CreateCommentRequest,
) (*model.CadComment, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return nil, errors.New("workspaceID is required")
	}
	if request == nil {
		return nil, errors.New("request is required")
	}
	trimOptionalString(&request.SketchID)
	trimOptionalString(&request.PartID)
	request.TargetType = strings.TrimSpace(request.TargetType)
	request.TargetID = strings.TrimSpace(request.TargetID)
	request.Body = strings.TrimSpace(request.Body)
	request.Kind = strings.TrimSpace(request.Kind)
	request.Status = strings.TrimSpace(request.Status)
	if request.Kind == "" {
		request.Kind = defaultKind
	}
	if request.Status == "" {
		request.Status = defaultStatus
	}
	if !isValidTargetType(request.TargetType) {
		return nil, errors.New("targetType is invalid")
	}
	if request.TargetID == "" {
		return nil, errors.New("targetId is required")
	}
	if !isValidKind(request.Kind) {
		return nil, errors.New("kind is invalid")
	}
	if !isValidStatus(request.Status) {
		return nil, errors.New("status is invalid")
	}
	if request.Body == "" {
		return nil, errors.New("body is required")
	}
	if request.SketchVersion != nil && *request.SketchVersion < 0 {
		return nil, errors.New("sketchVersion must be nonnegative")
	}
	if request.PartVersion != nil && *request.PartVersion < 0 {
		return nil, errors.New("partVersion must be nonnegative")
	}
	if err := validateJSONObject(request.Anchor, true, "anchor"); err != nil {
		return nil, err
	}
	if len(request.Metadata) == 0 {
		request.Metadata = easyjson.RawMessage(`{}`)
	}
	if err := validateJSONObject(request.Metadata, false, "metadata"); err != nil {
		return nil, err
	}
	normalizeAssignees(&request.AssigneeUserIDs)

	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, errors.New("authenticated user id is required")
	}
	return s.repo.Create(ctx, workspaceID, request, userID)
}

func (s *Service) Update(
	ctx context.Context,
	commentID string,
	request *model.UpdateCommentRequest,
) (*model.CadComment, error) {
	commentID = strings.TrimSpace(commentID)
	if commentID == "" {
		return nil, errors.New("commentID is required")
	}
	if request == nil {
		return nil, errors.New("request is required")
	}
	if request.Body == nil && len(request.Anchor) == 0 && len(request.Metadata) == 0 {
		return nil, errors.New("body, anchor, or metadata is required")
	}
	if request.Body != nil {
		trimmed := strings.TrimSpace(*request.Body)
		if trimmed == "" {
			return nil, errors.New("body must not be empty")
		}
		request.Body = &trimmed
	}
	if err := validateJSONObject(request.Anchor, true, "anchor"); err != nil {
		return nil, err
	}
	if err := validateJSONObject(request.Metadata, true, "metadata"); err != nil {
		return nil, err
	}
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, errors.New("authenticated user id is required")
	}
	return s.repo.Update(ctx, commentID, request, userID)
}

func (s *Service) Delete(ctx context.Context, commentID string) error {
	commentID = strings.TrimSpace(commentID)
	if commentID == "" {
		return errors.New("commentID is required")
	}
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return errors.New("authenticated user id is required")
	}
	return s.repo.Delete(ctx, commentID, userID)
}

func (s *Service) ChangeStatus(
	ctx context.Context,
	commentID string,
	request *model.ChangeCommentStatusRequest,
) (*model.CadComment, error) {
	commentID = strings.TrimSpace(commentID)
	if commentID == "" {
		return nil, errors.New("commentID is required")
	}
	if request == nil {
		return nil, errors.New("request is required")
	}
	request.Status = strings.TrimSpace(request.Status)
	if !isValidStatus(request.Status) {
		return nil, errors.New("status is invalid")
	}
	if request.Reason != nil {
		trimmed := strings.TrimSpace(*request.Reason)
		if trimmed == "" {
			request.Reason = nil
		} else {
			request.Reason = &trimmed
		}
	}
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, errors.New("authenticated user id is required")
	}
	return s.repo.ChangeStatus(ctx, commentID, request, userID)
}

func (s *Service) ReplaceAssignees(
	ctx context.Context,
	commentID string,
	request *model.ReplaceCommentAssigneesRequest,
) (*model.CadComment, error) {
	commentID = strings.TrimSpace(commentID)
	if commentID == "" {
		return nil, errors.New("commentID is required")
	}
	if request == nil {
		return nil, errors.New("request is required")
	}
	normalizeAssignees(&request.AssigneeUserIDs)
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, errors.New("authenticated user id is required")
	}
	return s.repo.ReplaceAssignees(ctx, commentID, request, userID)
}

func (s *Service) StatusHistory(ctx context.Context, commentID string) ([]model.CommentStatusHistoryItem, error) {
	commentID = strings.TrimSpace(commentID)
	if commentID == "" {
		return nil, errors.New("commentID is required")
	}
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, errors.New("authenticated user id is required")
	}
	return s.repo.StatusHistory(ctx, commentID, userID)
}

func (s *Service) EditHistory(ctx context.Context, commentID string) ([]model.CommentEditHistoryItem, error) {
	commentID = strings.TrimSpace(commentID)
	if commentID == "" {
		return nil, errors.New("commentID is required")
	}
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, errors.New("authenticated user id is required")
	}
	return s.repo.EditHistory(ctx, commentID, userID)
}

func validateJSONObject(raw easyjson.RawMessage, allowEmpty bool, field string) error {
	if len(raw) == 0 {
		if allowEmpty {
			return nil
		}
		return errors.New(field + " is required")
	}
	if string(raw) == "null" && allowEmpty {
		return nil
	}
	var value map[string]any
	if err := json.Unmarshal(raw, &value); err != nil {
		return errors.New(field + " must be a JSON object")
	}
	if value == nil && !allowEmpty {
		return errors.New(field + " must be a JSON object")
	}
	return nil
}

func normalizeAssignees(values *[]string) {
	seen := make(map[string]struct{}, len(*values))
	normalized := make([]string, 0, len(*values))
	for _, value := range *values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		if _, ok := seen[value]; ok {
			continue
		}
		seen[value] = struct{}{}
		normalized = append(normalized, value)
	}
	*values = normalized
}

func trimOptionalString(value **string) {
	if *value == nil {
		return
	}
	trimmed := strings.TrimSpace(**value)
	if trimmed == "" {
		*value = nil
		return
	}
	*value = &trimmed
}

func isValidKind(kind string) bool {
	return kind == "comment" || kind == "task"
}

func isValidStatus(status string) bool {
	switch status {
	case "open", "in_progress", "resolved", "reopened", "closed", "rejected":
		return true
	default:
		return false
	}
}

func isValidTargetType(targetType string) bool {
	switch targetType {
	case "workspace", "sketch", "sketch_entity", "constraint", "part", "feature_3d",
		"body", "face", "edge", "vertex", "profile", "topology_ref_3d",
		"representation_3d", "simulation_job",
		"simulation_result", "mesh_entity":
		return true
	default:
		return false
	}
}
