package operations

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/dnonakolesax/cccad-locks/internal/auth"
	"github.com/dnonakolesax/cccad-locks/internal/model"
	solverv1 "github.com/dnonakolesax/cccad-locks/internal/proto/solver/v1"
)

type repositoryStub struct {
	listUserID       string
	listSketchID     string
	listAfterVersion int64
	listLimit        int
	listResult       *model.SketchOperationPage
	submitUserID     string
	submitSketchID   string
	submitRequest    model.SubmitCommitRequest
	submitState      *model.SubmitState
	submitResult     *model.SubmitCommitResult
}

type solverStub struct {
	solveRequest *solverv1.SolveRequest
	response     *solverv1.SolveResponse
}

func (s *solverStub) Solve(
	_ context.Context,
	request *solverv1.SolveRequest,
) (*solverv1.SolveResponse, error) {
	s.solveRequest = request
	if s.response != nil {
		return s.response, nil
	}
	return &solverv1.SolveResponse{
		Status:           solverv1.SolveStatus_SOLVE_STATUS_OK,
		DegreesOfFreedom: 1,
		Solution: &solverv1.SketchSolution{
			Entities: []*solverv1.SolvedEntity{
				{
					Id: "point-2",
					Kind: &solverv1.SolvedEntity_Point{
						Point: &solverv1.SolvedPoint{X: 4, Y: 1},
					},
				},
			},
		},
	}, nil
}

func (r *repositoryStub) List(
	_ context.Context,
	userID string,
	sketchID string,
	afterVersion int64,
	limit int,
) (*model.SketchOperationPage, error) {
	r.listUserID = userID
	r.listSketchID = sketchID
	r.listAfterVersion = afterVersion
	r.listLimit = limit
	return r.listResult, nil
}

func (r *repositoryStub) GetSubmitState(
	_ context.Context,
	userID string,
	sketchID string,
) (*model.SubmitState, error) {
	r.submitUserID = userID
	r.submitSketchID = sketchID
	if r.submitState != nil {
		return r.submitState, nil
	}
	return &model.SubmitState{
		Version:              0,
		GraphState:           []byte(`{"entities":{},"constraints":{},"dimensions":{},"groups":{}}`),
		MaterializedGeometry: []byte(`{"entities":{}}`),
		SolveStatus:          []byte(`{"status":"ok","degreesOfFreedom":0,"diagnostics":[]}`),
	}, nil
}

func (r *repositoryStub) Submit(
	_ context.Context,
	userID string,
	sketchID string,
	request model.SubmitCommitRequest,
) (*model.SubmitCommitResult, error) {
	r.submitUserID = userID
	r.submitSketchID = sketchID
	r.submitRequest = request
	if r.submitResult != nil {
		return r.submitResult, nil
	}
	return &model.SubmitCommitResult{
		Status:         "committed",
		OpID:           "op-id",
		Version:        request.BaseVersion + 1,
		CurrentVersion: request.BaseVersion + 1,
	}, nil
}

func TestServiceListUsesAuthenticatedUser(t *testing.T) {
	repo := &repositoryStub{
		listResult: &model.SketchOperationPage{SketchID: "sketch-id"},
	}
	service := NewService(repo)
	ctx := auth.ContextWithUserID(context.Background(), "user-id")

	page, err := service.List(ctx, " sketch-id ", 12, 50)
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if page == nil {
		t.Fatal("List returned nil page")
	}
	if repo.listUserID != "user-id" {
		t.Fatalf("List userID = %q, want %q", repo.listUserID, "user-id")
	}
	if repo.listSketchID != "sketch-id" {
		t.Fatalf("List sketchID = %q, want %q", repo.listSketchID, "sketch-id")
	}
	if repo.listAfterVersion != 12 {
		t.Fatalf("List afterVersion = %d, want 12", repo.listAfterVersion)
	}
	if repo.listLimit != 50 {
		t.Fatalf("List limit = %d, want 50", repo.listLimit)
	}
}

