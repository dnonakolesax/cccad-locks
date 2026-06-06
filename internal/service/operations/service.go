package operations

import (
	"context"
	"crypto/sha1"
	"encoding/hex"
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
	ApplyIntent(ctx context.Context, req *solverv1.ApplyIntentRequest) (*solverv1.ApplyIntentResponse, error)
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
			Profiles:             state.Profiles,
		})
		if err != nil {
			return nil, err
		}
		if result.Status == submitStatusDuplicate {
			return accepted(result, easyjson.RawMessage(`{}`), state.SolveStatus, affectedIDs{}), nil
		}
		return rejected(request.ClientOpID, state.Version, submitStatusStaleVersion, "baseVersion does not match current sketch version"), nil
	}

	graph, err := decodeGraphState(state.GraphState)
	if err != nil {
		return nil, err
	}
	patch, affected, err := applyOperation(graph, request.Op)
	if err != nil {
		return rejected(request.ClientOpID, state.Version, "invalid_operation", err.Error()), nil
	}
	materializedGeometry := state.MaterializedGeometry
	solveStatus := state.SolveStatus
	profiles := state.Profiles
	if operationRequiresSolve(opType) {
		if s.solver == nil {
			return nil, fmt.Errorf("solver client is required for operation type %q", opType)
		}
		var solvePatch *sketchPatch
		var solvedGeometry easyjson.RawMessage
		var solvedStatus easyjson.RawMessage
		var solvedProfiles easyjson.RawMessage
		var solvedAffected affectedIDs
		var err error
		if operationRequiresSolverIntent(opType) {
			solvePatch, solvedGeometry, solvedStatus, solvedProfiles, solvedAffected, err = s.applySolverIntent(
				ctx,
				sketchID,
				request.BaseVersion+1,
				graph,
				request.Op,
			)
		} else {
			solvePatch, solvedGeometry, solvedStatus, solvedProfiles, solvedAffected, err = s.solveGraph(
				ctx,
				sketchID,
				request.BaseVersion+1,
				graph,
			)
		}
		if err != nil {
			return rejected(request.ClientOpID, state.Version, "solver_failure", err.Error()), nil
		}
		mergePatch(patch, solvePatch)
		affected.EntityIDs = mergeIDs(affected.EntityIDs, solvedAffected.EntityIDs)
		affected.ConstraintIDs = mergeIDs(affected.ConstraintIDs, solvedAffected.ConstraintIDs)
		affected.DimensionIDs = mergeIDs(affected.DimensionIDs, solvedAffected.DimensionIDs)
		affected.ComponentIDs = mergeIDs(affected.ComponentIDs, solvedAffected.ComponentIDs)
		materializedGeometry = solvedGeometry
		solveStatus = solvedStatus
		profiles = solvedProfiles
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
		Profiles:             profiles,
		ChangedEntityIDs:     affected.EntityIDs,
	})
	if err != nil {
		return nil, err
	}

	switch result.Status {
	case submitStatusCommitted, submitStatusDuplicate:
		return accepted(result, patchBody, solveStatus, affected), nil
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
	affected affectedIDs,
) *model.SubmitOperationResponse {
	return &model.SubmitOperationResponse{
		Accepted:             true,
		Duplicate:            result.Duplicate || result.Status == submitStatusDuplicate,
		OpID:                 optionalString(result.OpID),
		Version:              optionalInt64(result.Version),
		CurrentVersion:       result.CurrentVersion,
		Patch:                patch,
		SolveStatus:          solveStatus,
		ChangedEntityIDs:     affected.EntityIDs,
		ChangedConstraintIDs: affected.ConstraintIDs,
		ChangedDimensionIDs:  affected.DimensionIDs,
		ChangedComponentIDs:  affected.ComponentIDs,
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
	MaterializedGeometry map[string]json.RawMessage `json:"materializedGeometry,omitempty"`
	Profiles             []json.RawMessage          `json:"profiles,omitempty"`
	DeletedEntityIDs     []string                   `json:"deletedEntityIds,omitempty"`
	DeletedConstraintIDs []string                   `json:"deletedConstraintIds,omitempty"`
	DeletedDimensionIDs  []string                   `json:"deletedDimensionIds,omitempty"`
}

type affectedIDs struct {
	EntityIDs     []string
	ConstraintIDs []string
	DimensionIDs  []string
	ComponentIDs  []string
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
	repairDefaultConstructionEntities(graph)
	return graph, nil
}

func repairDefaultConstructionEntities(graph *graphState) {
	if graph == nil || graph.Entities == nil {
		return
	}
	if _, ok := graph.Entities["x-axis"]; ok {
		ensureDefaultConstructionEntity(graph, "x-axis-start", map[string]any{
			"id":             "x-axis-start",
			"type":           "point",
			"x":              -9999,
			"y":              0,
			"fixed":          true,
			"isConstruction": true,
		})
		ensureDefaultConstructionEntity(graph, "x-axis-end", map[string]any{
			"id":             "x-axis-end",
			"type":           "point",
			"x":              9999,
			"y":              0,
			"fixed":          true,
			"isConstruction": true,
		})
	}
	if _, ok := graph.Entities["y-axis"]; ok {
		ensureDefaultConstructionEntity(graph, "y-axis-start", map[string]any{
			"id":             "y-axis-start",
			"type":           "point",
			"x":              0,
			"y":              -9999,
			"fixed":          true,
			"isConstruction": true,
		})
		ensureDefaultConstructionEntity(graph, "y-axis-end", map[string]any{
			"id":             "y-axis-end",
			"type":           "point",
			"x":              0,
			"y":              9999,
			"fixed":          true,
			"isConstruction": true,
		})
	}
}

func ensureDefaultConstructionEntity(graph *graphState, id string, entity map[string]any) {
	if _, ok := graph.Entities[id]; ok {
		return
	}
	graph.Entities[id] = mustJSON(entity)
}

func applyOperation(graph *graphState, raw easyjson.RawMessage) (*sketchPatch, affectedIDs, error) {
	opType, err := operationType(raw)
	if err != nil {
		return nil, affectedIDs{}, err
	}

	switch opType {
	case "create_point":
		return applyCreatePoint(graph, raw)
	case "create_line":
		return applyCreateLine(graph, raw)
	case "create_circle":
		return applyCreateCircle(graph, raw)
	case "create_arc":
		return applyCreateArc(graph, raw)
	case "create_rectangle":
		return applyCreateRectangle(graph, raw)
	case "create_polyline":
		return applyCreatePolyline(graph, raw)
	case "ApplyFillet":
		return applyFillet(graph, raw)
	case "UpdateFillet":
		return applyUpdateFillet(graph, raw)
	case "ApplyChamfer":
		return applyChamfer(graph, raw)
	case "UpdateChamfer":
		return applyUpdateChamfer(graph, raw)
	case "set_entity_construction":
		return applySetEntityConstruction(graph, raw)
	case "split_entity":
		return applyEntityIntent(graph, raw, "split_entity")
	case "break_entity_at_point":
		return applyEntityIntent(graph, raw, "break_entity_at_point")
	case "trim_entity":
		return applyEntityIntent(graph, raw, "trim_entity")
	case "extend_entity":
		return applyEntityIntent(graph, raw, "extend_entity")
	case "mirror_entities":
		return applyMirrorEntities(graph, raw)
	case "linear_pattern":
		return applyLinearPattern(graph, raw)
	case "circular_pattern":
		return applyCircularPattern(graph, raw)
	case "move_point":
		return applyMovePoint(graph, raw)
	case "delete_entity":
		return applyDeleteEntity(graph, raw)
	case "add_constraint":
		return applyAddConstraint(graph, raw)
	case "remove_constraint":
		return applyRemoveConstraint(graph, raw)
	case "add_dimension":
		return applyAddDimension(graph, raw)
	case "set_dimension_value":
		return applySetDimensionValue(graph, raw)
	case "remove_dimension":
		return applyRemoveDimension(graph, raw)
	default:
		return nil, affectedIDs{}, fmt.Errorf("operation type %q is not supported yet", opType)
	}
}

func operationRequiresSolve(opType string) bool {
	switch opType {
	case "create_point", "create_line", "create_circle", "create_arc", "create_rectangle", "create_polyline",
		"ApplyFillet", "UpdateFillet", "ApplyChamfer", "UpdateChamfer",
		"split_entity", "break_entity_at_point", "trim_entity", "extend_entity",
		"mirror_entities", "linear_pattern", "circular_pattern",
		"move_point", "delete_entity", "add_constraint", "remove_constraint",
		"add_dimension", "set_dimension_value", "remove_dimension":
		return true
	default:
		return false
	}
}

func operationRequiresSolverIntent(opType string) bool {
	switch opType {
	case "ApplyFillet", "UpdateFillet", "ApplyChamfer", "UpdateChamfer",
		"split_entity", "break_entity_at_point", "trim_entity", "extend_entity",
		"mirror_entities", "linear_pattern", "circular_pattern":
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
) (*sketchPatch, easyjson.RawMessage, easyjson.RawMessage, easyjson.RawMessage, affectedIDs, error) {
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
		return nil, nil, nil, nil, affectedIDs{}, fmt.Errorf("solve sketch: %w", err)
	}
	if result == nil {
		return nil, nil, nil, nil, affectedIDs{}, errors.New("solver returned nil solve response")
	}

	patchBody, err := solverService.SolutionPatch(result.GetSolution())
	if err != nil {
		return nil, nil, nil, nil, affectedIDs{}, err
	}
	patch, entityIDs, err := applySolverPatch(graph, patchBody)
	if err != nil {
		return nil, nil, nil, nil, affectedIDs{}, err
	}

	solveStatus, err := encodeSolveStatus(result.GetStatus(), result.GetDegreesOfFreedom(), result.GetDiagnostics())
	if err != nil {
		return nil, nil, nil, nil, affectedIDs{}, err
	}
	profiles, err := profilesFromSolverPatch(patchBody)
	if err != nil {
		return nil, nil, nil, nil, affectedIDs{}, err
	}

	affected := affectedFromDiagnostics(result.GetDiagnostics())
	affected.EntityIDs = mergeIDs(affected.EntityIDs, entityIDs)
	return patch, patchBody, solveStatus, profiles, affected, nil
}

func (s *Service) applySolverIntent(
	ctx context.Context,
	sketchID string,
	version int64,
	graph *graphState,
	raw easyjson.RawMessage,
) (*sketchPatch, easyjson.RawMessage, easyjson.RawMessage, easyjson.RawMessage, affectedIDs, error) {
	intent, err := solverService.UserIntent(raw)
	if err != nil {
		return nil, nil, nil, nil, affectedIDs{}, err
	}
	result, err := s.solver.ApplyIntent(ctx, &solverv1.ApplyIntentRequest{
		Model: solverService.BuildSketchModel(&model.SketchDocument{
			ID:          sketchID,
			Version:     version,
			Entities:    rawMessageMap(graph.Entities),
			Constraints: rawMessageMap(graph.Constraints),
			Dimensions:  rawMessageMap(graph.Dimensions),
			Groups:      rawMessageMap(graph.Groups),
		}),
		Intent:  intent,
		Options: defaultSolverOptions(),
	})
	if err != nil {
		return nil, nil, nil, nil, affectedIDs{}, fmt.Errorf("apply solver intent: %w", err)
	}
	if result == nil {
		return nil, nil, nil, nil, affectedIDs{}, errors.New("solver returned nil apply intent response")
	}

	patchBody, err := solverService.SolutionPatch(result.GetSolution())
	if err != nil {
		return nil, nil, nil, nil, affectedIDs{}, err
	}
	patch, entityIDs, err := applySolverPatch(graph, patchBody)
	if err != nil {
		return nil, nil, nil, nil, affectedIDs{}, err
	}

	solveStatus, err := encodeSolveStatus(result.GetStatus(), result.GetDegreesOfFreedom(), result.GetDiagnostics())
	if err != nil {
		return nil, nil, nil, nil, affectedIDs{}, err
	}
	profiles, err := profilesFromSolverPatch(patchBody)
	if err != nil {
		return nil, nil, nil, nil, affectedIDs{}, err
	}

	affected := affectedFromDiagnostics(result.GetDiagnostics())
	affected.EntityIDs = mergeIDs(affected.EntityIDs, result.GetAffectedEntityIds())
	affected.EntityIDs = mergeIDs(affected.EntityIDs, entityIDs)
	return patch, patchBody, solveStatus, profiles, affected, nil
}

func applySolverPatch(graph *graphState, raw easyjson.RawMessage) (*sketchPatch, []string, error) {
	var patch struct {
		Entities map[string]json.RawMessage `json:"entities"`
		Profiles []json.RawMessage          `json:"profiles"`
	}
	if err := json.Unmarshal(raw, &patch); err != nil {
		return nil, nil, fmt.Errorf("decode solver patch: %w", err)
	}

	result := &sketchPatch{Entities: make(map[string]json.RawMessage)}
	result.MaterializedGeometry = make(map[string]json.RawMessage)
	entityIDs := make([]string, 0, len(patch.Entities))
	for id, entity := range patch.Entities {
		id = strings.TrimSpace(id)
		if id == "" {
			continue
		}
		graph.Entities[id] = append(json.RawMessage(nil), entity...)
		result.Entities[id] = append(json.RawMessage(nil), entity...)
		result.MaterializedGeometry[id] = append(json.RawMessage(nil), entity...)
		entityIDs = append(entityIDs, id)
	}
	if len(result.Entities) == 0 {
		result.Entities = nil
	}
	if len(result.MaterializedGeometry) == 0 {
		result.MaterializedGeometry = nil
	}
	if len(patch.Profiles) > 0 {
		result.Profiles = make([]json.RawMessage, 0, len(patch.Profiles))
		for _, profile := range patch.Profiles {
			result.Profiles = append(result.Profiles, append(json.RawMessage(nil), profile...))
		}
	}

	return result, entityIDs, nil
}

func profilesFromSolverPatch(raw easyjson.RawMessage) (easyjson.RawMessage, error) {
	var patch struct {
		Profiles []json.RawMessage `json:"profiles"`
	}
	if err := json.Unmarshal(raw, &patch); err != nil {
		return nil, fmt.Errorf("decode solver profiles: %w", err)
	}
	if patch.Profiles == nil {
		return easyjson.RawMessage(`[]`), nil
	}

	body, err := json.Marshal(patch.Profiles)
	if err != nil {
		return nil, fmt.Errorf("encode solver profiles: %w", err)
	}

	return easyjson.RawMessage(body), nil
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

func affectedFromDiagnostics(diagnostics []*solverv1.SolverDiagnostic) affectedIDs {
	var affected affectedIDs
	for _, diagnostic := range diagnostics {
		affected.EntityIDs = mergeIDs(affected.EntityIDs, diagnostic.GetEntityIds())
		affected.ConstraintIDs = mergeIDs(affected.ConstraintIDs, diagnostic.GetConstraintIds())
		affected.DimensionIDs = mergeIDs(affected.DimensionIDs, diagnostic.GetDimensionIds())
	}
	return affected
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
	base.MaterializedGeometry = mergeRawMessageMaps(base.MaterializedGeometry, next.MaterializedGeometry)
	if len(next.Profiles) > 0 {
		base.Profiles = append([]json.RawMessage(nil), next.Profiles...)
	}
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

func applyCreatePoint(graph *graphState, raw easyjson.RawMessage) (*sketchPatch, affectedIDs, error) {
	var op struct {
		PointID string  `json:"pointId"`
		X       float64 `json:"x"`
		Y       float64 `json:"y"`
	}
	if err := json.Unmarshal(raw, &op); err != nil {
		return nil, affectedIDs{}, fmt.Errorf("decode create_point: %w", err)
	}
	op.PointID = strings.TrimSpace(op.PointID)
	if op.PointID == "" {
		return nil, affectedIDs{}, errors.New("pointId is required")
	}
	if _, exists := graph.Entities[op.PointID]; exists {
		return nil, affectedIDs{}, fmt.Errorf("entity %q already exists", op.PointID)
	}

	entity := mustJSON(map[string]any{"id": op.PointID, "type": "point", "x": op.X, "y": op.Y})
	graph.Entities[op.PointID] = entity
	return &sketchPatch{Entities: map[string]json.RawMessage{op.PointID: entity}}, affectedIDs{EntityIDs: []string{op.PointID}}, nil
}

type pointRefOrNew struct {
	Kind      string   `json:"kind"`
	PointID   string   `json:"pointId"`
	X         *float64 `json:"x"`
	Y         *float64 `json:"y"`
	DefaultID string   `json:"-"`
}

func applyCreateLine(graph *graphState, raw easyjson.RawMessage) (*sketchPatch, affectedIDs, error) {
	var op struct {
		EntityID       string        `json:"entityId"`
		Start          pointRefOrNew `json:"start"`
		End            pointRefOrNew `json:"end"`
		IsConstruction bool          `json:"isConstruction"`
	}
	if err := json.Unmarshal(raw, &op); err != nil {
		return nil, affectedIDs{}, fmt.Errorf("decode create_line: %w", err)
	}
	if op.EntityID == "" {
		op.EntityID = generatedID(raw, "line")
	}
	op.EntityID = strings.TrimSpace(op.EntityID)
	if op.EntityID == "" {
		return nil, affectedIDs{}, errors.New("entityId is required")
	}
	if _, exists := graph.Entities[op.EntityID]; exists {
		return nil, affectedIDs{}, fmt.Errorf("entity %q already exists", op.EntityID)
	}

	patch := &sketchPatch{Entities: make(map[string]json.RawMessage)}
	changed := make([]string, 0, 3)
	op.Start.DefaultID = generatedID(raw, "start")
	startID, err := ensurePoint(graph, patch, op.Start)
	if err != nil {
		return nil, affectedIDs{}, fmt.Errorf("start: %w", err)
	}
	if _, ok := patch.Entities[startID]; ok {
		changed = append(changed, startID)
	}
	op.End.DefaultID = generatedID(raw, "end")
	endID, err := ensurePoint(graph, patch, op.End)
	if err != nil {
		return nil, affectedIDs{}, fmt.Errorf("end: %w", err)
	}
	if _, ok := patch.Entities[endID]; ok {
		changed = append(changed, endID)
	}

	line := mustJSON(map[string]any{
		"id":             op.EntityID,
		"type":           "line",
		"startPointId":   startID,
		"endPointId":     endID,
		"isConstruction": op.IsConstruction,
	})
	graph.Entities[op.EntityID] = line
	patch.Entities[op.EntityID] = line
	changed = append(changed, op.EntityID)
	return patch, affectedIDs{EntityIDs: changed}, nil
}

func applyCreateCircle(graph *graphState, raw easyjson.RawMessage) (*sketchPatch, affectedIDs, error) {
	var op struct {
		EntityID       string        `json:"entityId"`
		Center         pointRefOrNew `json:"center"`
		Radius         float64       `json:"radius"`
		IsConstruction bool          `json:"isConstruction"`
	}
	if err := json.Unmarshal(raw, &op); err != nil {
		return nil, affectedIDs{}, fmt.Errorf("decode create_circle: %w", err)
	}
	if op.EntityID == "" {
		op.EntityID = generatedID(raw, "circle")
	}
	op.EntityID = strings.TrimSpace(op.EntityID)
	if op.EntityID == "" {
		return nil, affectedIDs{}, errors.New("entityId is required")
	}
	if op.Radius <= 0 {
		return nil, affectedIDs{}, errors.New("radius must be greater than 0")
	}
	if _, exists := graph.Entities[op.EntityID]; exists {
		return nil, affectedIDs{}, fmt.Errorf("entity %q already exists", op.EntityID)
	}

	patch := &sketchPatch{Entities: make(map[string]json.RawMessage)}
	changed := make([]string, 0, 2)
	op.Center.DefaultID = generatedID(raw, "center")
	centerID, err := ensurePoint(graph, patch, op.Center)
	if err != nil {
		return nil, affectedIDs{}, fmt.Errorf("center: %w", err)
	}
	if _, ok := patch.Entities[centerID]; ok {
		changed = append(changed, centerID)
	}

	circle := mustJSON(map[string]any{
		"id":             op.EntityID,
		"type":           "circle",
		"centerPointId":  centerID,
		"radius":         op.Radius,
		"isConstruction": op.IsConstruction,
	})
	graph.Entities[op.EntityID] = circle
	patch.Entities[op.EntityID] = circle
	changed = append(changed, op.EntityID)
	return patch, affectedIDs{EntityIDs: changed}, nil
}

func applyCreateArc(graph *graphState, raw easyjson.RawMessage) (*sketchPatch, affectedIDs, error) {
	var op struct {
		EntityID       string        `json:"entityId"`
		Center         pointRefOrNew `json:"center"`
		Start          pointRefOrNew `json:"start"`
		End            pointRefOrNew `json:"end"`
		Clockwise      bool          `json:"clockwise"`
		Branch         string        `json:"branch"`
		IsConstruction bool          `json:"isConstruction"`
	}
	if err := json.Unmarshal(raw, &op); err != nil {
		return nil, affectedIDs{}, fmt.Errorf("decode create_arc: %w", err)
	}
	if op.EntityID == "" {
		op.EntityID = generatedID(raw, "arc")
	}
	op.EntityID = strings.TrimSpace(op.EntityID)
	if op.EntityID == "" {
		return nil, affectedIDs{}, errors.New("entityId is required")
	}
	if _, exists := graph.Entities[op.EntityID]; exists {
		return nil, affectedIDs{}, fmt.Errorf("entity %q already exists", op.EntityID)
	}

	patch := &sketchPatch{Entities: make(map[string]json.RawMessage)}
	changed := make([]string, 0, 4)
	op.Center.DefaultID = generatedID(raw, "center")
	centerID, err := ensurePoint(graph, patch, op.Center)
	if err != nil {
		return nil, affectedIDs{}, fmt.Errorf("center: %w", err)
	}
	if _, ok := patch.Entities[centerID]; ok {
		changed = append(changed, centerID)
	}
	op.Start.DefaultID = generatedID(raw, "start")
	startID, err := ensurePoint(graph, patch, op.Start)
	if err != nil {
		return nil, affectedIDs{}, fmt.Errorf("start: %w", err)
	}
	if _, ok := patch.Entities[startID]; ok {
		changed = append(changed, startID)
	}
	op.End.DefaultID = generatedID(raw, "end")
	endID, err := ensurePoint(graph, patch, op.End)
	if err != nil {
		return nil, affectedIDs{}, fmt.Errorf("end: %w", err)
	}
	if _, ok := patch.Entities[endID]; ok {
		changed = append(changed, endID)
	}
	op.Branch = strings.TrimSpace(op.Branch)
	if op.Branch == "" {
		op.Branch = "minor"
	}

	arc := mustJSON(map[string]any{
		"id":             op.EntityID,
		"type":           "arc",
		"centerPointId":  centerID,
		"startPointId":   startID,
		"endPointId":     endID,
		"clockwise":      op.Clockwise,
		"branch":         op.Branch,
		"isConstruction": op.IsConstruction,
	})
	graph.Entities[op.EntityID] = arc
	patch.Entities[op.EntityID] = arc
	changed = append(changed, op.EntityID)
	return patch, affectedIDs{EntityIDs: changed}, nil
}

type vec2Op struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

func applyCreateRectangle(graph *graphState, raw easyjson.RawMessage) (*sketchPatch, affectedIDs, error) {
	var op struct {
		CornerA        vec2Op `json:"cornerA"`
		CornerB        vec2Op `json:"cornerB"`
		IsConstruction bool   `json:"isConstruction"`
	}
	if err := json.Unmarshal(raw, &op); err != nil {
		return nil, affectedIDs{}, fmt.Errorf("decode create_rectangle: %w", err)
	}

	points := []vec2Op{
		op.CornerA,
		{X: op.CornerB.X, Y: op.CornerA.Y},
		op.CornerB,
		{X: op.CornerA.X, Y: op.CornerB.Y},
	}
	return createPolylineEntities(graph, raw, points, true, "rectangle", op.IsConstruction)
}

func applyCreatePolyline(graph *graphState, raw easyjson.RawMessage) (*sketchPatch, affectedIDs, error) {
	var op struct {
		Points         []vec2Op `json:"points"`
		Closed         bool     `json:"closed"`
		IsConstruction bool     `json:"isConstruction"`
	}
	if err := json.Unmarshal(raw, &op); err != nil {
		return nil, affectedIDs{}, fmt.Errorf("decode create_polyline: %w", err)
	}
	if len(op.Points) < 2 {
		return nil, affectedIDs{}, errors.New("points must contain at least 2 items")
	}
	return createPolylineEntities(graph, raw, op.Points, op.Closed, "polyline", op.IsConstruction)
}

func createPolylineEntities(
	graph *graphState,
	raw easyjson.RawMessage,
	points []vec2Op,
	closed bool,
	prefix string,
	isConstruction bool,
) (*sketchPatch, affectedIDs, error) {
	patch := &sketchPatch{Entities: make(map[string]json.RawMessage)}
	changed := make([]string, 0, len(points)*2)
	pointIDs := make([]string, 0, len(points))
	for i, point := range points {
		id := generatedID(raw, fmt.Sprintf("%s-point-%d", prefix, i))
		if _, exists := graph.Entities[id]; exists {
			return nil, affectedIDs{}, fmt.Errorf("entity %q already exists", id)
		}
		entity := mustJSON(map[string]any{"id": id, "type": "point", "x": point.X, "y": point.Y})
		graph.Entities[id] = entity
		patch.Entities[id] = entity
		pointIDs = append(pointIDs, id)
		changed = append(changed, id)
	}

	segmentCount := len(pointIDs) - 1
	if closed {
		segmentCount = len(pointIDs)
	}
	for i := 0; i < segmentCount; i++ {
		id := generatedID(raw, fmt.Sprintf("%s-line-%d", prefix, i))
		if _, exists := graph.Entities[id]; exists {
			return nil, affectedIDs{}, fmt.Errorf("entity %q already exists", id)
		}
		endIndex := i + 1
		if endIndex == len(pointIDs) {
			endIndex = 0
		}
		line := mustJSON(map[string]any{
			"id":             id,
			"type":           "line",
			"startPointId":   pointIDs[i],
			"endPointId":     pointIDs[endIndex],
			"isConstruction": isConstruction,
		})
		graph.Entities[id] = line
		patch.Entities[id] = line
		changed = append(changed, id)
	}

	return patch, affectedIDs{EntityIDs: changed}, nil
}

func applyMovePoint(graph *graphState, raw easyjson.RawMessage) (*sketchPatch, affectedIDs, error) {
	var op struct {
		PointID string `json:"pointId"`
		Target  vec2Op `json:"target"`
	}
	if err := json.Unmarshal(raw, &op); err != nil {
		return nil, affectedIDs{}, fmt.Errorf("decode move_point: %w", err)
	}
	op.PointID = strings.TrimSpace(op.PointID)
	if op.PointID == "" {
		return nil, affectedIDs{}, errors.New("pointId is required")
	}

	var entity struct {
		ID   string `json:"id"`
		Type string `json:"type"`
	}
	body, exists := graph.Entities[op.PointID]
	if !exists {
		return nil, affectedIDs{}, fmt.Errorf("point %q does not exist", op.PointID)
	}
	if err := json.Unmarshal(body, &entity); err != nil {
		return nil, affectedIDs{}, fmt.Errorf("decode point %q: %w", op.PointID, err)
	}
	if entity.Type != "point" {
		return nil, affectedIDs{}, fmt.Errorf("entity %q is not a point", op.PointID)
	}

	point := mustJSON(map[string]any{"id": op.PointID, "type": "point", "x": op.Target.X, "y": op.Target.Y})
	graph.Entities[op.PointID] = point
	return &sketchPatch{Entities: map[string]json.RawMessage{op.PointID: point}}, affectedIDs{EntityIDs: []string{op.PointID}}, nil
}

func applyFillet(graph *graphState, raw easyjson.RawMessage) (*sketchPatch, affectedIDs, error) {
	var op struct {
		FeatureID       string  `json:"featureId"`
		Line1ID         string  `json:"line1Id"`
		Line2ID         string  `json:"line2Id"`
		CornerPointID   string  `json:"cornerPointId"`
		CreatedPoint1ID string  `json:"createdPoint1Id"`
		CreatedPoint2ID string  `json:"createdPoint2Id"`
		CreatedArcID    string  `json:"createdArcId"`
		Radius          float64 `json:"radius"`
		Trim            bool    `json:"trim"`
		Clockwise       bool    `json:"clockwise"`
	}
	if err := json.Unmarshal(raw, &op); err != nil {
		return nil, affectedIDs{}, fmt.Errorf("decode ApplyFillet: %w", err)
	}
	if op.FeatureID == "" {
		op.FeatureID = generatedID(raw, "fillet")
	}
	op.FeatureID = strings.TrimSpace(op.FeatureID)
	op.Line1ID = strings.TrimSpace(op.Line1ID)
	op.Line2ID = strings.TrimSpace(op.Line2ID)
	op.CornerPointID = strings.TrimSpace(op.CornerPointID)
	op.CreatedPoint1ID = strings.TrimSpace(op.CreatedPoint1ID)
	op.CreatedPoint2ID = strings.TrimSpace(op.CreatedPoint2ID)
	op.CreatedArcID = strings.TrimSpace(op.CreatedArcID)
	if op.FeatureID == "" {
		return nil, affectedIDs{}, errors.New("featureId is required")
	}
	if _, exists := graph.Entities[op.FeatureID]; exists {
		return nil, affectedIDs{}, fmt.Errorf("entity %q already exists", op.FeatureID)
	}
	if op.Radius <= 0 {
		return nil, affectedIDs{}, errors.New("radius must be greater than 0")
	}
	if err := requireLineEntity(graph, op.Line1ID, "line1Id"); err != nil {
		return nil, affectedIDs{}, err
	}
	if err := requireLineEntity(graph, op.Line2ID, "line2Id"); err != nil {
		return nil, affectedIDs{}, err
	}
	if op.Line1ID == op.Line2ID {
		return nil, affectedIDs{}, errors.New("line1Id and line2Id must be different")
	}
	if op.CornerPointID != "" {
		if err := requirePointEntity(graph, op.CornerPointID, "cornerPointId"); err != nil {
			return nil, affectedIDs{}, err
		}
	}
	if op.CreatedPoint1ID == "" {
		op.CreatedPoint1ID = generatedID(raw, "fillet-point-1")
	}
	if op.CreatedPoint2ID == "" {
		op.CreatedPoint2ID = generatedID(raw, "fillet-point-2")
	}
	if op.CreatedArcID == "" {
		op.CreatedArcID = generatedID(raw, "fillet-arc")
	}
	for _, id := range []string{op.CreatedPoint1ID, op.CreatedPoint2ID, op.CreatedArcID} {
		if _, exists := graph.Entities[id]; exists {
			return nil, affectedIDs{}, fmt.Errorf("entity %q already exists", id)
		}
	}

	entity := mustJSON(map[string]any{
		"id":              op.FeatureID,
		"type":            "fillet",
		"line1Id":         op.Line1ID,
		"line2Id":         op.Line2ID,
		"cornerPointId":   op.CornerPointID,
		"createdPoint1Id": op.CreatedPoint1ID,
		"createdPoint2Id": op.CreatedPoint2ID,
		"createdArcId":    op.CreatedArcID,
		"radius":          op.Radius,
		"trim":            op.Trim,
		"clockwise":       op.Clockwise,
	})
	graph.Entities[op.FeatureID] = entity
	return &sketchPatch{Entities: map[string]json.RawMessage{op.FeatureID: entity}}, affectedIDs{
		EntityIDs: []string{
			op.Line1ID,
			op.Line2ID,
			op.CornerPointID,
			op.CreatedPoint1ID,
			op.CreatedPoint2ID,
			op.CreatedArcID,
			op.FeatureID,
		},
	}, nil
}

func applyChamfer(graph *graphState, raw easyjson.RawMessage) (*sketchPatch, affectedIDs, error) {
	var op struct {
		FeatureID       string  `json:"featureId"`
		Line1ID         string  `json:"line1Id"`
		Line2ID         string  `json:"line2Id"`
		CornerPointID   string  `json:"cornerPointId"`
		CreatedPoint1ID string  `json:"createdPoint1Id"`
		CreatedPoint2ID string  `json:"createdPoint2Id"`
		CreatedLineID   string  `json:"createdLineId"`
		Distance1       float64 `json:"distance1"`
		Distance2       float64 `json:"distance2"`
		Trim            bool    `json:"trim"`
	}
	if err := json.Unmarshal(raw, &op); err != nil {
		return nil, affectedIDs{}, fmt.Errorf("decode ApplyChamfer: %w", err)
	}
	if op.FeatureID == "" {
		op.FeatureID = generatedID(raw, "chamfer")
	}
	op.FeatureID = strings.TrimSpace(op.FeatureID)
	op.Line1ID = strings.TrimSpace(op.Line1ID)
	op.Line2ID = strings.TrimSpace(op.Line2ID)
	op.CornerPointID = strings.TrimSpace(op.CornerPointID)
	op.CreatedPoint1ID = strings.TrimSpace(op.CreatedPoint1ID)
	op.CreatedPoint2ID = strings.TrimSpace(op.CreatedPoint2ID)
	op.CreatedLineID = strings.TrimSpace(op.CreatedLineID)
	if op.FeatureID == "" {
		return nil, affectedIDs{}, errors.New("featureId is required")
	}
	if _, exists := graph.Entities[op.FeatureID]; exists {
		return nil, affectedIDs{}, fmt.Errorf("entity %q already exists", op.FeatureID)
	}
	if op.Distance1 <= 0 {
		return nil, affectedIDs{}, errors.New("distance1 must be greater than 0")
	}
	if op.Distance2 <= 0 {
		return nil, affectedIDs{}, errors.New("distance2 must be greater than 0")
	}
	if err := requireLineEntity(graph, op.Line1ID, "line1Id"); err != nil {
		return nil, affectedIDs{}, err
	}
	if err := requireLineEntity(graph, op.Line2ID, "line2Id"); err != nil {
		return nil, affectedIDs{}, err
	}
	if op.Line1ID == op.Line2ID {
		return nil, affectedIDs{}, errors.New("line1Id and line2Id must be different")
	}
	if op.CornerPointID != "" {
		if err := requirePointEntity(graph, op.CornerPointID, "cornerPointId"); err != nil {
			return nil, affectedIDs{}, err
		}
	}
	if op.CreatedPoint1ID == "" {
		op.CreatedPoint1ID = generatedID(raw, "chamfer-point-1")
	}
	if op.CreatedPoint2ID == "" {
		op.CreatedPoint2ID = generatedID(raw, "chamfer-point-2")
	}
	if op.CreatedLineID == "" {
		op.CreatedLineID = generatedID(raw, "chamfer-line")
	}
	for _, id := range []string{op.CreatedPoint1ID, op.CreatedPoint2ID, op.CreatedLineID} {
		if _, exists := graph.Entities[id]; exists {
			return nil, affectedIDs{}, fmt.Errorf("entity %q already exists", id)
		}
	}

	entity := mustJSON(map[string]any{
		"id":              op.FeatureID,
		"type":            "chamfer",
		"line1Id":         op.Line1ID,
		"line2Id":         op.Line2ID,
		"cornerPointId":   op.CornerPointID,
		"createdPoint1Id": op.CreatedPoint1ID,
		"createdPoint2Id": op.CreatedPoint2ID,
		"createdLineId":   op.CreatedLineID,
		"distance1":       op.Distance1,
		"distance2":       op.Distance2,
		"trim":            op.Trim,
	})
	graph.Entities[op.FeatureID] = entity
	return &sketchPatch{Entities: map[string]json.RawMessage{op.FeatureID: entity}}, affectedIDs{
		EntityIDs: []string{
			op.Line1ID,
			op.Line2ID,
			op.CornerPointID,
			op.CreatedPoint1ID,
			op.CreatedPoint2ID,
			op.CreatedLineID,
			op.FeatureID,
		},
	}, nil
}

func applyUpdateFillet(graph *graphState, raw easyjson.RawMessage) (*sketchPatch, affectedIDs, error) {
	var op struct {
		FeatureID       string  `json:"featureId"`
		CreatedPoint1ID *string `json:"createdPoint1Id"`
		CreatedPoint2ID *string `json:"createdPoint2Id"`
		CreatedArcID    *string `json:"createdArcId"`
		Radius          float64 `json:"radius"`
		Trim            bool    `json:"trim"`
		Clockwise       bool    `json:"clockwise"`
	}
	if err := json.Unmarshal(raw, &op); err != nil {
		return nil, affectedIDs{}, fmt.Errorf("decode UpdateFillet: %w", err)
	}
	if op.CreatedPoint1ID != nil || op.CreatedPoint2ID != nil || op.CreatedArcID != nil {
		return nil, affectedIDs{}, errors.New("UpdateFillet must not include createdPoint1Id, createdPoint2Id, or createdArcId")
	}
	op.FeatureID = strings.TrimSpace(op.FeatureID)
	if op.FeatureID == "" {
		return nil, affectedIDs{}, errors.New("featureId is required")
	}
	if op.Radius <= 0 {
		return nil, affectedIDs{}, errors.New("radius must be greater than 0")
	}

	feature, err := featureEntity(graph, op.FeatureID, "fillet")
	if err != nil {
		return nil, affectedIDs{}, err
	}
	feature["radius"] = op.Radius
	feature["trim"] = op.Trim
	feature["clockwise"] = op.Clockwise

	body, err := json.Marshal(feature)
	if err != nil {
		return nil, affectedIDs{}, fmt.Errorf("encode fillet feature: %w", err)
	}
	graph.Entities[op.FeatureID] = body
	return &sketchPatch{Entities: map[string]json.RawMessage{op.FeatureID: body}}, affectedIDs{
		EntityIDs: featureAffectedIDs(feature, op.FeatureID),
	}, nil
}

func applyUpdateChamfer(graph *graphState, raw easyjson.RawMessage) (*sketchPatch, affectedIDs, error) {
	var op struct {
		FeatureID       string  `json:"featureId"`
		CreatedPoint1ID *string `json:"createdPoint1Id"`
		CreatedPoint2ID *string `json:"createdPoint2Id"`
		CreatedLineID   *string `json:"createdLineId"`
		Distance1       float64 `json:"distance1"`
		Distance2       float64 `json:"distance2"`
		Trim            bool    `json:"trim"`
	}
	if err := json.Unmarshal(raw, &op); err != nil {
		return nil, affectedIDs{}, fmt.Errorf("decode UpdateChamfer: %w", err)
	}
	if op.CreatedPoint1ID != nil || op.CreatedPoint2ID != nil || op.CreatedLineID != nil {
		return nil, affectedIDs{}, errors.New("UpdateChamfer must not include createdPoint1Id, createdPoint2Id, or createdLineId")
	}
	op.FeatureID = strings.TrimSpace(op.FeatureID)
	if op.FeatureID == "" {
		return nil, affectedIDs{}, errors.New("featureId is required")
	}
	if op.Distance1 <= 0 {
		return nil, affectedIDs{}, errors.New("distance1 must be greater than 0")
	}
	if op.Distance2 <= 0 {
		return nil, affectedIDs{}, errors.New("distance2 must be greater than 0")
	}

	feature, err := featureEntity(graph, op.FeatureID, "chamfer")
	if err != nil {
		return nil, affectedIDs{}, err
	}
	feature["distance1"] = op.Distance1
	feature["distance2"] = op.Distance2
	feature["trim"] = op.Trim

	body, err := json.Marshal(feature)
	if err != nil {
		return nil, affectedIDs{}, fmt.Errorf("encode chamfer feature: %w", err)
	}
	graph.Entities[op.FeatureID] = body
	return &sketchPatch{Entities: map[string]json.RawMessage{op.FeatureID: body}}, affectedIDs{
		EntityIDs: featureAffectedIDs(feature, op.FeatureID),
	}, nil
}

func applySetEntityConstruction(graph *graphState, raw easyjson.RawMessage) (*sketchPatch, affectedIDs, error) {
	var op struct {
		EntityID       string `json:"entityId"`
		IsConstruction *bool  `json:"isConstruction"`
	}
	if err := json.Unmarshal(raw, &op); err != nil {
		return nil, affectedIDs{}, fmt.Errorf("decode set_entity_construction: %w", err)
	}
	op.EntityID = strings.TrimSpace(op.EntityID)
	if op.EntityID == "" {
		return nil, affectedIDs{}, errors.New("entityId is required")
	}
	if op.IsConstruction == nil {
		return nil, affectedIDs{}, errors.New("isConstruction is required")
	}

	entity, err := mutableEntity(graph, op.EntityID)
	if err != nil {
		return nil, affectedIDs{}, err
	}
	entityType, _ := entity["type"].(string)
	switch entityType {
	case "line", "circle", "arc":
	default:
		return nil, affectedIDs{}, fmt.Errorf("entity %q is not construction-capable", op.EntityID)
	}
	entity["isConstruction"] = *op.IsConstruction

	body, err := json.Marshal(entity)
	if err != nil {
		return nil, affectedIDs{}, fmt.Errorf("encode entity %q: %w", op.EntityID, err)
	}
	graph.Entities[op.EntityID] = body
	return &sketchPatch{Entities: map[string]json.RawMessage{op.EntityID: body}}, affectedIDs{
		EntityIDs: []string{op.EntityID},
	}, nil
}

func applyEntityIntent(graph *graphState, raw easyjson.RawMessage, opType string) (*sketchPatch, affectedIDs, error) {
	var op struct {
		OpID              string   `json:"opId"`
		EntityID          string   `json:"entityId"`
		PointID           string   `json:"pointId"`
		Endpoint          string   `json:"endpoint"`
		CreatedPointID    string   `json:"createdPointId"`
		CreatedEntityIDs  []string `json:"createdEntityIds"`
		BoundaryEntityIDs []string `json:"boundaryEntityIds"`
		TargetEntityIDs   []string `json:"targetEntityIds"`
		PickPoint         *vec2Op  `json:"pickPoint"`
		Target            *vec2Op  `json:"target"`
	}
	if err := json.Unmarshal(raw, &op); err != nil {
		return nil, affectedIDs{}, fmt.Errorf("decode %s: %w", opType, err)
	}
	op.OpID = strings.TrimSpace(op.OpID)
	if op.OpID == "" {
		op.OpID = generatedID(raw, opType)
	}
	op.EntityID = strings.TrimSpace(op.EntityID)
	if op.EntityID == "" {
		return nil, affectedIDs{}, errors.New("entityId is required")
	}
	if _, exists := graph.Entities[op.EntityID]; !exists {
		return nil, affectedIDs{}, fmt.Errorf("entity %q does not exist", op.EntityID)
	}
	if _, exists := graph.Entities[op.OpID]; exists {
		return nil, affectedIDs{}, fmt.Errorf("entity %q already exists", op.OpID)
	}

	switch opType {
	case "split_entity", "trim_entity":
		if op.PickPoint == nil {
			return nil, affectedIDs{}, errors.New("pickPoint is required")
		}
	case "break_entity_at_point":
		op.PointID = strings.TrimSpace(op.PointID)
		if op.PointID == "" && op.PickPoint == nil {
			return nil, affectedIDs{}, errors.New("pointId or pickPoint is required")
		}
		if op.PointID != "" {
			if err := requirePointEntity(graph, op.PointID, "pointId"); err != nil {
				return nil, affectedIDs{}, err
			}
		}
	case "extend_entity":
		op.Endpoint = strings.TrimSpace(op.Endpoint)
		if op.Endpoint != "start" && op.Endpoint != "end" {
			return nil, affectedIDs{}, errors.New("endpoint must be start or end")
		}
		if op.Target == nil && len(op.TargetEntityIDs) == 0 {
			return nil, affectedIDs{}, errors.New("target or targetEntityIds is required")
		}
	}

	changed := []string{op.EntityID}
	changed = mergeIDs(changed, []string{op.PointID, op.CreatedPointID, op.OpID})
	changed = mergeIDs(changed, op.CreatedEntityIDs)
	changed = mergeIDs(changed, op.BoundaryEntityIDs)
	changed = mergeIDs(changed, op.TargetEntityIDs)

	entity := mustJSON(map[string]any{
		"id":                op.OpID,
		"type":              opType,
		"entityId":          op.EntityID,
		"pointId":           strings.TrimSpace(op.PointID),
		"endpoint":          strings.TrimSpace(op.Endpoint),
		"createdPointId":    strings.TrimSpace(op.CreatedPointID),
		"createdEntityIds":  nonEmptyIDs(op.CreatedEntityIDs),
		"boundaryEntityIds": nonEmptyIDs(op.BoundaryEntityIDs),
		"targetEntityIds":   nonEmptyIDs(op.TargetEntityIDs),
		"pickPoint":         op.PickPoint,
		"target":            op.Target,
	})
	graph.Entities[op.OpID] = entity
	return &sketchPatch{Entities: map[string]json.RawMessage{op.OpID: entity}}, affectedIDs{EntityIDs: changed}, nil
}

func applyMirrorEntities(graph *graphState, raw easyjson.RawMessage) (*sketchPatch, affectedIDs, error) {
	var op struct {
		FeatureID        string   `json:"featureId"`
		SourceEntityIDs  []string `json:"sourceEntityIds"`
		MirrorLineID     string   `json:"mirrorLineId"`
		CreatedEntityIDs []string `json:"createdEntityIds"`
		Copy             bool     `json:"copy"`
		KeepConstraints  bool     `json:"keepConstraints"`
	}
	if err := json.Unmarshal(raw, &op); err != nil {
		return nil, affectedIDs{}, fmt.Errorf("decode mirror_entities: %w", err)
	}
	op.FeatureID = defaultFeatureID(raw, strings.TrimSpace(op.FeatureID), "mirror")
	op.MirrorLineID = strings.TrimSpace(op.MirrorLineID)
	if op.FeatureID == "" {
		return nil, affectedIDs{}, errors.New("featureId is required")
	}
	if _, exists := graph.Entities[op.FeatureID]; exists {
		return nil, affectedIDs{}, fmt.Errorf("entity %q already exists", op.FeatureID)
	}
	sourceIDs, err := requireEntities(graph, op.SourceEntityIDs, "sourceEntityIds")
	if err != nil {
		return nil, affectedIDs{}, err
	}
	if err := requireLineEntity(graph, op.MirrorLineID, "mirrorLineId"); err != nil {
		return nil, affectedIDs{}, err
	}
	createdIDs, err := validateCreatedEntityIDs(graph, op.CreatedEntityIDs)
	if err != nil {
		return nil, affectedIDs{}, err
	}

	entity := mustJSON(map[string]any{
		"id":               op.FeatureID,
		"type":             "mirror_entities",
		"sourceEntityIds":  sourceIDs,
		"mirrorLineId":     op.MirrorLineID,
		"createdEntityIds": createdIDs,
		"copy":             op.Copy,
		"keepConstraints":  op.KeepConstraints,
	})
	graph.Entities[op.FeatureID] = entity
	changed := mergeIDs([]string{op.FeatureID, op.MirrorLineID}, sourceIDs)
	changed = mergeIDs(changed, createdIDs)
	return &sketchPatch{Entities: map[string]json.RawMessage{op.FeatureID: entity}}, affectedIDs{EntityIDs: changed}, nil
}

func applyLinearPattern(graph *graphState, raw easyjson.RawMessage) (*sketchPatch, affectedIDs, error) {
	var op struct {
		FeatureID        string   `json:"featureId"`
		SourceEntityIDs  []string `json:"sourceEntityIds"`
		Direction        *vec2Op  `json:"direction"`
		Spacing          float64  `json:"spacing"`
		Count            int32    `json:"count"`
		CreatedEntityIDs []string `json:"createdEntityIds"`
		KeepConstraints  bool     `json:"keepConstraints"`
	}
	if err := json.Unmarshal(raw, &op); err != nil {
		return nil, affectedIDs{}, fmt.Errorf("decode linear_pattern: %w", err)
	}
	op.FeatureID = defaultFeatureID(raw, strings.TrimSpace(op.FeatureID), "linear-pattern")
	if op.FeatureID == "" {
		return nil, affectedIDs{}, errors.New("featureId is required")
	}
	if _, exists := graph.Entities[op.FeatureID]; exists {
		return nil, affectedIDs{}, fmt.Errorf("entity %q already exists", op.FeatureID)
	}
	sourceIDs, err := requireEntities(graph, op.SourceEntityIDs, "sourceEntityIds")
	if err != nil {
		return nil, affectedIDs{}, err
	}
	if op.Direction == nil {
		return nil, affectedIDs{}, errors.New("direction is required")
	}
	if op.Direction.X == 0 && op.Direction.Y == 0 {
		return nil, affectedIDs{}, errors.New("direction must not be zero")
	}
	if op.Spacing <= 0 {
		return nil, affectedIDs{}, errors.New("spacing must be greater than 0")
	}
	if op.Count < 2 {
		return nil, affectedIDs{}, errors.New("count must be greater than or equal to 2")
	}
	createdIDs, err := validateCreatedEntityIDs(graph, op.CreatedEntityIDs)
	if err != nil {
		return nil, affectedIDs{}, err
	}

	entity := mustJSON(map[string]any{
		"id":               op.FeatureID,
		"type":             "linear_pattern",
		"sourceEntityIds":  sourceIDs,
		"direction":        op.Direction,
		"spacing":          op.Spacing,
		"count":            op.Count,
		"createdEntityIds": createdIDs,
		"keepConstraints":  op.KeepConstraints,
	})
	graph.Entities[op.FeatureID] = entity
	changed := mergeIDs([]string{op.FeatureID}, sourceIDs)
	changed = mergeIDs(changed, createdIDs)
	return &sketchPatch{Entities: map[string]json.RawMessage{op.FeatureID: entity}}, affectedIDs{EntityIDs: changed}, nil
}

func applyCircularPattern(graph *graphState, raw easyjson.RawMessage) (*sketchPatch, affectedIDs, error) {
	var op struct {
		FeatureID        string   `json:"featureId"`
		SourceEntityIDs  []string `json:"sourceEntityIds"`
		CenterPointID    string   `json:"centerPointId"`
		TotalAngleRad    float64  `json:"totalAngleRad"`
		Count            int32    `json:"count"`
		CreatedEntityIDs []string `json:"createdEntityIds"`
		RotateInstances  bool     `json:"rotateInstances"`
		KeepConstraints  bool     `json:"keepConstraints"`
	}
	if err := json.Unmarshal(raw, &op); err != nil {
		return nil, affectedIDs{}, fmt.Errorf("decode circular_pattern: %w", err)
	}
	op.FeatureID = defaultFeatureID(raw, strings.TrimSpace(op.FeatureID), "circular-pattern")
	op.CenterPointID = strings.TrimSpace(op.CenterPointID)
	if op.FeatureID == "" {
		return nil, affectedIDs{}, errors.New("featureId is required")
	}
	if _, exists := graph.Entities[op.FeatureID]; exists {
		return nil, affectedIDs{}, fmt.Errorf("entity %q already exists", op.FeatureID)
	}
	sourceIDs, err := requireEntities(graph, op.SourceEntityIDs, "sourceEntityIds")
	if err != nil {
		return nil, affectedIDs{}, err
	}
	if err := requirePointEntity(graph, op.CenterPointID, "centerPointId"); err != nil {
		return nil, affectedIDs{}, err
	}
	if op.TotalAngleRad == 0 {
		return nil, affectedIDs{}, errors.New("totalAngleRad must not be 0")
	}
	if op.Count < 2 {
		return nil, affectedIDs{}, errors.New("count must be greater than or equal to 2")
	}
	createdIDs, err := validateCreatedEntityIDs(graph, op.CreatedEntityIDs)
	if err != nil {
		return nil, affectedIDs{}, err
	}

	entity := mustJSON(map[string]any{
		"id":               op.FeatureID,
		"type":             "circular_pattern",
		"sourceEntityIds":  sourceIDs,
		"centerPointId":    op.CenterPointID,
		"totalAngleRad":    op.TotalAngleRad,
		"count":            op.Count,
		"createdEntityIds": createdIDs,
		"rotateInstances":  op.RotateInstances,
		"keepConstraints":  op.KeepConstraints,
	})
	graph.Entities[op.FeatureID] = entity
	changed := mergeIDs([]string{op.FeatureID, op.CenterPointID}, sourceIDs)
	changed = mergeIDs(changed, createdIDs)
	return &sketchPatch{Entities: map[string]json.RawMessage{op.FeatureID: entity}}, affectedIDs{EntityIDs: changed}, nil
}

func defaultFeatureID(raw easyjson.RawMessage, value string, prefix string) string {
	if value != "" {
		return value
	}
	return generatedID(raw, prefix)
}

func requireEntities(graph *graphState, ids []string, field string) ([]string, error) {
	ids = nonEmptyIDs(ids)
	if len(ids) == 0 {
		return nil, fmt.Errorf("%s must contain at least one entity", field)
	}
	for _, id := range ids {
		if _, exists := graph.Entities[id]; !exists {
			return nil, fmt.Errorf("%s contains missing entity %q", field, id)
		}
	}
	return ids, nil
}

func validateCreatedEntityIDs(graph *graphState, ids []string) ([]string, error) {
	ids = nonEmptyIDs(ids)
	seen := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if _, ok := seen[id]; ok {
			return nil, fmt.Errorf("createdEntityIds contains duplicate entity %q", id)
		}
		seen[id] = struct{}{}
		if _, exists := graph.Entities[id]; exists {
			return nil, fmt.Errorf("entity %q already exists", id)
		}
	}
	return ids, nil
}

func featureEntity(graph *graphState, id string, wantType string) (map[string]any, error) {
	entity, err := mutableEntity(graph, id)
	if err != nil {
		return nil, err
	}
	entityType, _ := entity["type"].(string)
	if entityType != wantType {
		return nil, fmt.Errorf("feature %q is not a %s", id, wantType)
	}
	return entity, nil
}

func mutableEntity(graph *graphState, id string) (map[string]any, error) {
	body, exists := graph.Entities[id]
	if !exists {
		return nil, fmt.Errorf("entity %q does not exist", id)
	}
	var entity map[string]any
	if err := json.Unmarshal(body, &entity); err != nil {
		return nil, fmt.Errorf("decode entity %q: %w", id, err)
	}
	return entity, nil
}

func featureAffectedIDs(feature map[string]any, featureID string) []string {
	ids := []string{featureID}
	for _, key := range []string{
		"line1Id", "line2Id", "cornerPointId",
		"createdPoint1Id", "createdPoint2Id", "createdArcId", "createdLineId",
	} {
		value, _ := feature[key].(string)
		ids = mergeIDs(ids, []string{value})
	}
	return ids
}

func nonEmptyIDs(ids []string) []string {
	result := make([]string, 0, len(ids))
	for _, id := range ids {
		id = strings.TrimSpace(id)
		if id != "" {
			result = append(result, id)
		}
	}
	return result
}

func requireLineEntity(graph *graphState, id string, field string) error {
	return requireEntityType(graph, id, field, "line")
}

func requirePointEntity(graph *graphState, id string, field string) error {
	return requireEntityType(graph, id, field, "point")
}

func requireEntityType(graph *graphState, id string, field string, wantType string) error {
	id = strings.TrimSpace(id)
	if id == "" {
		return fmt.Errorf("%s is required", field)
	}
	body, exists := graph.Entities[id]
	if !exists {
		return fmt.Errorf("%s %q does not exist", field, id)
	}
	var entity struct {
		Type string `json:"type"`
	}
	if err := json.Unmarshal(body, &entity); err != nil {
		return fmt.Errorf("decode %s %q: %w", field, id, err)
	}
	if entity.Type != wantType {
		return fmt.Errorf("%s %q is not a %s", field, id, wantType)
	}
	return nil
}

func ensurePoint(graph *graphState, patch *sketchPatch, point pointRefOrNew) (string, error) {
	point.Kind = strings.TrimSpace(point.Kind)
	point.PointID = strings.TrimSpace(point.PointID)

	switch point.Kind {
	case "existing_point":
		if point.PointID == "" {
			return "", errors.New("pointId is required")
		}
		if _, exists := graph.Entities[point.PointID]; !exists {
			return "", fmt.Errorf("point %q does not exist", point.PointID)
		}
		return point.PointID, nil
	case "new_point":
		if point.PointID == "" {
			point.PointID = strings.TrimSpace(point.DefaultID)
		}
		if point.PointID == "" {
			return "", errors.New("pointId is required")
		}
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

func applyDeleteEntity(graph *graphState, raw easyjson.RawMessage) (*sketchPatch, affectedIDs, error) {
	var op struct {
		EntityID string `json:"entityId"`
	}
	if err := json.Unmarshal(raw, &op); err != nil {
		return nil, affectedIDs{}, fmt.Errorf("decode delete_entity: %w", err)
	}
	op.EntityID = strings.TrimSpace(op.EntityID)
	if op.EntityID == "" {
		return nil, affectedIDs{}, errors.New("entityId is required")
	}
	if _, exists := graph.Entities[op.EntityID]; !exists {
		return nil, affectedIDs{}, fmt.Errorf("entity %q does not exist", op.EntityID)
	}
	delete(graph.Entities, op.EntityID)
	return &sketchPatch{DeletedEntityIDs: []string{op.EntityID}}, affectedIDs{EntityIDs: []string{op.EntityID}}, nil
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

type dimensionPayload struct {
	ID          string   `json:"id"`
	Type        string   `json:"type"`
	Refs        []string `json:"refs"`
	Value       float64  `json:"value"`
	Driving     bool     `json:"driving"`
	Status      string   `json:"status,omitempty"`
	RefAID      string   `json:"refAId,omitempty"`
	RefBID      string   `json:"refBId,omitempty"`
	RefKind     string   `json:"refKind,omitempty"`
	EntityID    string   `json:"entityId,omitempty"`
	LineAID     string   `json:"lineAId,omitempty"`
	LineBID     string   `json:"lineBId,omitempty"`
	Orientation string   `json:"orientation,omitempty"`
}

func applyAddConstraint(graph *graphState, raw easyjson.RawMessage) (*sketchPatch, affectedIDs, error) {
	var op struct {
		ConstraintID string             `json:"constraintId"`
		Constraint   *constraintPayload `json:"constraint"`
	}
	if err := json.Unmarshal(raw, &op); err != nil {
		return nil, affectedIDs{}, fmt.Errorf("decode add_constraint: %w", err)
	}

	constraint := op.Constraint
	if constraint == nil {
		var inline constraintPayload
		if err := json.Unmarshal(raw, &inline); err != nil {
			return nil, affectedIDs{}, fmt.Errorf("decode inline add_constraint: %w", err)
		}
		constraint = &inline
	}
	if constraint.ID == "" {
		constraint.ID = op.ConstraintID
	}
	if constraint.ID == "" {
		constraint.ID = generatedID(raw, "constraint")
	}
	normalizeConstraint(constraint)
	if err := validateConstraint(graph, constraint); err != nil {
		return nil, affectedIDs{}, err
	}
	if constraint.Status == "" {
		constraint.Status = "active"
	}

	body, err := json.Marshal(constraint)
	if err != nil {
		return nil, affectedIDs{}, fmt.Errorf("encode constraint: %w", err)
	}
	graph.Constraints[constraint.ID] = body

	changed := constraintReferencedEntityIDs(constraint)
	return &sketchPatch{Constraints: map[string]json.RawMessage{constraint.ID: body}}, affectedIDs{
		EntityIDs:     changed,
		ConstraintIDs: []string{constraint.ID},
	}, nil
}

func applyRemoveConstraint(graph *graphState, raw easyjson.RawMessage) (*sketchPatch, affectedIDs, error) {
	var op struct {
		ConstraintID string `json:"constraintId"`
	}
	if err := json.Unmarshal(raw, &op); err != nil {
		return nil, affectedIDs{}, fmt.Errorf("decode remove_constraint: %w", err)
	}
	op.ConstraintID = strings.TrimSpace(op.ConstraintID)
	if op.ConstraintID == "" {
		return nil, affectedIDs{}, errors.New("constraintId is required")
	}

	body, exists := graph.Constraints[op.ConstraintID]
	if !exists {
		return nil, affectedIDs{}, fmt.Errorf("constraint %q does not exist", op.ConstraintID)
	}

	var constraint constraintPayload
	if err := json.Unmarshal(body, &constraint); err != nil {
		return nil, affectedIDs{}, fmt.Errorf("decode existing constraint %q: %w", op.ConstraintID, err)
	}
	normalizeConstraint(&constraint)
	changed := constraintReferencedEntityIDs(&constraint)

	delete(graph.Constraints, op.ConstraintID)
	return &sketchPatch{DeletedConstraintIDs: []string{op.ConstraintID}}, affectedIDs{
		EntityIDs:     changed,
		ConstraintIDs: []string{op.ConstraintID},
	}, nil
}

func applyAddDimension(graph *graphState, raw easyjson.RawMessage) (*sketchPatch, affectedIDs, error) {
	var op struct {
		DimensionID string            `json:"dimensionId"`
		Dimension   *dimensionPayload `json:"dimension"`
	}
	if err := json.Unmarshal(raw, &op); err != nil {
		return nil, affectedIDs{}, fmt.Errorf("decode add_dimension: %w", err)
	}

	dimension := op.Dimension
	if dimension == nil {
		var inline dimensionPayload
		if err := json.Unmarshal(raw, &inline); err != nil {
			return nil, affectedIDs{}, fmt.Errorf("decode inline add_dimension: %w", err)
		}
		dimension = &inline
	}
	if dimension.ID == "" {
		dimension.ID = op.DimensionID
	}
	if dimension.ID == "" {
		dimension.ID = generatedID(raw, "dimension")
	}
	normalizeDimension(dimension)
	if err := validateDimension(graph, dimension); err != nil {
		return nil, affectedIDs{}, err
	}
	if dimension.Status == "" {
		dimension.Status = "active"
	}

	body, err := json.Marshal(dimension)
	if err != nil {
		return nil, affectedIDs{}, fmt.Errorf("encode dimension: %w", err)
	}
	graph.Dimensions[dimension.ID] = body

	return &sketchPatch{Dimensions: map[string]json.RawMessage{dimension.ID: body}}, affectedIDs{
		EntityIDs:    dimensionReferencedEntityIDs(dimension),
		DimensionIDs: []string{dimension.ID},
	}, nil
}

func applySetDimensionValue(graph *graphState, raw easyjson.RawMessage) (*sketchPatch, affectedIDs, error) {
	var op struct {
		DimensionID string  `json:"dimensionId"`
		Value       float64 `json:"value"`
	}
	if err := json.Unmarshal(raw, &op); err != nil {
		return nil, affectedIDs{}, fmt.Errorf("decode set_dimension_value: %w", err)
	}
	op.DimensionID = strings.TrimSpace(op.DimensionID)
	if op.DimensionID == "" {
		return nil, affectedIDs{}, errors.New("dimensionId is required")
	}

	body, exists := graph.Dimensions[op.DimensionID]
	if !exists {
		return nil, affectedIDs{}, fmt.Errorf("dimension %q does not exist", op.DimensionID)
	}
	var dimension dimensionPayload
	if err := json.Unmarshal(body, &dimension); err != nil {
		return nil, affectedIDs{}, fmt.Errorf("decode dimension %q: %w", op.DimensionID, err)
	}
	normalizeDimension(&dimension)
	dimension.Value = op.Value

	next, err := json.Marshal(&dimension)
	if err != nil {
		return nil, affectedIDs{}, fmt.Errorf("encode dimension: %w", err)
	}
	graph.Dimensions[op.DimensionID] = next
	return &sketchPatch{Dimensions: map[string]json.RawMessage{op.DimensionID: next}}, affectedIDs{
		EntityIDs:    dimensionReferencedEntityIDs(&dimension),
		DimensionIDs: []string{op.DimensionID},
	}, nil
}

func applyRemoveDimension(graph *graphState, raw easyjson.RawMessage) (*sketchPatch, affectedIDs, error) {
	var op struct {
		DimensionID string `json:"dimensionId"`
	}
	if err := json.Unmarshal(raw, &op); err != nil {
		return nil, affectedIDs{}, fmt.Errorf("decode remove_dimension: %w", err)
	}
	op.DimensionID = strings.TrimSpace(op.DimensionID)
	if op.DimensionID == "" {
		return nil, affectedIDs{}, errors.New("dimensionId is required")
	}

	body, exists := graph.Dimensions[op.DimensionID]
	if !exists {
		return nil, affectedIDs{}, fmt.Errorf("dimension %q does not exist", op.DimensionID)
	}
	var dimension dimensionPayload
	if err := json.Unmarshal(body, &dimension); err != nil {
		return nil, affectedIDs{}, fmt.Errorf("decode dimension %q: %w", op.DimensionID, err)
	}
	normalizeDimension(&dimension)
	delete(graph.Dimensions, op.DimensionID)
	return &sketchPatch{DeletedDimensionIDs: []string{op.DimensionID}}, affectedIDs{
		EntityIDs:    dimensionReferencedEntityIDs(&dimension),
		DimensionIDs: []string{op.DimensionID},
	}, nil
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

func normalizeDimension(dimension *dimensionPayload) {
	dimension.ID = strings.TrimSpace(dimension.ID)
	dimension.Type = strings.TrimSpace(dimension.Type)
	dimension.Status = strings.TrimSpace(dimension.Status)
	dimension.RefAID = strings.TrimSpace(dimension.RefAID)
	dimension.RefBID = strings.TrimSpace(dimension.RefBID)
	dimension.RefKind = strings.TrimSpace(dimension.RefKind)
	dimension.EntityID = strings.TrimSpace(dimension.EntityID)
	dimension.LineAID = strings.TrimSpace(dimension.LineAID)
	dimension.LineBID = strings.TrimSpace(dimension.LineBID)
	dimension.Orientation = strings.TrimSpace(dimension.Orientation)
	for i := range dimension.Refs {
		dimension.Refs[i] = strings.TrimSpace(dimension.Refs[i])
	}
	inferDimensionRefs(dimension)
}

func inferDimensionRefs(dimension *dimensionPayload) {
	ref := func(index int, refs []string) string {
		if index < 0 || index >= len(refs) {
			return ""
		}
		return refs[index]
	}

	switch dimension.Type {
	case "distance":
		entityRefs := make([]string, 0, len(dimension.Refs))
		for _, candidate := range dimension.Refs {
			if normalized := normalizeDistanceRefKind(candidate); normalized != "" {
				if dimension.RefKind == "" {
					dimension.RefKind = normalized
				}
				continue
			}
			entityRefs = append(entityRefs, candidate)
		}
		if dimension.RefAID == "" {
			dimension.RefAID = ref(0, entityRefs)
		}
		if dimension.RefBID == "" {
			dimension.RefBID = ref(1, entityRefs)
		}
		if dimension.RefKind == "" {
			dimension.RefKind = "point_point"
		}
	case "radius", "diameter":
		if dimension.EntityID == "" {
			dimension.EntityID = ref(0, dimension.Refs)
		}
	case "angle":
		if dimension.LineAID == "" {
			dimension.LineAID = ref(0, dimension.Refs)
		}
		if dimension.LineBID == "" {
			dimension.LineBID = ref(1, dimension.Refs)
		}
	}
}

func isDistanceRefKind(value string) bool {
	return normalizeDistanceRefKind(value) != ""
}

func normalizeDistanceRefKind(value string) string {
	switch strings.TrimSpace(value) {
	case "point_point", "point_line", "line_line":
		return strings.TrimSpace(value)
	case "parallel_lines":
		return "line_line"
	default:
		return ""
	}
}

func validateDimension(graph *graphState, dimension *dimensionPayload) error {
	if dimension.ID == "" {
		return errors.New("dimension.id or dimensionId is required")
	}
	if _, exists := graph.Dimensions[dimension.ID]; exists {
		return fmt.Errorf("dimension %q already exists", dimension.ID)
	}
	if dimension.Type == "" {
		return errors.New("dimension.type is required")
	}

	refs := dimensionReferencedEntityIDs(dimension)
	if len(refs) == 0 {
		return errors.New("dimension must reference at least one entity")
	}
	for _, ref := range refs {
		if ref == "" {
			return errors.New("dimension references must not be empty")
		}
		if _, exists := graph.Entities[ref]; !exists {
			return fmt.Errorf("dimension reference %q does not exist", ref)
		}
	}

	switch dimension.Type {
	case "distance", "radius", "diameter", "angle":
		return nil
	default:
		return fmt.Errorf("unsupported dimension type %q", dimension.Type)
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

func dimensionReferencedEntityIDs(dimension *dimensionPayload) []string {
	seen := make(map[string]struct{})
	refs := make([]string, 0, len(dimension.Refs)+5)
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

	switch dimension.Type {
	case "distance":
		add(dimension.RefAID)
		add(dimension.RefBID)
	case "radius", "diameter":
		add(dimension.EntityID)
	case "angle":
		add(dimension.LineAID)
		add(dimension.LineBID)
	default:
		for _, ref := range dimension.Refs {
			if !isDistanceRefKind(ref) {
				add(ref)
			}
		}
		add(dimension.RefAID)
		add(dimension.RefBID)
		add(dimension.EntityID)
		add(dimension.LineAID)
		add(dimension.LineBID)
	}

	return refs
}

func generatedID(raw easyjson.RawMessage, suffix string) string {
	sum := sha1.Sum(append(append([]byte(nil), raw...), []byte("|"+suffix)...))
	return suffix + "-" + hex.EncodeToString(sum[:8])
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
