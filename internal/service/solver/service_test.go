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

func TestPreviewIncludesDefaultConstructionAxesAsLines(t *testing.T) {
	document := testSketchDocument()
	document.Entities["zero-point"] = easyjson.RawMessage(`{"id":"zero-point","type":"point","x":0,"y":0,"fixed":true,"isConstruction":true}`)
	document.Entities["x-axis-start"] = easyjson.RawMessage(`{"id":"x-axis-start","type":"point","x":-9999,"y":0,"fixed":true,"isConstruction":true}`)
	document.Entities["x-axis-end"] = easyjson.RawMessage(`{"id":"x-axis-end","type":"point","x":9999,"y":0,"fixed":true,"isConstruction":true}`)
	document.Entities["x-axis"] = easyjson.RawMessage(`{"id":"x-axis","type":"line","startPointId":"x-axis-start","endPointId":"x-axis-end","isConstruction":true}`)
	document.Entities["y-axis-start"] = easyjson.RawMessage(`{"id":"y-axis-start","type":"point","x":0,"y":-9999,"fixed":true,"isConstruction":true}`)
	document.Entities["y-axis-end"] = easyjson.RawMessage(`{"id":"y-axis-end","type":"point","x":0,"y":9999,"fixed":true,"isConstruction":true}`)
	document.Entities["y-axis"] = easyjson.RawMessage(`{"id":"y-axis","type":"line","startPointId":"y-axis-start","endPointId":"y-axis-end","isConstruction":true}`)
	client := &clientStub{}
	service := NewService(&sketchRepositoryStub{document: document}, client)

	_, err := service.Preview(context.Background(), "sketch-id", &model.SolvePreviewRequest{
		BaseVersion: 7,
		Intent:      easyjson.RawMessage(`{"type":"move_point","pointId":"p1","target":{"x":3,"y":4}}`),
	})
	if err != nil {
		t.Fatalf("Preview returned error: %v", err)
	}
	entities := client.applyIntentRequest.GetModel().GetEntities()
	seen := map[string]bool{}
	for _, entity := range entities {
		seen[entity.GetId()] = true
	}
	for _, id := range []string{"zero-point", "x-axis-start", "x-axis-end", "x-axis", "y-axis-start", "y-axis-end", "y-axis"} {
		if !seen[id] {
			t.Fatalf("solver model missing default construction entity %q; got %#v", id, entities)
		}
	}
}

func TestPreviewBuildsApplyFilletIntentRequest(t *testing.T) {
	client := &clientStub{}
	service := NewService(&sketchRepositoryStub{document: testSketchDocument()}, client)

	_, err := service.Preview(context.Background(), "sketch-id", &model.SolvePreviewRequest{
		BaseVersion: 7,
		Intent: easyjson.RawMessage(`{
			"type":"ApplyFillet",
			"featureId":"fillet-1",
			"line1Id":"line-1",
			"line2Id":"line-2",
			"cornerPointId":"corner",
			"createdPoint1Id":"fillet-p1",
			"createdPoint2Id":"fillet-p2",
			"createdArcId":"fillet-arc",
			"radius":2.5,
			"trim":true,
			"clockwise":true
		}`),
	})
	if err != nil {
		t.Fatalf("Preview returned error: %v", err)
	}
	intent := client.applyIntentRequest.GetIntent().GetApplyFillet()
	if intent == nil {
		t.Fatal("intent was not ApplyFillet")
	}
	if intent.GetLine1Id() != "line-1" || intent.GetLine2Id() != "line-2" ||
		intent.GetCreatedArcId() != "fillet-arc" || intent.GetRadius() != 2.5 ||
		!intent.GetTrim() || !intent.GetClockwise() {
		t.Fatalf("unexpected fillet intent: %#v", intent)
	}
}

func TestPreviewBuildsApplyChamferIntentRequest(t *testing.T) {
	client := &clientStub{}
	service := NewService(&sketchRepositoryStub{document: testSketchDocument()}, client)

	_, err := service.Preview(context.Background(), "sketch-id", &model.SolvePreviewRequest{
		BaseVersion: 7,
		Intent: easyjson.RawMessage(`{
			"type":"ApplyChamfer",
			"featureId":"chamfer-1",
			"line1Id":"line-1",
			"line2Id":"line-2",
			"cornerPointId":"corner",
			"createdPoint1Id":"chamfer-p1",
			"createdPoint2Id":"chamfer-p2",
			"createdArcId":"chamfer-arc",
			"distance1":2,
			"distance2":3,
			"trim":true
		}`),
	})
	if err != nil {
		t.Fatalf("Preview returned error: %v", err)
	}
	intent := client.applyIntentRequest.GetIntent().GetApplyChamfer()
	if intent == nil {
		t.Fatal("intent was not ApplyChamfer")
	}
	if intent.GetLine1Id() != "line-1" || intent.GetLine2Id() != "line-2" ||
		intent.GetCreatedArcId() != "chamfer-arc" || intent.GetDistance1() != 2 ||
		intent.GetDistance2() != 3 || !intent.GetTrim() {
		t.Fatalf("unexpected chamfer intent: %#v", intent)
	}
}

