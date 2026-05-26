package operations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/dnonakolesax/cccad-locks/internal/auth"
	"github.com/dnonakolesax/cccad-locks/internal/model"
	"github.com/mailru/easyjson"
)

type Repository interface {
	List(
		ctx context.Context,
		userID string,
		sketchID string,
		afterVersion int64,
		limit int,
	) (*model.SketchOperationPage, error)
	GetSubmitState(ctx context.Context, userID string, sketchID string) (*model.SubmitState, error)
	Submit(
		ctx context.Context,
		userID string,
		sketchID string,
		request model.SubmitCommitRequest,
	) (*model.SubmitCommitResult, error)
}

type Service struct {
	repo Repository
}

const (
	submitStatusCommitted        = "committed"
	submitStatusDuplicate        = "duplicate"
	submitStatusStaleVersion     = "stale_version"
	submitStatusPermissionDenied = "permission_denied"
	submitStatusNotFound         = "not_found"
)

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) List(
	ctx context.Context,
	sketchID string,
	afterVersion int64,
	limit int,
) (*model.SketchOperationPage, error) {
	sketchID = strings.TrimSpace(sketchID)
	if sketchID == "" {
		return nil, errors.New("sketchID is required")
	}
	if afterVersion < 0 {
		return nil, errors.New("afterVersion must be greater than or equal to 0")
	}
	if limit < 1 {
		return nil, errors.New("limit must be greater than 0")
	}

	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, errors.New("authenticated user id is required")
	}

	return s.repo.List(ctx, userID, sketchID, afterVersion, limit)
}

func (s *Service) Submit(
	ctx context.Context,
	sketchID string,
	request *model.SubmitOperationRequest,
) (*model.SubmitOperationResponse, error) {
	sketchID = strings.TrimSpace(sketchID)
	if sketchID == "" {
		return nil, errors.New("sketchID is required")
	}
	if request == nil {
		return nil, errors.New("request is required")
	}
	request.ClientOpID = strings.TrimSpace(request.ClientOpID)
	if request.BaseVersion < 0 {
		return nil, errors.New("baseVersion must be greater than or equal to 0")
	}
	if request.ClientOpID == "" {
		return nil, errors.New("clientOpId is required")
	}
	if len(request.Op) == 0 {
		return nil, errors.New("op is required")
	}

	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, errors.New("authenticated user id is required")
	}

	opType, err := operationType(request.Op)
	if err != nil {
		return nil, err
	}

	state, err := s.repo.GetSubmitState(ctx, userID, sketchID)
	if err != nil {
		return nil, err
	}
	if state.Version != request.BaseVersion {
		result, err := s.repo.Submit(ctx, userID, sketchID, model.SubmitCommitRequest{
			BaseVersion:          request.BaseVersion,
			ClientOpID:           request.ClientOpID,
			OpType:               opType,
			Payload:              append(easyjson.RawMessage(nil), request.Op...),
			Patch:                easyjson.RawMessage(`{}`),
			GraphState:           state.GraphState,
			MaterializedGeometry: state.MaterializedGeometry,
			SolveStatus:          state.SolveStatus,
		})
		if err != nil {
			return nil, err
		}
		if result.Status == submitStatusDuplicate {
			return accepted(result, easyjson.RawMessage(`{}`), state.SolveStatus, nil), nil
		}
		return rejected(request.ClientOpID, state.Version, submitStatusStaleVersion, "baseVersion does not match current sketch version"), nil
	}

	graph, err := decodeGraphState(state.GraphState)
	if err != nil {
		return nil, err
	}
	patch, changedEntityIDs, err := applyOperation(graph, request.Op)
	if err != nil {
		return rejected(request.ClientOpID, state.Version, "invalid_operation", err.Error()), nil
	}

	graphState, err := json.Marshal(graph)
	if err != nil {
		return nil, fmt.Errorf("encode graph state: %w", err)
	}
	patchBody, err := json.Marshal(patch)
	if err != nil {
		return nil, fmt.Errorf("encode operation patch: %w", err)
	}

	result, err := s.repo.Submit(ctx, userID, sketchID, model.SubmitCommitRequest{
		BaseVersion:          request.BaseVersion,
		ClientOpID:           request.ClientOpID,
		OpType:               opType,
		Payload:              append(easyjson.RawMessage(nil), request.Op...),
		Patch:                patchBody,
		GraphState:           graphState,
		MaterializedGeometry: state.MaterializedGeometry,
		SolveStatus:          state.SolveStatus,
		ChangedEntityIDs:     changedEntityIDs,
	})
	if err != nil {
		return nil, err
	}

	switch result.Status {
	case submitStatusCommitted, submitStatusDuplicate:
		return accepted(result, patchBody, state.SolveStatus, changedEntityIDs), nil
	case submitStatusStaleVersion:
		return rejected(request.ClientOpID, result.CurrentVersion, submitStatusStaleVersion, "baseVersion does not match current sketch version"), nil
	case submitStatusPermissionDenied:
		return rejected(request.ClientOpID, result.CurrentVersion, submitStatusPermissionDenied, "user cannot edit sketch"), nil
	case submitStatusNotFound:
		return rejected(request.ClientOpID, result.CurrentVersion, submitStatusNotFound, "sketch not found"), nil
	default:
		return nil, fmt.Errorf("unknown submit status %q", result.Status)
	}
}

