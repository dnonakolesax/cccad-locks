package v1

import (
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
