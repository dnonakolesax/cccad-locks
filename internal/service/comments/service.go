package comments

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"strings"
	"sync"
	"time"

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
	DocumentWorkspace(ctx context.Context, documentID string, userID string) (string, error)
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

	mu          sync.Mutex
	subscribers map[string]map[string]chan model.CommentRealtimeEvent
}

func NewService(repo Repository) *Service {
	return &Service{
		repo:        repo,
		subscribers: make(map[string]map[string]chan model.CommentRealtimeEvent),
	}
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

type Subscription struct {
	id         string
	documentID string
	events     <-chan model.CommentRealtimeEvent
	close      func()
	closeOnce  sync.Once
}

func (s *Subscription) ID() string {
	return s.id
}

func (s *Subscription) DocumentID() string {
	return s.documentID
}

func (s *Subscription) Events() <-chan model.CommentRealtimeEvent {
	return s.events
}

func (s *Subscription) Close() {
	s.closeOnce.Do(s.close)
}

func (s *Service) SubscribeDocument(ctx context.Context, documentID string) (model.CommentSubscription, error) {
	documentID = strings.TrimSpace(documentID)
	if documentID == "" {
		return nil, errors.New("documentID is required")
	}
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, errors.New("authenticated user id is required")
	}
	if _, err := s.repo.DocumentWorkspace(ctx, documentID, userID); err != nil {
		return nil, err
	}

	id := newID()
	events := make(chan model.CommentRealtimeEvent, 32)
	s.mu.Lock()
	if s.subscribers[documentID] == nil {
		s.subscribers[documentID] = make(map[string]chan model.CommentRealtimeEvent)
	}
	s.subscribers[documentID][id] = events
	s.mu.Unlock()

	return &Subscription{
		id:         id,
		documentID: documentID,
		events:     events,
		close: func() {
			s.mu.Lock()
			if documentSubscribers := s.subscribers[documentID]; documentSubscribers != nil {
				delete(documentSubscribers, id)
				if len(documentSubscribers) == 0 {
					delete(s.subscribers, documentID)
				}
			}
			s.mu.Unlock()
			close(events)
		},
	}, nil
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
	comment, err := s.repo.Create(ctx, workspaceID, request, userID)
	if err != nil {
		return nil, err
	}
	s.publishCommentCreated(userID, comment)
	return comment, nil
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
	comment, err := s.repo.Update(ctx, commentID, request, userID)
	if err != nil {
		return nil, err
	}
	s.publishCommentUpdated(userID, comment)
	return comment, nil
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
	comment, _ := s.repo.Get(ctx, commentID, userID)
	if err := s.repo.Delete(ctx, commentID, userID); err != nil {
		return err
	}
	s.publishCommentDeleted(userID, comment)
	return nil
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
	oldComment, _ := s.repo.Get(ctx, commentID, userID)
	comment, err := s.repo.ChangeStatus(ctx, commentID, request, userID)
	if err != nil {
		return nil, err
	}
	s.publishCommentStatusChanged(userID, oldComment, comment, request.Reason)
	return comment, nil
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
	comment, err := s.repo.ReplaceAssignees(ctx, commentID, request, userID)
	if err != nil {
		return nil, err
	}
	s.publishCommentAssigneesChanged(userID, comment)
	return comment, nil
}

func (s *Service) publishCommentCreated(actorUserID string, comment *model.CadComment) {
	s.publishCommentEvent(actorUserID, comment, "comment.created", map[string]any{"comment": comment})
}

func (s *Service) publishCommentUpdated(actorUserID string, comment *model.CadComment) {
	if comment == nil {
		return
	}
	s.publishCommentEvent(actorUserID, comment, "comment.updated", map[string]any{
		"commentId": comment.ID,
		"comment":   comment,
	})
}

func (s *Service) publishCommentDeleted(actorUserID string, comment *model.CadComment) {
	if comment == nil {
		return
	}
	s.publishCommentEvent(actorUserID, comment, "comment.deleted", map[string]any{
		"commentId": comment.ID,
		"deletedAt": time.Now().UTC().Format(time.RFC3339Nano),
	})
}

func (s *Service) publishCommentStatusChanged(
	actorUserID string,
	oldComment *model.CadComment,
	comment *model.CadComment,
	reason *string,
) {
	if comment == nil {
		return
	}
	var oldStatus *string
	if oldComment != nil {
		oldStatus = &oldComment.Status
	}
	s.publishCommentEvent(actorUserID, comment, "comment.statusChanged", map[string]any{
		"commentId":       comment.ID,
		"oldStatus":       oldStatus,
		"newStatus":       comment.Status,
		"changedByUserId": actorUserID,
		"changedAt":       time.Now().UTC().Format(time.RFC3339Nano),
		"reason":          reason,
	})
}

func (s *Service) publishCommentAssigneesChanged(actorUserID string, comment *model.CadComment) {
	if comment == nil {
		return
	}
	s.publishCommentEvent(actorUserID, comment, "comment.assigneesChanged", map[string]any{
		"commentId":       comment.ID,
		"assigneeUserIds": comment.AssigneeUserIDs,
	})
}

func (s *Service) publishCommentEvent(actorUserID string, comment *model.CadComment, eventType string, payload any) {
	if comment == nil || comment.SketchID == nil || strings.TrimSpace(*comment.SketchID) == "" {
		return
	}
	payloadBody, err := json.Marshal(payload)
	if err != nil {
		return
	}
	event := model.CommentRealtimeEvent{
		Type:        eventType,
		EventID:     newID(),
		WorkspaceID: comment.WorkspaceID,
		OccurredAt:  time.Now().UTC().Format(time.RFC3339Nano),
		ActorUserID: actorUserID,
		Payload:     easyjson.RawMessage(payloadBody),
	}
	s.broadcastDocument(*comment.SketchID, event)
}

func (s *Service) broadcastDocument(documentID string, event model.CommentRealtimeEvent) {
	s.mu.Lock()
	recipients := make([]chan model.CommentRealtimeEvent, 0, len(s.subscribers[documentID]))
	for _, ch := range s.subscribers[documentID] {
		recipients = append(recipients, ch)
	}
	s.mu.Unlock()

	for _, ch := range recipients {
		select {
		case ch <- event:
		default:
		}
	}
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

func newID() string {
	var raw [16]byte
	if _, err := rand.Read(raw[:]); err != nil {
		return time.Now().UTC().Format("20060102150405.000000000")
	}
	raw[6] = (raw[6] & 0x0f) | 0x40
	raw[8] = (raw[8] & 0x3f) | 0x80

	encoded := make([]byte, 36)
	hex.Encode(encoded[0:8], raw[0:4])
	encoded[8] = '-'
	hex.Encode(encoded[9:13], raw[4:6])
	encoded[13] = '-'
	hex.Encode(encoded[14:18], raw[6:8])
	encoded[18] = '-'
	hex.Encode(encoded[19:23], raw[8:10])
	encoded[23] = '-'
	hex.Encode(encoded[24:36], raw[10:16])
	return string(encoded)
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
