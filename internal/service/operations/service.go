package operations

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"

	"github.com/dnonakolesax/cccad-locks/internal/auth"
	"github.com/dnonakolesax/cccad-locks/internal/model"
	solverv1 "github.com/dnonakolesax/cccad-locks/internal/proto/solver/v1"
	solverService "github.com/dnonakolesax/cccad-locks/internal/service/solver"
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

type SolverClient interface {
	Solve(ctx context.Context, req *solverv1.SolveRequest) (*solverv1.SolveResponse, error)
}

type Service struct {
	repo   Repository
	solver SolverClient
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

func NewServiceWithSolver(repo Repository, solver SolverClient) *Service {
	return &Service{repo: repo, solver: solver}
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
	materializedGeometry := state.MaterializedGeometry
	solveStatus := state.SolveStatus
	if operationRequiresSolve(opType) {
		if s.solver == nil {
			return nil, fmt.Errorf("solver client is required for operation type %q", opType)
		}
		solvePatch, solvedGeometry, solvedStatus, solvedEntityIDs, err := s.solveGraph(
			ctx,
			sketchID,
			request.BaseVersion+1,
			graph,
		)
		if err != nil {
			return rejected(request.ClientOpID, state.Version, "solver_failure", err.Error()), nil
		}
		mergePatch(patch, solvePatch)
		changedEntityIDs = mergeIDs(changedEntityIDs, solvedEntityIDs)
		materializedGeometry = solvedGeometry
		solveStatus = solvedStatus
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
		MaterializedGeometry: materializedGeometry,
		SolveStatus:          solveStatus,
		ChangedEntityIDs:     changedEntityIDs,
	})
	if err != nil {
		return nil, err
	}

	switch result.Status {
	case submitStatusCommitted, submitStatusDuplicate:
		return accepted(result, patchBody, solveStatus, changedEntityIDs), nil
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
	case "add_constraint":
		return applyAddConstraint(graph, raw)
	case "remove_constraint":
		return applyRemoveConstraint(graph, raw)
	default:
		return nil, nil, fmt.Errorf("operation type %q is not supported yet", opType)
	}
}

func operationRequiresSolve(opType string) bool {
	switch opType {
	case "add_constraint", "remove_constraint":
		return true
	default:
		return false
	}
}

func (s *Service) solveGraph(
	ctx context.Context,
	sketchID string,
	version int64,
	graph *graphState,
) (*sketchPatch, easyjson.RawMessage, easyjson.RawMessage, []string, error) {
	result, err := s.solver.Solve(ctx, &solverv1.SolveRequest{
		SketchId: sketchID,
		Version:  version,
		Model: solverService.BuildSketchModel(&model.SketchDocument{
			ID:          sketchID,
			Version:     version,
			Entities:    rawMessageMap(graph.Entities),
			Constraints: rawMessageMap(graph.Constraints),
			Dimensions:  rawMessageMap(graph.Dimensions),
			Groups:      rawMessageMap(graph.Groups),
		}),
		Options: defaultSolverOptions(),
	})
	if err != nil {
		return nil, nil, nil, nil, fmt.Errorf("solve sketch: %w", err)
	}
	if result == nil {
		return nil, nil, nil, nil, errors.New("solver returned nil solve response")
	}

	patchBody, err := solverService.SolutionPatch(result.GetSolution())
	if err != nil {
		return nil, nil, nil, nil, err
	}
	patch, entityIDs, err := applySolverPatch(graph, patchBody)
	if err != nil {
		return nil, nil, nil, nil, err
	}

	solveStatus, err := encodeSolveStatus(result.GetStatus(), result.GetDegreesOfFreedom(), result.GetDiagnostics())
	if err != nil {
		return nil, nil, nil, nil, err
	}

	return patch, patchBody, solveStatus, entityIDs, nil
}

