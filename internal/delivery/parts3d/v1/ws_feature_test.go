package v1

import (
	"encoding/json"
	"testing"

	"github.com/dnonakolesax/cccad-locks/internal/model"
	geometryv1 "github.com/dnonakolesax/cccad-locks/internal/proto/geometry/v1"
)

func TestPart3DSketchPlaneFromSketchPreservesXAxis(t *testing.T) {
	plane := part3DSketchPlaneFromSketch(model.SketchPlane{
		Origin: model.Vector3{X: 1, Y: 2, Z: 3},
		Normal: model.Vector3{X: 0, Y: 0, Z: 1},
		XAxis:  model.Vector3{X: 1, Y: 0, Z: 0},
	})

	if plane == nil {
		t.Fatal("plane is nil")
	}
	if plane.XAxis.X != 1 || plane.XAxis.Y != 0 || plane.XAxis.Z != 0 {
		t.Fatalf("xAxis = %#v, want stored sketch xAxis", plane.XAxis)
	}
	if plane.YAxis.X != 0 || plane.YAxis.Y != 1 || plane.YAxis.Z != 0 {
		t.Fatalf("yAxis = %#v, want normal cross xAxis", plane.YAxis)
	}
	if plane.Normal.Z != 1 {
		t.Fatalf("normal = %#v", plane.Normal)
	}
}

func TestEmptyPart3DSketchPlaneIsNotUsable(t *testing.T) {
	if isUsableSketchPlane(&part3DSketchPlane{}) {
		t.Fatal("empty sketch plane is usable")
	}
}

func TestSketchProfileFromStateBuildsOuterLoopCurves(t *testing.T) {
	profiles := json.RawMessage(`[
		{"id":"profile-1","outerLoop":{"entityIds":["l1","l2","l3","l4"]},"validForExtrude":true}
	]`)
	entities := json.RawMessage(`{
		"p1":{"id":"p1","type":"point","x":0,"y":0},
		"p2":{"id":"p2","type":"point","x":10,"y":0},
		"p3":{"id":"p3","type":"point","x":10,"y":5},
		"p4":{"id":"p4","type":"point","x":0,"y":5},
		"l1":{"id":"l1","type":"line","startPointId":"p1","endPointId":"p2"},
		"l2":{"id":"l2","type":"line","startPointId":"p2","endPointId":"p3"},
		"l3":{"id":"l3","type":"line","startPointId":"p3","endPointId":"p4"},
		"l4":{"id":"l4","type":"line","startPointId":"p4","endPointId":"p1"}
	}`)

	profile, err := sketchProfileFromState("profile-1", profiles, entities)
	if err != nil {
		t.Fatalf("sketchProfileFromState returned error: %v", err)
	}
	if profile.GetProfileId() != "profile-1" {
		t.Fatalf("profileId = %q", profile.GetProfileId())
	}
	if len(profile.GetOuterLoop()) != 4 {
		t.Fatalf("outer loop length = %d, want 4", len(profile.GetOuterLoop()))
	}
	if profile.GetOuterLoop()[0].GetLine() == nil {
		t.Fatalf("first curve = %#v, want line", profile.GetOuterLoop()[0])
	}
}

func TestCommitFromBuildResponseRejectsBodiesWithoutTopology(t *testing.T) {
	_, err := commitFromBuildResponse(
		"11111111-1111-1111-1111-111111111111",
		"22222222-2222-2222-2222-222222222222",
		"33333333-3333-3333-3333-333333333333",
		1,
		part3DFeaturePayload{Type: "extrude"},
		json.RawMessage(`{"type":"extrude"}`),
		&geometryv1.BuildFeatureResponse{
			Success: true,
			Bodies: []*geometryv1.BodyResult{{
				BodyId: "body-1",
			}},
		},
	)

	if err == nil {
		t.Fatal("commitFromBuildResponse returned nil error for body without topology")
	}
}

func TestBodyInputsFromGeometryUsesBrepArtifacts(t *testing.T) {
	response := &geometryv1.BuildFeatureResponse{
		Bodies: []*geometryv1.BodyResult{{
			BodyId: "body-1",
			Artifacts: []*geometryv1.ArtifactRef{
				{Kind: "glb", StorageKey: "body-1.glb"},
				{Kind: "brep", StorageKey: "body-1.brep"},
			},
		}},
	}

	inputs := bodyInputsFromGeometry(response)
	if len(inputs) != 1 {
		t.Fatalf("inputs length = %d, want 1", len(inputs))
	}
	if inputs[0].GetBodyId() != "body-1" {
		t.Fatalf("bodyId = %q", inputs[0].GetBodyId())
	}
	if inputs[0].GetBrep().GetStorageKey() != "body-1.brep" {
		t.Fatalf("brep storage key = %q", inputs[0].GetBrep().GetStorageKey())
	}
}