func TestServiceListRequiresAuthenticatedUser(t *testing.T) {
	service := NewService(&repositoryStub{})

	if _, err := service.List(context.Background(), "sketch-id", 0, 50); err == nil {
		t.Fatal("List returned nil error without authenticated user")
	}
}

func TestServiceListRejectsInvalidArguments(t *testing.T) {
	service := NewService(&repositoryStub{})
	ctx := auth.ContextWithUserID(context.Background(), "user-id")

	tests := map[string]struct {
		sketchID     string
		afterVersion int64
		limit        int
	}{
		"blank sketch id":  {sketchID: " ", afterVersion: 0, limit: 1},
		"negative version": {sketchID: "sketch-id", afterVersion: -1, limit: 1},
		"zero limit":       {sketchID: "sketch-id", afterVersion: 0, limit: 0},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := service.List(ctx, tt.sketchID, tt.afterVersion, tt.limit); err == nil {
				t.Fatal("List returned nil error")
			}
		})
	}
}

func TestServiceSubmitCreateLineCommitsGraphState(t *testing.T) {
	repo := &repositoryStub{}
	service := NewServiceWithSolver(repo, &solverStub{})
	ctx := auth.ContextWithUserID(context.Background(), "user-id")

	response, err := service.Submit(ctx, " sketch-id ", &model.SubmitOperationRequest{
		BaseVersion: 0,
		ClientOpID:  "client-op-id",
		Op: []byte(`{
			"type":"create_line",
			"entityId":"line-1",
			"start":{"kind":"new_point","pointId":"point-1","x":8,"y":4.055555555555555},
			"end":{"kind":"new_point","pointId":"point-2","x":16.060741644965276,"y":4}
		}`),
	})
	if err != nil {
		t.Fatalf("Submit returned error: %v", err)
	}
	if response == nil || !response.Accepted {
		t.Fatalf("Submit accepted = false, response = %#v", response)
	}
	if repo.submitUserID != "user-id" {
		t.Fatalf("Submit userID = %q, want user-id", repo.submitUserID)
	}
	if repo.submitSketchID != "sketch-id" {
		t.Fatalf("Submit sketchID = %q, want sketch-id", repo.submitSketchID)
	}
	if repo.submitRequest.OpType != "create_line" {
		t.Fatalf("Submit opType = %q, want create_line", repo.submitRequest.OpType)
	}
	if len(response.ChangedEntityIDs) != 3 {
		t.Fatalf("ChangedEntityIDs length = %d, want 3", len(response.ChangedEntityIDs))
	}

	var graph struct {
		Entities map[string]map[string]any `json:"entities"`
	}
	if err := json.Unmarshal(repo.submitRequest.GraphState, &graph); err != nil {
		t.Fatalf("decode graph state: %v", err)
	}
	if graph.Entities["line-1"]["type"] != "line" {
		t.Fatalf("line entity type = %#v, want line", graph.Entities["line-1"]["type"])
	}
	if graph.Entities["point-1"]["type"] != "point" {
		t.Fatalf("point entity type = %#v, want point", graph.Entities["point-1"]["type"])
	}
}

func TestServiceSubmitRejectsStaleVersion(t *testing.T) {
	repo := &repositoryStub{
		submitState: &model.SubmitState{
			Version:              4,
			GraphState:           []byte(`{"entities":{},"constraints":{},"dimensions":{},"groups":{}}`),
			MaterializedGeometry: []byte(`{"entities":{}}`),
			SolveStatus:          []byte(`{"status":"ok","degreesOfFreedom":0,"diagnostics":[]}`),
		},
		submitResult: &model.SubmitCommitResult{Status: "stale_version", CurrentVersion: 4},
	}
	service := NewService(repo)
	ctx := auth.ContextWithUserID(context.Background(), "user-id")

	response, err := service.Submit(ctx, "sketch-id", &model.SubmitOperationRequest{
		BaseVersion: 3,
		ClientOpID:  "client-op-id",
		Op:          []byte(`{"type":"create_point","pointId":"point-1","x":1,"y":2}`),
	})
	if err != nil {
		t.Fatalf("Submit returned error: %v", err)
	}
	if response.Accepted {
		t.Fatal("Submit accepted stale operation")
	}
	if response.CurrentVersion != 4 {
		t.Fatalf("CurrentVersion = %d, want 4", response.CurrentVersion)
	}
	if response.Rejection == nil || response.Rejection.Reason != "stale_version" {
		t.Fatalf("Rejection = %#v, want stale_version", response.Rejection)
	}
}