func TestPreviewBuildsUpdateAndEditIntentRequests(t *testing.T) {
	tests := map[string]struct {
		raw   easyjson.RawMessage
		check func(*testing.T, *solverv1.UserIntent)
	}{
		"update fillet": {
			raw: easyjson.RawMessage(`{"type":"UpdateFillet","featureId":"fillet-1","radius":4}`),
			check: func(t *testing.T, intent *solverv1.UserIntent) {
				t.Helper()
				if intent.GetUpdateFillet().GetFeatureId() != "fillet-1" || intent.GetUpdateFillet().GetRadius() != 4 {
					t.Fatalf("unexpected UpdateFillet intent: %#v", intent)
				}
			},
		},
		"update chamfer": {
			raw: easyjson.RawMessage(`{"type":"UpdateChamfer","featureId":"chamfer-1","distance1":2,"distance2":3}`),
			check: func(t *testing.T, intent *solverv1.UserIntent) {
				t.Helper()
				if intent.GetUpdateChamfer().GetFeatureId() != "chamfer-1" ||
					intent.GetUpdateChamfer().GetDistance1() != 2 ||
					intent.GetUpdateChamfer().GetDistance2() != 3 {
					t.Fatalf("unexpected UpdateChamfer intent: %#v", intent)
				}
			},
		},
		"split entity": {
			raw: easyjson.RawMessage(`{"type":"split_entity","entityId":"line-1","pickPoint":{"x":1,"y":2},"createdPointId":"p-new","createdEntityIds":["line-a","line-b"]}`),
			check: func(t *testing.T, intent *solverv1.UserIntent) {
				t.Helper()
				if intent.GetSplitEntity().GetEntityId() != "line-1" ||
					intent.GetSplitEntity().GetCreatedPointId() != "p-new" ||
					len(intent.GetSplitEntity().GetCreatedEntityIds()) != 2 {
					t.Fatalf("unexpected split_entity intent: %#v", intent)
				}
			},
		},
		"break entity": {
			raw: easyjson.RawMessage(`{"type":"break_entity_at_point","entityId":"line-1","pointId":"p1","createdEntityIds":["line-a"]}`),
			check: func(t *testing.T, intent *solverv1.UserIntent) {
				t.Helper()
				if intent.GetBreakEntityAtPoint().GetEntityId() != "line-1" ||
					intent.GetBreakEntityAtPoint().GetPointId() != "p1" {
					t.Fatalf("unexpected break_entity_at_point intent: %#v", intent)
				}
			},
		},
		"trim entity": {
			raw: easyjson.RawMessage(`{"type":"trim_entity","entityId":"line-1","pickPoint":{"x":1,"y":2},"boundaryEntityIds":["line-2"]}`),
			check: func(t *testing.T, intent *solverv1.UserIntent) {
				t.Helper()
				if intent.GetTrimEntity().GetEntityId() != "line-1" ||
					len(intent.GetTrimEntity().GetBoundaryEntityIds()) != 1 {
					t.Fatalf("unexpected trim_entity intent: %#v", intent)
				}
			},
		},
		"extend entity": {
			raw: easyjson.RawMessage(`{"type":"extend_entity","entityId":"line-1","endpoint":"end","target":{"x":5,"y":6},"targetEntityIds":["line-2"]}`),
			check: func(t *testing.T, intent *solverv1.UserIntent) {
				t.Helper()
				if intent.GetExtendEntity().GetEntityId() != "line-1" ||
					intent.GetExtendEntity().GetEndpoint() != "end" ||
					intent.GetExtendEntity().GetTarget().GetX() != 5 {
					t.Fatalf("unexpected extend_entity intent: %#v", intent)
				}
			},
		},
		"mirror entities": {
			raw: easyjson.RawMessage(`{"type":"mirror_entities","featureId":"mirror-1","sourceEntityIds":["line-1"],"mirrorLineId":"axis-1","createdEntityIds":["line-2"],"copy":true,"keepConstraints":true}`),
			check: func(t *testing.T, intent *solverv1.UserIntent) {
				t.Helper()
				if intent.GetMirrorEntities().GetFeatureId() != "mirror-1" ||
					intent.GetMirrorEntities().GetMirrorLineId() != "axis-1" ||
					len(intent.GetMirrorEntities().GetSourceEntityIds()) != 1 ||
					!intent.GetMirrorEntities().GetCopy() ||
					!intent.GetMirrorEntities().GetKeepConstraints() {
					t.Fatalf("unexpected mirror_entities intent: %#v", intent)
				}
			},
		},
		"linear pattern": {
			raw: easyjson.RawMessage(`{"type":"linear_pattern","featureId":"pattern-1","sourceEntityIds":["line-1"],"direction":{"x":1,"y":0},"spacing":5,"count":3,"createdEntityIds":["line-2","line-3"],"keepConstraints":true}`),
			check: func(t *testing.T, intent *solverv1.UserIntent) {
				t.Helper()
				if intent.GetLinearPattern().GetFeatureId() != "pattern-1" ||
					intent.GetLinearPattern().GetDirection().GetX() != 1 ||
					intent.GetLinearPattern().GetSpacing() != 5 ||
					intent.GetLinearPattern().GetCount() != 3 ||
					len(intent.GetLinearPattern().GetCreatedEntityIds()) != 2 {
					t.Fatalf("unexpected linear_pattern intent: %#v", intent)
				}
			},
		},
		"circular pattern": {
			raw: easyjson.RawMessage(`{"type":"circular_pattern","featureId":"pattern-1","sourceEntityIds":["line-1"],"centerPointId":"p1","totalAngleRad":6.283185307179586,"count":4,"createdEntityIds":["line-2"],"rotateInstances":true,"keepConstraints":true}`),
			check: func(t *testing.T, intent *solverv1.UserIntent) {
				t.Helper()
				if intent.GetCircularPattern().GetFeatureId() != "pattern-1" ||
					intent.GetCircularPattern().GetCenterPointId() != "p1" ||
					intent.GetCircularPattern().GetTotalAngleRad() == 0 ||
					intent.GetCircularPattern().GetCount() != 4 ||
					!intent.GetCircularPattern().GetRotateInstances() {
					t.Fatalf("unexpected circular_pattern intent: %#v", intent)
				}
			},
		},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			client := &clientStub{}
			service := NewService(&sketchRepositoryStub{document: testSketchDocument()}, client)
			_, err := service.Preview(context.Background(), "sketch-id", &model.SolvePreviewRequest{
				BaseVersion: 7,
				Intent:      tt.raw,
			})
			if err != nil {
				t.Fatalf("Preview returned error: %v", err)
			}
			tt.check(t, client.applyIntentRequest.GetIntent())
		})
	}
}