func TestCommitFromBuildResponsePersistsTopologyRefs(t *testing.T) {
	commit, err := commitFromBuildResponse(
		"11111111-1111-1111-1111-111111111111",
		"22222222-2222-2222-2222-222222222222",
		"33333333-3333-3333-3333-333333333333",
		1,
		part3DFeaturePayload{Type: "extrude"},
		json.RawMessage(`{"type":"extrude"}`),
		&geometryv1.BuildFeatureResponse{
			Success: true,
			Bodies: []*geometryv1.BodyResult{{
				BodyId: "body-1",
			}},
			Topology: &geometryv1.TopologySummary{
				Bodies: []*geometryv1.Body{{
					BodyId: "body-1",
					Shells: []*geometryv1.Shell{{
						ShellId: "shell-1",
						Faces: []*geometryv1.Face{{
							FaceId:      "face-1",
							SurfaceType: "plane",
						}},
					}},
				}},
			},
		},
	)
	if err != nil {
		t.Fatalf("commitFromBuildResponse returned error: %v", err)
	}
	if len(commit.Topology) != 3 {
		t.Fatalf("topology refs = %d, want 3", len(commit.Topology))
	}
}

func TestTopologyFromGeometryQualifiesRepeatedLoopAndEdgeIDs(t *testing.T) {
	refs := topologyFromGeometry(&geometryv1.TopologySummary{
		Bodies: []*geometryv1.Body{{
			BodyId: "body-1",
			Shells: []*geometryv1.Shell{{
				ShellId: "shell-1",
				Faces: []*geometryv1.Face{
					{
						FaceId: "face-1",
						Loops: []*geometryv1.Loop{{
							LoopId: "loop-1",
							Edges: []*geometryv1.Edge{{
								EdgeId: "edge-1",
							}},
						}},
					},
					{
						FaceId: "face-2",
						Loops: []*geometryv1.Loop{{
							LoopId: "loop-1",
							Edges: []*geometryv1.Edge{{
								EdgeId: "edge-1",
							}},
						}},
					},
				},
			}},
		}},
	})

	seen := map[string]struct{}{}
	for _, ref := range refs {
		key := ref.RefKind + "/" + ref.RefID
		if _, ok := seen[key]; ok {
			t.Fatalf("duplicate topology ref %q", key)
		}
		seen[key] = struct{}{}
	}
	if _, ok := seen["loop/face-1/loop-1"]; !ok {
		t.Fatal("missing qualified loop ref for face-1")
	}
	if _, ok := seen["loop/face-2/loop-1"]; !ok {
		t.Fatal("missing qualified loop ref for face-2")
	}
	if _, ok := seen["edge/face-1/loop-1/edge-1"]; !ok {
		t.Fatal("missing qualified edge ref for face-1")
	}
	if _, ok := seen["edge/face-2/loop-1/edge-1"]; !ok {
		t.Fatal("missing qualified edge ref for face-2")
	}
}

func TestTopologyFromGeometryDisambiguatesRepeatedEdgesInSameLoop(t *testing.T) {
	refs := topologyFromGeometry(&geometryv1.TopologySummary{
		Bodies: []*geometryv1.Body{{
			BodyId: "body-1",
			Shells: []*geometryv1.Shell{{
				ShellId: "shell-1",
				Faces: []*geometryv1.Face{{
					FaceId: "face-5",
					Loops: []*geometryv1.Loop{{
						LoopId: "loop-1",
						Edges: []*geometryv1.Edge{
							{EdgeId: "edge-13"},
							{EdgeId: "edge-13"},
						},
					}},
				}},
			}},
		}},
	})

	seen := map[string]struct{}{}
	for _, ref := range refs {
		key := ref.RefKind + "/" + ref.RefID
		if _, ok := seen[key]; ok {
			t.Fatalf("duplicate topology ref %q", key)
		}
		seen[key] = struct{}{}
	}
	if _, ok := seen["edge/face-5/loop-1/edge-13"]; !ok {
		t.Fatal("missing first edge ref")
	}
	if _, ok := seen["edge/face-5/loop-1/edge-13#2"]; !ok {
		t.Fatal("missing disambiguated second edge ref")
	}
}
