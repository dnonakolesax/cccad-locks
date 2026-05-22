package solver

import (
	"context"
	"testing"

	"github.com/dnonakolesax/cccad-locks/internal/model"
	solverv1 "github.com/dnonakolesax/cccad-locks/internal/proto/solver/v1"
	"github.com/mailru/easyjson"
)

type sketchRepositoryStub struct {
	document *model.SketchDocument
}

func (s *sketchRepositoryStub) Get(context.Context, string) (*model.SketchDocument, error) {
	return s.document, nil
}

type clientStub struct {
	applyIntentRequest *solverv1.ApplyIntentRequest
	analyzeRequest     *solverv1.AnalyzeRequest
}

func (c *clientStub) ApplyIntent(
	_ context.Context,
	request *solverv1.ApplyIntentRequest,
) (*solverv1.ApplyIntentResponse, error) {
	c.applyIntentRequest = request
	return &solverv1.ApplyIntentResponse{
		Status:            solverv1.SolveStatus_SOLVE_STATUS_OK,
		DegreesOfFreedom:  2,
		AffectedEntityIds: []string{"p1"},
		Solution: &solverv1.SketchSolution{
			Entities: []*solverv1.SolvedEntity{
				{
					Id: "p1",
					Kind: &solverv1.SolvedEntity_Point{
						Point: &solverv1.SolvedPoint{X: 3, Y: 4},
					},
				},
			},
		},
	}, nil
}

func (c *clientStub) Analyze(
	_ context.Context,
	request *solverv1.AnalyzeRequest,
) (*solverv1.AnalyzeResponse, error) {
	c.analyzeRequest = request
	return &solverv1.AnalyzeResponse{
		Status:           solverv1.SolveStatus_SOLVE_STATUS_FULLY_CONSTRAINED,
		DegreesOfFreedom: 0,
	}, nil
}

func TestPreviewBuildsApplyIntentRequest(t *testing.T) {
	client := &clientStub{}
	service := NewService(&sketchRepositoryStub{document: testSketchDocument()}, client)

	response, err := service.Preview(context.Background(), "sketch-id", &model.SolvePreviewRequest{
		BaseVersion: 7,
		Intent:      easyjson.RawMessage(`{"type":"move_point","pointId":"p1","target":{"x":3,"y":4}}`),
	})
	if err != nil {
		t.Fatalf("Preview returned error: %v", err)
	}
	if response.Status.Status != "ok" {
		t.Fatalf("Preview status = %q, want ok", response.Status.Status)
	}
	if client.applyIntentRequest == nil {
		t.Fatal("ApplyIntent was not called")
	}
	if len(client.applyIntentRequest.GetModel().GetEntities()) != 2 {
		t.Fatalf("entity count = %d, want 2", len(client.applyIntentRequest.GetModel().GetEntities()))
	}
	intent := client.applyIntentRequest.GetIntent().GetMovePoint()
	if intent == nil {
		t.Fatal("intent was not move_point")
	}
	if intent.GetPointId() != "p1" || intent.GetTarget().GetX() != 3 || intent.GetTarget().GetY() != 4 {
		t.Fatalf("unexpected intent: %#v", intent)
	}
}

func TestAnalyzeCallsSolverAnalyze(t *testing.T) {
	client := &clientStub{}
	service := NewService(&sketchRepositoryStub{document: testSketchDocument()}, client)

	response, err := service.Analyze(context.Background(), "sketch-id")
	if err != nil {
		t.Fatalf("Analyze returned error: %v", err)
	}
	if response.Status.Status != "fully_constrained" {
		t.Fatalf("Analyze status = %q, want fully_constrained", response.Status.Status)
	}
	if client.analyzeRequest == nil {
		t.Fatal("Analyze was not called")
	}
}

func testSketchDocument() *model.SketchDocument {
	return &model.SketchDocument{
		ID:      "sketch-id",
		Version: 7,
		Entities: map[string]easyjson.RawMessage{
			"p1": []byte(`{"id":"p1","type":"point","x":1,"y":2}`),
			"p2": []byte(`{"id":"p2","type":"point","x":4,"y":5}`),
		},
		Constraints: map[string]easyjson.RawMessage{
			"c1": []byte(`{"id":"c1","type":"coincident","status":"active","pointAId":"p1","pointBId":"p2"}`),
		},
		Dimensions: map[string]easyjson.RawMessage{
			"d1": []byte(`{"id":"d1","type":"distance","status":"active","driving":true,"refAId":"p1","refBId":"p2","value":10}`),
		},
	}
}