func TestSolutionPatchIncludesSolvedLine(t *testing.T) {
	patch, err := SolutionPatch(&solverv1.SketchSolution{
		Entities: []*solverv1.SolvedEntity{
			{
				Id: "line-1",
				Kind: &solverv1.SolvedEntity_Line{
					Line: &solverv1.SolvedLine{StartPointId: "p1", EndPointId: "p2"},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("SolutionPatch returned error: %v", err)
	}
	if string(patch) != `{"entities":{"line-1":{"id":"line-1","type":"line","startPointId":"p1","endPointId":"p2"}}}` {
		t.Fatalf("patch = %s", patch)
	}
}

func TestSolutionPatchIncludesZeroPointCoordinates(t *testing.T) {
	patch, err := SolutionPatch(&solverv1.SketchSolution{
		Entities: []*solverv1.SolvedEntity{
			{
				Id: "point-1",
				Kind: &solverv1.SolvedEntity_Point{
					Point: &solverv1.SolvedPoint{X: 0, Y: 0},
				},
			},
		},
	})
	if err != nil {
		t.Fatalf("SolutionPatch returned error: %v", err)
	}
	if string(patch) != `{"entities":{"point-1":{"id":"point-1","type":"point","x":0,"y":0}}}` {
		t.Fatalf("patch = %s", patch)
	}
}

func TestSolutionPatchIncludesProfiles(t *testing.T) {
	patch, err := SolutionPatch(&solverv1.SketchSolution{
		Profiles: []*solverv1.Profile{
			{
				Id:              "profile-1",
				OuterLoop:       &solverv1.ProfileLoop{EntityIds: []string{"l1", "l2", "l3", "l4"}},
				InnerLoops:      []*solverv1.ProfileLoop{{EntityIds: []string{"c1"}}},
				Area:            42.5,
				ValidForExtrude: true,
			},
		},
	})
	if err != nil {
		t.Fatalf("SolutionPatch returned error: %v", err)
	}
	want := `{"entities":{},"profiles":[{"id":"profile-1","outerLoop":{"entityIds":["l1","l2","l3","l4"]},"innerLoops":[{"entityIds":["c1"]}],"area":42.5,"validForExtrude":true}]}`
	if string(patch) != want {
		t.Fatalf("patch = %s, want %s", patch, want)
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
			"d1": []byte(
				`{"id":"d1","type":"distance","status":"active","driving":true,"refAId":"p1","refBId":"p2","value":10}`,
			),
		},
	}
}