func TestServiceSubmitAddConstraintCommitsGraphState(t *testing.T) {
	repo := &repositoryStub{
		submitState: &model.SubmitState{
			Version: 1,
			GraphState: []byte(`{
				"entities":{
					"line-1":{"id":"line-1","type":"line","startPointId":"point-1","endPointId":"point-2"}
				},
				"constraints":{},
				"dimensions":{},
				"groups":{}
			}`),
			MaterializedGeometry: []byte(`{"entities":{}}`),
			SolveStatus:          []byte(`{"status":"ok","degreesOfFreedom":0,"diagnostics":[]}`),
		},
	}
	solver := &solverStub{}
	service := NewServiceWithSolver(repo, solver)
	ctx := auth.ContextWithUserID(context.Background(), "user-id")

	response, err := service.Submit(ctx, "sketch-id", &model.SubmitOperationRequest{
		BaseVersion: 1,
		ClientOpID:  "client-op-id",
		Op: []byte(`{
			"type":"add_constraint",
			"constraint":{"id":"constraint-1","type":"horizontal","refs":["line-1"]}
		}`),
	})
	if err != nil {
		t.Fatalf("Submit returned error: %v", err)
	}
	if response == nil || !response.Accepted {
		t.Fatalf("Submit accepted = false, response = %#v", response)
	}
	if repo.submitRequest.OpType != "add_constraint" {
		t.Fatalf("Submit opType = %q, want add_constraint", repo.submitRequest.OpType)
	}
	if solver.solveRequest == nil {
		t.Fatal("solver Solve was not called")
	}
	if len(solver.solveRequest.GetModel().GetConstraints()) != 1 ||
		solver.solveRequest.GetModel().GetConstraints()[0].GetHorizontal().GetLineId() != "line-1" {
		t.Fatalf("solver constraints = %#v, want horizontal line-1", solver.solveRequest.GetModel().GetConstraints())
	}

	var graph struct {
		Constraints map[string]map[string]any `json:"constraints"`
		Entities    map[string]map[string]any `json:"entities"`
	}
	if err := json.Unmarshal(repo.submitRequest.GraphState, &graph); err != nil {
		t.Fatalf("decode graph state: %v", err)
	}
	constraint := graph.Constraints["constraint-1"]
	if constraint["type"] != "horizontal" {
		t.Fatalf("constraint type = %#v, want horizontal", constraint["type"])
	}
	if constraint["status"] != "active" {
		t.Fatalf("constraint status = %#v, want active", constraint["status"])
	}
	if constraint["lineId"] != "line-1" {
		t.Fatalf("constraint lineId = %#v, want line-1", constraint["lineId"])
	}
	if graph.Entities["point-2"]["x"] != float64(4) || graph.Entities["point-2"]["y"] != float64(1) {
		t.Fatalf("solved point-2 = %#v, want x=4 y=1", graph.Entities["point-2"])
	}
	if len(response.ChangedEntityIDs) != 2 ||
		response.ChangedEntityIDs[0] != "line-1" ||
		response.ChangedEntityIDs[1] != "point-2" {
		t.Fatalf("ChangedEntityIDs = %#v, want [line-1 point-2]", response.ChangedEntityIDs)
	}

	var patch struct {
		Constraints map[string]map[string]any `json:"constraints"`
		Entities    map[string]map[string]any `json:"entities"`
	}
	if err := json.Unmarshal(repo.submitRequest.Patch, &patch); err != nil {
		t.Fatalf("decode patch: %v", err)
	}
	if patch.Constraints["constraint-1"]["type"] != "horizontal" {
		t.Fatalf("patch constraint type = %#v, want horizontal", patch.Constraints["constraint-1"]["type"])
	}
	if patch.Entities["point-2"]["x"] != float64(4) || patch.Entities["point-2"]["y"] != float64(1) {
		t.Fatalf("patch point-2 = %#v, want x=4 y=1", patch.Entities["point-2"])
	}

	var solveStatus struct {
		Status           string `json:"status"`
		DegreesOfFreedom int    `json:"degreesOfFreedom"`
	}
	if err := json.Unmarshal(repo.submitRequest.SolveStatus, &solveStatus); err != nil {
		t.Fatalf("decode solve status: %v", err)
	}
	if solveStatus.Status != "ok" || solveStatus.DegreesOfFreedom != 1 {
		t.Fatalf("solve status = %#v, want ok with 1 DOF", solveStatus)
	}
}