func applySolverPatch(graph *graphState, raw easyjson.RawMessage) (*sketchPatch, []string, error) {
	var patch struct {
		Entities map[string]json.RawMessage `json:"entities"`
	}
	if err := json.Unmarshal(raw, &patch); err != nil {
		return nil, nil, fmt.Errorf("decode solver patch: %w", err)
	}

	result := &sketchPatch{Entities: make(map[string]json.RawMessage)}
	entityIDs := make([]string, 0, len(patch.Entities))
	for id, entity := range patch.Entities {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		graph.Entities[id] = append(json.RawMessage(nil), entity...)
		result.Entities[id] = append(json.RawMessage(nil), entity...)
		entityIDs = append(entityIDs, id)
	}
	if len(result.Entities) == 0 {
		result.Entities = nil
	}

	return result, entityIDs, nil
}

func encodeSolveStatus(
	status solverv1.SolveStatus,
	degreesOfFreedom int32,
	diagnostics []*solverv1.SolverDiagnostic,
) (easyjson.RawMessage, error) {
	body, err := json.Marshal(struct {
		model.SolveStatusInfo
		Diagnostics []model.SolverDiagnostic `json:"diagnostics"`
	}{
		SolveStatusInfo: solverService.SolveStatusInfo(status, degreesOfFreedom),
		Diagnostics:     solverService.SolverDiagnostics(diagnostics),
	})
	if err != nil {
		return nil, fmt.Errorf("encode solve status: %w", err)
	}

	return easyjson.RawMessage(body), nil
}

func defaultSolverOptions() *solverv1.SolverOptions {
	return &solverv1.SolverOptions{
		Tolerance:         1e-6,
		MaxIterations:     100,
		Deterministic:     true,
		ReturnDiagnostics: true,
	}
}

func mergePatch(base *sketchPatch, next *sketchPatch) {
	if base == nil || next == nil {
		return
	}
	if len(next.Entities) > 0 {
		if base.Entities == nil {
			base.Entities = make(map[string]json.RawMessage, len(next.Entities))
		}
		for id, entity := range next.Entities {
			base.Entities[id] = entity
		}
	}
	if len(next.Constraints) > 0 {
		if base.Constraints == nil {
			base.Constraints = make(map[string]json.RawMessage, len(next.Constraints))
		}
		for id, constraint := range next.Constraints {
			base.Constraints[id] = constraint
		}
	}
	base.Dimensions = mergeRawMessageMaps(base.Dimensions, next.Dimensions)
	base.DeletedEntityIDs = mergeIDs(base.DeletedEntityIDs, next.DeletedEntityIDs)
	base.DeletedConstraintIDs = mergeIDs(base.DeletedConstraintIDs, next.DeletedConstraintIDs)
	base.DeletedDimensionIDs = mergeIDs(base.DeletedDimensionIDs, next.DeletedDimensionIDs)
}

func mergeRawMessageMaps(
	base map[string]json.RawMessage,
	next map[string]json.RawMessage,
) map[string]json.RawMessage {
	if len(next) == 0 {
		return base
	}
	if base == nil {
		base = make(map[string]json.RawMessage, len(next))
	}
	for id, value := range next {
		base[id] = value
	}
	return base
}

func mergeIDs(base []string, next []string) []string {
	seen := make(map[string]struct{}, len(base)+len(next))
	result := make([]string, 0, len(base)+len(next))
	for _, id := range append(base, next...) {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		if _, ok := seen[id]; ok {
			continue
		}
		seen[id] = struct{}{}
		result = append(result, id)
	}
	return result
}

