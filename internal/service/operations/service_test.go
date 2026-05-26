package operations

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/dnonakolesax/cccad-locks/internal/auth"
	"github.com/dnonakolesax/cccad-locks/internal/model"
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
	service := NewService(repo)
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