func TestServiceSubmitRemoveConstraintCommitsGraphState(t *testing.T) {
	repo := &repositoryStub{
		submitState: &model.SubmitState{
			Version: 2,
			GraphState: []byte(`{
				"entities":{
					"line-1":{"id":"line-1","type":"line","startPointId":"point-1","endPointId":"point-2"}
				},
				"constraints":{
					"constraint-1":{"id":"constraint-1","type":"horizontal","refs":["line-1"],"status":"active"}
				},
				"dimensions":{},
				"groups":{}
			}`),
			MaterializedGeometry: []byte(`{"entities":{}}`),
			SolveStatus:          []byte(`{"status":"ok","degreesOfFreedom":0,"diagnostics":[]}`),
		},
	}
	service := NewServiceWithSolver(repo, &solverStub{response: &solverv1.SolveResponse{
		Status:           solverv1.SolveStatus_SOLVE_STATUS_OK,
		DegreesOfFreedom: 2,
		Solution:         &solverv1.SketchSolution{},
	}})
	ctx := auth.ContextWithUserID(context.Background(), "user-id")

	response, err := service.Submit(ctx, "sketch-id", &model.SubmitOperationRequest{
		BaseVersion: 2,
		ClientOpID:  "client-op-id",
		Op: []byte(`{
			"type":"remove_constraint",
			"constraintId":"constraint-1"
		}`),
	})
	if err != nil {
		t.Fatalf("Submit returned error: %v", err)
	}
	if response == nil || !response.Accepted {
		t.Fatalf("Submit accepted = false, response = %#v", response)
	}
	if repo.submitRequest.OpType != "remove_constraint" {
		t.Fatalf("Submit opType = %q, want remove_constraint", repo.submitRequest.OpType)
	}

	var graph struct {
		Constraints map[string]map[string]any `json:"constraints"`
	}
	if err := json.Unmarshal(repo.submitRequest.GraphState, &graph); err != nil {
		t.Fatalf("decode graph state: %v", err)
	}
	if _, exists := graph.Constraints["constraint-1"]; exists {
		t.Fatal("constraint-1 still exists in graph state")
	}

	var patch struct {
		DeletedConstraintIDs []string `json:"deletedConstraintIds"`
	}
	if err := json.Unmarshal(repo.submitRequest.Patch, &patch); err != nil {
		t.Fatalf("decode patch: %v", err)
	}
	if len(patch.DeletedConstraintIDs) != 1 || patch.DeletedConstraintIDs[0] != "constraint-1" {
		t.Fatalf("DeletedConstraintIDs = %#v, want [constraint-1]", patch.DeletedConstraintIDs)
	}
	if len(response.ChangedEntityIDs) != 1 || response.ChangedEntityIDs[0] != "line-1" {
		t.Fatalf("ChangedEntityIDs = %#v, want [line-1]", response.ChangedEntityIDs)
	}
}