func accepted(
	result *model.SubmitCommitResult,
	patch easyjson.RawMessage,
	solveStatus easyjson.RawMessage,
	changedEntityIDs []string,
) *model.SubmitOperationResponse {
	return &model.SubmitOperationResponse{
		Accepted:         true,
		Duplicate:        result.Duplicate || result.Status == submitStatusDuplicate,
		OpID:             optionalString(result.OpID),
		Version:          optionalInt64(result.Version),
		CurrentVersion:   result.CurrentVersion,
		Patch:            patch,
		SolveStatus:      solveStatus,
		ChangedEntityIDs: changedEntityIDs,
	}
}

func operationType(raw easyjson.RawMessage) (string, error) {
	var envelope struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return "", fmt.Errorf("decode op: %w", err)
	}
	envelope.Type = strings.TrimSpace(envelope.Type)
	if envelope.Type == "" {
		return "", errors.New("op.type is required")
	}
	return envelope.Type, nil
}

type graphState struct {
	Entities    map[string]json.RawMessage `json:"entities"`
	Constraints map[string]json.RawMessage `json:"constraints"`
	Dimensions  map[string]json.RawMessage `json:"dimensions"`
	Groups      map[string]json.RawMessage `json:"groups"`
}

type sketchPatch struct {
	Entities             map[string]json.RawMessage `json:"entities,omitempty"`
	Constraints          map[string]json.RawMessage `json:"constraints,omitempty"`
	Dimensions           map[string]json.RawMessage `json:"dimensions,omitempty"`
	DeletedEntityIDs     []string                   `json:"deletedEntityIds,omitempty"`
	DeletedConstraintIDs []string                   `json:"deletedConstraintIds,omitempty"`
	DeletedDimensionIDs  []string                   `json:"deletedDimensionIds,omitempty"`
}

func decodeGraphState(raw easyjson.RawMessage) (*graphState, error) {
	graph := &graphState{}
	if len(raw) > 0 {
		if err := json.Unmarshal(raw, graph); err != nil {
			return nil, fmt.Errorf("decode graph state: %w", err)
		}
	}
	if graph.Entities == nil {
		graph.Entities = make(map[string]json.RawMessage)
	}
	if graph.Constraints == nil {
		graph.Constraints = make(map[string]json.RawMessage)
	}
	if graph.Dimensions == nil {
		graph.Dimensions = make(map[string]json.RawMessage)
	}
	if graph.Groups == nil {
		graph.Groups = make(map[string]json.RawMessage)
	}
	return graph, nil
}

func applyOperation(graph *graphState, raw easyjson.RawMessage) (*sketchPatch, []string, error) {
	opType, err := operationType(raw)
	if err != nil {
		return nil, nil, err
	}

	switch opType {
	case "create_point":
		return applyCreatePoint(graph, raw)
	case "create_line":
		return applyCreateLine(graph, raw)
	case "delete_entity":
		return applyDeleteEntity(graph, raw)
	default:
		return nil, nil, fmt.Errorf("operation type %q is not supported yet", opType)
	}
}

func applyCreatePoint(graph *graphState, raw easyjson.RawMessage) (*sketchPatch, []string, error) {
	var op struct {
		PointID string  `json:"pointId"`
		X       float64 `json:"x"`
		Y       float64 `json:"y"`
	}
	if err := json.Unmarshal(raw, &op); err != nil {
		return nil, nil, fmt.Errorf("decode create_point: %w", err)
	}
	op.PointID = strings.TrimSpace(op.PointID)
	if op.PointID == "" {
		return nil, nil, errors.New("pointId is required")
	}
	if _, exists := graph.Entities[op.PointID]; exists {
		return nil, nil, fmt.Errorf("entity %q already exists", op.PointID)
	}

	entity := mustJSON(map[string]any{"id": op.PointID, "type": "point", "x": op.X, "y": op.Y})
	graph.Entities[op.PointID] = entity
	return &sketchPatch{Entities: map[string]json.RawMessage{op.PointID: entity}}, []string{op.PointID}, nil
}

