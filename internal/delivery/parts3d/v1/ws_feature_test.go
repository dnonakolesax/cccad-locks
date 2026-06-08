package v1

import (
	"encoding/json"
	"testing"

	"github.com/dnonakolesax/cccad-locks/internal/model"
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