func TestServiceSubmitApplyFilletCommitsFeatureIntent(t *testing.T) {
	repo := &repositoryStub{
		submitState: &model.SubmitState{
			Version: 3,
			GraphState: []byte(`{
				"entities":{
					"corner":{"id":"corner","type":"point","x":0,"y":0},
					"line-1":{"id":"line-1","type":"line","startPointId":"corner","endPointId":"p1"},
					"line-2":{"id":"line-2","type":"line","startPointId":"corner","endPointId":"p2"}
				},
				"constraints":{},
				"dimensions":{},
				"groups":{}
			}`),
			MaterializedGeometry: []byte(`{"entities":{}}`),
			SolveStatus:          []byte(`{"status":"ok","degreesOfFreedom":0,"diagnostics":[]}`),
		},
	}
	solver := &solverStub{response: &solverv1.SolveResponse{
		Status:           solverv1.SolveStatus_SOLVE_STATUS_OK,
		DegreesOfFreedom: 3,
		Solution:         &solverv1.SketchSolution{},
	}}
	service := NewServiceWithSolver(repo, solver)
	ctx := auth.ContextWithUserID(context.Background(), "user-id")

	response, err := service.Submit(ctx, "sketch-id", &model.SubmitOperationRequest{
		BaseVersion: 3,
		ClientOpID:  "client-op-id",
		Op: []byte(`{
			"type":"ApplyFillet",
			"featureId":"fillet-1",
			"line1Id":"line-1",
			"line2Id":"line-2",
			"cornerPointId":"corner",
			"createdPoint1Id":"fillet-p1",
			"createdPoint2Id":"fillet-p2",
			"createdArcId":"fillet-arc",
			"radius":4.5,
			"trim":true,
			"clockwise":false
		}`),
	})
	if err != nil {
		t.Fatalf("Submit returned error: %v", err)
	}
	if response == nil || !response.Accepted {
		t.Fatalf("Submit accepted = false, response = %#v", response)
	}
	if repo.submitRequest.OpType != "ApplyFillet" {
		t.Fatalf("Submit opType = %q, want ApplyFillet", repo.submitRequest.OpType)
	}
	if solver.solveRequest == nil {
		t.Fatal("solver Solve was not called")
	}

	var graph struct {
		Entities map[string]map[string]any `json:"entities"`
	}
	if err := json.Unmarshal(repo.submitRequest.GraphState, &graph); err != nil {
		t.Fatalf("decode graph state: %v", err)
	}
	feature := graph.Entities["fillet-1"]
	if feature["type"] != "fillet" {
		t.Fatalf("feature type = %#v, want fillet", feature["type"])
	}
	if feature["createdArcId"] != "fillet-arc" || feature["radius"] != float64(4.5) {
		t.Fatalf("fillet feature = %#v, want createdArcId fillet-arc radius 4.5", feature)
	}
	if !containsAll(response.ChangedEntityIDs, "line-1", "line-2", "corner", "fillet-p1", "fillet-p2", "fillet-arc", "fillet-1") {
		t.Fatalf("ChangedEntityIDs = %#v, missing fillet affected IDs", response.ChangedEntityIDs)
	}
}