func rawMessageMap(values map[string]json.RawMessage) map[string]easyjson.RawMessage {
	result := make(map[string]easyjson.RawMessage, len(values))
	for key, value := range values {
		result[key] = easyjson.RawMessage(value)
	}
	return result
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

type constraintPayload struct {
	ID         string   `json:"id"`
	Type       string   `json:"type"`
	Refs       []string `json:"refs"`
	Status     string   `json:"status,omitempty"`
	PointAID   string   `json:"pointAId,omitempty"`
	PointBID   string   `json:"pointBId,omitempty"`
	LineID     string   `json:"lineId,omitempty"`
	LineAID    string   `json:"lineAId,omitempty"`
	LineBID    string   `json:"lineBId,omitempty"`
	EntityID   string   `json:"entityId,omitempty"`
	EntityAID  string   `json:"entityAId,omitempty"`
	EntityBID  string   `json:"entityBId,omitempty"`
	MidpointID string   `json:"midpointId,omitempty"`
	CircleAID  string   `json:"circleAId,omitempty"`
	CircleBID  string   `json:"circleBId,omitempty"`
	Branch     string   `json:"branch,omitempty"`
	Kind       string   `json:"kind,omitempty"`
}

func applyAddConstraint(graph *graphState, raw easyjson.RawMessage) (*sketchPatch, []string, error) {
	var op struct {
		ConstraintID string             `json:"constraintId"`
		Constraint   *constraintPayload `json:"constraint"`
	}
	if err := json.Unmarshal(raw, &op); err != nil {
		return nil, nil, fmt.Errorf("decode add_constraint: %w", err)
	}

	constraint := op.Constraint
	if constraint == nil {
		var inline constraintPayload
		if err := json.Unmarshal(raw, &inline); err != nil {
			return nil, nil, fmt.Errorf("decode inline add_constraint: %w", err)
		}
		constraint = &inline
	}
	if constraint.ID == "" {
		constraint.ID = op.ConstraintID
	}
	normalizeConstraint(constraint)
	if err := validateConstraint(graph, constraint); err != nil {
		return nil, nil, err
	}
	if constraint.Status == "" {
		constraint.Status = "active"
	}

	body, err := json.Marshal(constraint)
	if err != nil {
		return nil, nil, fmt.Errorf("encode constraint: %w", err)
	}
	graph.Constraints[constraint.ID] = body

	changed := constraintReferencedEntityIDs(constraint)
	return &sketchPatch{Constraints: map[string]json.RawMessage{constraint.ID: body}}, changed, nil
}

func applyRemoveConstraint(graph *graphState, raw easyjson.RawMessage) (*sketchPatch, []string, error) {
	var op struct {
		ConstraintID string `json:"constraintId"`
	}
	if err := json.Unmarshal(raw, &op); err != nil {
		return nil, nil, fmt.Errorf("decode remove_constraint: %w", err)
	}
	op.ConstraintID = strings.TrimSpace(op.ConstraintID)
	if op.ConstraintID == "" {
		return nil, nil, errors.New("constraintId is required")
	}

	body, exists := graph.Constraints[op.ConstraintID]
	if !exists {
		return nil, nil, fmt.Errorf("constraint %q does not exist", op.ConstraintID)
	}

	var constraint constraintPayload
	if err := json.Unmarshal(body, &constraint); err != nil {
		return nil, nil, fmt.Errorf("decode existing constraint %q: %w", op.ConstraintID, err)
	}
	normalizeConstraint(&constraint)
	changed := constraintReferencedEntityIDs(&constraint)

	delete(graph.Constraints, op.ConstraintID)
	return &sketchPatch{DeletedConstraintIDs: []string{op.ConstraintID}}, changed, nil
}

func normalizeConstraint(constraint *constraintPayload) {
	constraint.ID = strings.TrimSpace(constraint.ID)
	constraint.Type = strings.TrimSpace(constraint.Type)
	constraint.Status = strings.TrimSpace(constraint.Status)
	constraint.PointAID = strings.TrimSpace(constraint.PointAID)
	constraint.PointBID = strings.TrimSpace(constraint.PointBID)
	constraint.LineID = strings.TrimSpace(constraint.LineID)
	constraint.LineAID = strings.TrimSpace(constraint.LineAID)
	constraint.LineBID = strings.TrimSpace(constraint.LineBID)
	constraint.EntityID = strings.TrimSpace(constraint.EntityID)
	constraint.EntityAID = strings.TrimSpace(constraint.EntityAID)
	constraint.EntityBID = strings.TrimSpace(constraint.EntityBID)
	constraint.MidpointID = strings.TrimSpace(constraint.MidpointID)
	constraint.CircleAID = strings.TrimSpace(constraint.CircleAID)
	constraint.CircleBID = strings.TrimSpace(constraint.CircleBID)
	constraint.Branch = strings.TrimSpace(constraint.Branch)
	constraint.Kind = strings.TrimSpace(constraint.Kind)
	for i := range constraint.Refs {
		constraint.Refs[i] = strings.TrimSpace(constraint.Refs[i])
	}
	inferConstraintRefs(constraint)
}

func inferConstraintRefs(constraint *constraintPayload) {
	ref := func(index int) string {
		if index < 0 || index >= len(constraint.Refs) {
			return ""
		}
		return constraint.Refs[index]
	}

	switch constraint.Type {
	case "coincident":
		if constraint.PointAID == "" {
			constraint.PointAID = ref(0)
		}
		if constraint.PointBID == "" {
			constraint.PointBID = ref(1)
		}
	case "horizontal", "vertical":
		if constraint.LineID == "" {
			constraint.LineID = ref(0)
		}
	case "parallel", "perpendicular":
		if constraint.LineAID == "" {
			constraint.LineAID = ref(0)
		}
		if constraint.LineBID == "" {
			constraint.LineBID = ref(1)
		}
	case "tangent", "equal":
		if constraint.EntityAID == "" {
			constraint.EntityAID = ref(0)
		}
		if constraint.EntityBID == "" {
			constraint.EntityBID = ref(1)
		}
	case "fixed":
		if constraint.EntityID == "" {
			constraint.EntityID = ref(0)
		}
	case "midpoint":
		if constraint.MidpointID == "" {
			constraint.MidpointID = ref(0)
		}
		if constraint.PointAID == "" {
			constraint.PointAID = ref(1)
		}
		if constraint.PointBID == "" {
			constraint.PointBID = ref(2)
		}
	case "concentric":
		if constraint.CircleAID == "" {
			constraint.CircleAID = ref(0)
		}
		if constraint.CircleBID == "" {
			constraint.CircleBID = ref(1)
		}
	}
}

func validateConstraint(graph *graphState, constraint *constraintPayload) error {
	if constraint.ID == "" {
		return errors.New("constraint.id or constraintId is required")
	}
	if _, exists := graph.Constraints[constraint.ID]; exists {
		return fmt.Errorf("constraint %q already exists", constraint.ID)
	}
	if constraint.Type == "" {
		return errors.New("constraint.type is required")
	}

	refs := constraintReferencedEntityIDs(constraint)
	if len(refs) == 0 {
		return errors.New("constraint must reference at least one entity")
	}
	for _, ref := range refs {
		if ref == "" {
			return errors.New("constraint references must not be empty")
		}
		if _, exists := graph.Entities[ref]; !exists {
			return fmt.Errorf("constraint reference %q does not exist", ref)
		}
	}

	switch constraint.Type {
	case "coincident", "horizontal", "vertical", "parallel", "perpendicular", "tangent", "equal", "fixed", "midpoint", "concentric":
		return nil
	default:
		return fmt.Errorf("unsupported constraint type %q", constraint.Type)
	}
}

func constraintReferencedEntityIDs(constraint *constraintPayload) []string {
	seen := make(map[string]struct{})
	refs := make([]string, 0, len(constraint.Refs)+8)
	add := func(value string) {
		value = strings.TrimSpace(value)
		if value == "" {
			return
		}
		if _, ok := seen[value]; ok {
			return
		}
		seen[value] = struct{}{}
		refs = append(refs, value)
	}

	for _, ref := range constraint.Refs {
		add(ref)
	}
	add(constraint.PointAID)
	add(constraint.PointBID)
	add(constraint.LineID)
	add(constraint.LineAID)
	add(constraint.LineBID)
	add(constraint.EntityID)
	add(constraint.EntityAID)
	add(constraint.EntityBID)
	add(constraint.MidpointID)
	add(constraint.CircleAID)
	add(constraint.CircleBID)

	return refs
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