type pointRefOrNew struct {
	Kind    string   `json:"kind"`
	PointID string   `json:"pointId"`
	X       *float64 `json:"x"`
	Y       *float64 `json:"y"`
}

func applyCreateLine(graph *graphState, raw easyjson.RawMessage) (*sketchPatch, []string, error) {
	var op struct {
		EntityID string        `json:"entityId"`
		Start    pointRefOrNew `json:"start"`
		End      pointRefOrNew `json:"end"`
	}
	if err := json.Unmarshal(raw, &op); err != nil {
		return nil, nil, fmt.Errorf("decode create_line: %w", err)
	}
	op.EntityID = strings.TrimSpace(op.EntityID)
	if op.EntityID == "" {
		return nil, nil, errors.New("entityId is required")
	}
	if _, exists := graph.Entities[op.EntityID]; exists {
		return nil, nil, fmt.Errorf("entity %q already exists", op.EntityID)
	}

	patch := &sketchPatch{Entities: make(map[string]json.RawMessage)}
	changed := make([]string, 0, 3)
	startID, err := ensurePoint(graph, patch, op.Start)
	if err != nil {
		return nil, nil, fmt.Errorf("start: %w", err)
	}
	if _, ok := patch.Entities[startID]; ok {
		changed = append(changed, startID)
	}
	endID, err := ensurePoint(graph, patch, op.End)
	if err != nil {
		return nil, nil, fmt.Errorf("end: %w", err)
	}
	if _, ok := patch.Entities[endID]; ok {
		changed = append(changed, endID)
	}

	line := mustJSON(map[string]any{
		"id":           op.EntityID,
		"type":         "line",
		"startPointId": startID,
		"endPointId":   endID,
	})
	graph.Entities[op.EntityID] = line
	patch.Entities[op.EntityID] = line
	changed = append(changed, op.EntityID)
	return patch, changed, nil
}

func ensurePoint(graph *graphState, patch *sketchPatch, point pointRefOrNew) (string, error) {
	point.Kind = strings.TrimSpace(point.Kind)
	point.PointID = strings.TrimSpace(point.PointID)
	if point.PointID == "" {
		return "", errors.New("pointId is required")
	}

	switch point.Kind {
	case "existing_point":
		if _, exists := graph.Entities[point.PointID]; !exists {
			return "", fmt.Errorf("point %q does not exist", point.PointID)
		}
		return point.PointID, nil
	case "new_point":
		if point.X == nil || point.Y == nil {
			return "", errors.New("new_point requires x and y")
		}
		if _, exists := graph.Entities[point.PointID]; exists {
			return "", fmt.Errorf("entity %q already exists", point.PointID)
		}
		entity := mustJSON(map[string]any{"id": point.PointID, "type": "point", "x": *point.X, "y": *point.Y})
		graph.Entities[point.PointID] = entity
		patch.Entities[point.PointID] = entity
		return point.PointID, nil
	default:
		return "", errors.New("kind must be existing_point or new_point")
	}
}

func applyDeleteEntity(graph *graphState, raw easyjson.RawMessage) (*sketchPatch, []string, error) {
	var op struct {
		EntityID string `json:"entityId"`
	}
	if err := json.Unmarshal(raw, &op); err != nil {
		return nil, nil, fmt.Errorf("decode delete_entity: %w", err)
	}
	op.EntityID = strings.TrimSpace(op.EntityID)
	if op.EntityID == "" {
		return nil, nil, errors.New("entityId is required")
	}
	if _, exists := graph.Entities[op.EntityID]; !exists {
		return nil, nil, fmt.Errorf("entity %q does not exist", op.EntityID)
	}
	delete(graph.Entities, op.EntityID)
	return &sketchPatch{DeletedEntityIDs: []string{op.EntityID}}, []string{op.EntityID}, nil
}

func mustJSON(value any) json.RawMessage {
	body, err := json.Marshal(value)
	if err != nil {
		panic(err)
	}
	return body
}

func rejected(clientOpID string, currentVersion int64, reason string, message string) *model.SubmitOperationResponse {
	return &model.SubmitOperationResponse{
		Accepted:       false,
		CurrentVersion: currentVersion,
		Rejection: &model.OperationRejection{
			Reason:  reason,
			Message: message,
		},
	}
}

func optionalString(value string) *string {
	value = strings.TrimSpace(value)
	if value == "" {
		return nil
	}
	return &value
}

func optionalInt64(value int64) *int64 {
	if value == 0 {
		return nil
	}
	return &value
}
