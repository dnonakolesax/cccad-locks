package model

import "encoding/json"

type Feature3D struct {
	ID         string          `json:"id"`
	PartID     string          `json:"partId"`
	SketchID   string          `json:"sketchId,omitempty"`
	Type       string          `json:"type"`
	Payload    json.RawMessage `json:"payload"`
	OrderIndex int             `json:"orderIndex"`
	Suppressed bool            `json:"suppressed"`
	CreatedBy  string          `json:"createdBy,omitempty"`
	CreatedAt  string          `json:"createdAt"`
	UpdatedAt  string          `json:"updatedAt,omitempty"`
}

type Feature3DList struct {
	Features []Feature3D `json:"features"`
}

type Body3D struct {
	ID                 string `json:"id"`
	PartID             string `json:"partId"`
	Name               string `json:"name"`
	Active             bool   `json:"active"`
	CreatedByFeatureID string `json:"createdByFeatureId,omitempty"`
	StableRef          string `json:"stableRef,omitempty"`
	CreatedAt          string `json:"createdAt,omitempty"`
	UpdatedAt          string `json:"updatedAt,omitempty"`
}

type Body3DList struct {
	Bodies []Body3D `json:"bodies"`
}

type TopologySummary3D struct {
	Bodies []TopologyBody3D `json:"bodies"`
}

type TopologyBody3D struct {
	BodyID    string             `json:"bodyId"`
	StableRef string             `json:"stableRef,omitempty"`
	Shells    []TopologyShell3D  `json:"shells"`
	Vertices  []TopologyVertex3D `json:"vertices,omitempty"`
}

type TopologyShell3D struct {
	ShellID   string           `json:"shellId"`
	StableRef string           `json:"stableRef,omitempty"`
	Faces     []TopologyFace3D `json:"faces"`
}

type TopologyFace3D struct {
	FaceID      string           `json:"faceId"`
	StableRef   string           `json:"stableRef,omitempty"`
	SurfaceType string           `json:"surfaceType,omitempty"`
	Plane       *SketchPlane3D   `json:"plane,omitempty"`
	Loops       []TopologyLoop3D `json:"loops"`
}

type TopologyLoop3D struct {
	LoopID    string           `json:"loopId"`
	StableRef string           `json:"stableRef,omitempty"`
	Edges     []TopologyEdge3D `json:"edges"`
}

type TopologyEdge3D struct {
	EdgeID        string `json:"edgeId"`
	StableRef     string `json:"stableRef,omitempty"`
	CurveType     string `json:"curveType,omitempty"`
	StartVertexID string `json:"startVertexId,omitempty"`
	EndVertexID   string `json:"endVertexId,omitempty"`
}

type TopologyVertex3D struct {
	VertexID  string   `json:"vertexId"`
	StableRef string   `json:"stableRef,omitempty"`
	Point     *Vector3 `json:"point,omitempty"`
}

type SketchPlane3D struct {
	Kind   string  `json:"kind"`
	Origin Vector3 `json:"origin"`
	XAxis  Vector3 `json:"xAxis"`
	YAxis  Vector3 `json:"yAxis"`
	Normal Vector3 `json:"normal"`
}

type FacePlane3D struct {
	SurfaceType string         `json:"surfaceType"`
	Plane       *SketchPlane3D `json:"plane,omitempty"`
	Diagnostics []Diagnostic3D `json:"diagnostics,omitempty"`
}

type Diagnostic3D struct {
	Code      string `json:"code"`
	Severity  string `json:"severity"`
	Message   string `json:"message"`
	FeatureID string `json:"featureId,omitempty"`
	BodyID    string `json:"bodyId,omitempty"`
}

type Feature3DCommit struct {
	FeatureID       string
	PartID          string
	SketchID        string
	Type            string
	Payload         json.RawMessage
	Suppressed      bool
	CreatedBy       string
	DocumentVersion int64
	Success         bool
	Diagnostics     []Diagnostic3D
	Bodies          []Body3DCommit
	Representations []Representation3DCommit
	Topology        []TopologyRef3DCommit
}

type Feature3DCommitResult struct {
	FeatureID       string
	OrderIndex      int
	DocumentVersion int64
}

type Body3DCommit struct {
	ID                 string
	Name               string
	CreatedByFeatureID string
	StableRef          string
	Active             bool
}

type Representation3DCommit struct {
	BodyID      string
	Kind        string
	StorageKey  string
	ContentType string
	SizeBytes   int64
	SHA256      string
}

type TopologyRef3DCommit struct {
	BodyID             string
	RefKind            string
	RefID              string
	StableRef          string
	ParentRefID        string
	SurfaceOrCurveType string
	Payload            json.RawMessage
}