func TestServiceSubmitApplyChamferCommitsFeatureIntent(t *testing.T) {
	repo := &repositoryStub{
		submitState: &model.SubmitState{
			Version: 5,
			GraphState: []byte(`{
				"entities":{
					"corner":{"id":"corner","type":"point","x":0,"y":0},
					"line-1":{"id":"line-1","type":"line","startPointId":"corner","endPointId":"p1"},
					"line-2":{"id":"line-2","type":"line","startPointId":"corner","endPointId":"p2"}
				},
				"constraints":{},
				"dimensions":{},
				"groups":{}
			}`),
			MaterializedGeometry: []byte(`{"entities":{}}`),
			SolveStatus:          []byte(`{"status":"ok","degreesOfFreedom":0,"diagnostics":[]}`),
		},
	}
	solver := &solverStub{response: &solverv1.SolveResponse{
		Status:           solverv1.SolveStatus_SOLVE_STATUS_OK,
		DegreesOfFreedom: 4,
		Solution:         &solverv1.SketchSolution{},
	}}
	service := NewServiceWithSolver(repo, solver)
	ctx := auth.ContextWithUserID(context.Background(), "user-id")

	response, err := service.Submit(ctx, "sketch-id", &model.SubmitOperationRequest{
		BaseVersion: 5,
		ClientOpID:  "client-op-id",
		Op: []byte(`{
			"type":"ApplyChamfer",
			"featureId":"chamfer-1",
			"line1Id":"line-1",
			"line2Id":"line-2",
			"cornerPointId":"corner",
			"createdPoint1Id":"chamfer-p1",
			"createdPoint2Id":"chamfer-p2",
			"createdLineId":"chamfer-line",
			"distance1":2,
			"distance2":3,
			"trim":true
		}`),
	})
	if err != nil {
		t.Fatalf("Submit returned error: %v", err)
	}
	if response == nil || !response.Accepted {
		t.Fatalf("Submit accepted = false, response = %#v", response)
	}
	if repo.submitRequest.OpType != "ApplyChamfer" {
		t.Fatalf("Submit opType = %q, want ApplyChamfer", repo.submitRequest.OpType)
	}
	if solver.solveRequest == nil {
		t.Fatal("solver Solve was not called")
	}

	var graph struct {
		Entities map[string]map[string]any `json:"entities"`
	}
	if err := json.Unmarshal(repo.submitRequest.GraphState, &graph); err != nil {
		t.Fatalf("decode graph state: %v", err)
	}
	feature := graph.Entities["chamfer-1"]
	if feature["type"] != "chamfer" {
		t.Fatalf("feature type = %#v, want chamfer", feature["type"])
	}
	if feature["createdLineId"] != "chamfer-line" || feature["distance1"] != float64(2) || feature["distance2"] != float64(3) {
		t.Fatalf("chamfer feature = %#v, want createdLineId chamfer-line distances 2 and 3", feature)
	}
	if !containsAll(response.ChangedEntityIDs, "line-1", "line-2", "corner", "chamfer-p1", "chamfer-p2", "chamfer-line", "chamfer-1") {
		t.Fatalf("ChangedEntityIDs = %#v, missing chamfer affected IDs", response.ChangedEntityIDs)
	}
}

func TestServiceSubmitApplyChamferRejectsInvalidDistance(t *testing.T) {
	repo := &repositoryStub{
		submitState: &model.SubmitState{
			Version: 1,
			GraphState: []byte(`{
				"entities":{
					"line-1":{"id":"line-1","type":"line","startPointId":"p0","endPointId":"p1"},
					"line-2":{"id":"line-2","type":"line","startPointId":"p0","endPointId":"p2"}
				},
				"constraints":{},
				"dimensions":{},
				"groups":{}
			}`),
			MaterializedGeometry: []byte(`{"entities":{}}`),
			SolveStatus:          []byte(`{"status":"ok","degreesOfFreedom":0,"diagnostics":[]}`),
		},
	}
	service := NewServiceWithSolver(repo, &solverStub{})
	ctx := auth.ContextWithUserID(context.Background(), "user-id")

	response, err := service.Submit(ctx, "sketch-id", &model.SubmitOperationRequest{
		BaseVersion: 1,
		ClientOpID:  "client-op-id",
		Op: []byte(`{
			"type":"ApplyChamfer",
			"line1Id":"line-1",
			"line2Id":"line-2",
			"distance1":0,
			"distance2":3
		}`),
	})
	if err != nil {
		t.Fatalf("Submit returned error: %v", err)
	}
	if response.Accepted {
		t.Fatal("Submit accepted invalid chamfer")
	}
	if response.Rejection == nil || response.Rejection.Reason != "invalid_operation" {
		t.Fatalf("Rejection = %#v, want invalid_operation", response.Rejection)
	}
}

func containsAll(values []string, want ...string) bool {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		seen[value] = struct{}{}
	}
	for _, value := range want {
		if _, ok := seen[value]; !ok {
			return false
		}
	}
	return true
}
