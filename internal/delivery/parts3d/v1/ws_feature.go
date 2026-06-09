package v1

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/dnonakolesax/cccad-locks/internal/model"
	geometryv1 "github.com/dnonakolesax/cccad-locks/internal/proto/geometry/v1"
	"google.golang.org/protobuf/encoding/protojson"
)

type part3DFeatureIntent struct {
	MessageID         string          `json:"messageId"`
	Type              string          `json:"type"`
	PartID            string          `json:"partId"`
	DocumentVersion   int64           `json:"documentVersion"`
	ActorUserID       string          `json:"actorUserId"`
	ClientOperationID string          `json:"clientOperationId"`
	PreviewOnly       bool            `json:"previewOnly"`
	Feature           json.RawMessage `json:"feature"`
}

type part3DFeaturePayload struct {
	Type             string             `json:"type"`
	SketchID         string             `json:"sketchId"`
	ProfileID        string             `json:"profileId"`
	Depth            float64            `json:"depth"`
	Direction        string             `json:"direction"`
	Operation        string             `json:"operation"`
	TargetBodyID     string             `json:"targetBodyId"`
	ToolBodyIDs      []string           `json:"toolBodyIds"`
	Center           *part3DVec2        `json:"center"`
	Diameter         float64            `json:"diameter"`
	ThroughAll       bool               `json:"throughAll"`
	SourceFeatureIDs []string           `json:"sourceFeatureIds"`
	SourceBodyIDs    []string           `json:"sourceBodyIds"`
	Linear           *part3DLinear      `json:"linear"`
	Circular         *part3DCircular    `json:"circular"`
	EdgeRefs         []string           `json:"edgeRefs"`
	Radius           float64            `json:"radius"`
	Distance         float64            `json:"distance"`
	SketchPlane      *part3DSketchPlane `json:"sketchPlane"`
	SketchProfile    *geometryv1.SketchProfile
}

type part3DVec2 struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

type part3DVec3 struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
	Z float64 `json:"z"`
}

type part3DLinear struct {
	Direction part3DVec3 `json:"direction"`
	Count     int32      `json:"count"`
	Spacing   float64    `json:"spacing"`
}

type part3DCircular struct {
	Axis     part3DAxis3D `json:"axis"`
	Count    int32        `json:"count"`
	AngleDeg float64      `json:"angleDeg"`
}

type part3DAxis3D struct {
	Origin    part3DVec3 `json:"origin"`
	Direction part3DVec3 `json:"direction"`
}

type part3DSketchPlane struct {
	Kind   string     `json:"kind"`
	Origin part3DVec3 `json:"origin"`
	XAxis  part3DVec3 `json:"xAxis"`
	YAxis  part3DVec3 `json:"yAxis"`
	Normal part3DVec3 `json:"normal"`
}

func (h *Parts3DWSHandler) handleFeatureIntent(
	ctx context.Context,
	conn *part3DWSConnection,
	data []byte,
) error {
	if h.geometry == nil || h.repo == nil {
		return h.sendFeatureRejected(conn, "", "", []model.Diagnostic3D{{
			Code:     "PROCESSOR_UNAVAILABLE",
			Severity: "error",
			Message:  "3d feature processor is not configured",
		}})
	}

	var intent part3DFeatureIntent
	if err := json.Unmarshal(data, &intent); err != nil {
		return err
	}
	if len(intent.Feature) == 0 {
		return h.sendFeatureRejected(conn, intent.ClientOperationID, "", []model.Diagnostic3D{{
			Code:     "INVALID_FEATURE",
			Severity: "error",
			Message:  "feature is required",
		}})
	}

	var feature part3DFeaturePayload
	if err := json.Unmarshal(intent.Feature, &feature); err != nil {
		return h.sendFeatureRejected(conn, intent.ClientOperationID, "", []model.Diagnostic3D{{
			Code:     "INVALID_FEATURE",
			Severity: "error",
			Message:  err.Error(),
		}})
	}
	feature.Type = strings.TrimSpace(feature.Type)
	if !isKnownFeatureType(feature.Type) {
		return h.sendFeatureRejected(conn, intent.ClientOperationID, "", []model.Diagnostic3D{{
			Code:     "UNSUPPORTED_FEATURE_TYPE",
			Severity: "error",
			Message:  "unsupported feature type " + feature.Type,
		}})
	}
	if err := h.ensureSketchPlane(ctx, &feature); err != nil {
		return h.sendFeatureRejected(conn, intent.ClientOperationID, "", []model.Diagnostic3D{{
			Code:     "INVALID_SKETCH_PLANE",
			Severity: "error",
			Message:  err.Error(),
		}})
	}
	if err := h.ensureSketchProfile(ctx, &feature); err != nil {
		return h.sendFeatureRejected(conn, intent.ClientOperationID, "", []model.Diagnostic3D{{
			Code:     "INVALID_SKETCH_PROFILE",
			Severity: "error",
			Message:  err.Error(),
		}})
	}
	featureID := newUUID()
	documentVersion := intent.DocumentVersion + 1
	if documentVersion <= 0 {
		documentVersion = 1
	}

	response, err := h.callGeometryBuild(ctx, conn.partID, featureID, documentVersion, feature)
	if err != nil {
		diagnostics := []model.Diagnostic3D{{
			Code:      "GEOMETRY_BUILD_FAILED",
			Severity:  "error",
			Message:   err.Error(),
			FeatureID: featureID,
		}}
		_, commitErr := h.repo.CommitFeatureBuild(ctx, model.Feature3DCommit{
			FeatureID:       featureID,
			PartID:          conn.partID,
			SketchID:        feature.SketchID,
			Type:            feature.Type,
			Payload:         append(json.RawMessage(nil), intent.Feature...),
			CreatedBy:       safeUUID(conn.userID),
			DocumentVersion: documentVersion,
			Success:         false,
			Diagnostics:     diagnostics,
		})
		if commitErr != nil {
			return commitErr
		}
		return h.sendFeatureRejected(conn, intent.ClientOperationID, featureID, diagnostics)
	}

	if response == nil {
		return errors.New("geometry service returned nil build response")
	}
	diagnostics := diagnosticsFromGeometry(response.GetDiagnostics())
	if !response.GetSuccess() {
		_, commitErr := h.repo.CommitFeatureBuild(ctx, model.Feature3DCommit{
			FeatureID:       featureID,
			PartID:          conn.partID,
			SketchID:        feature.SketchID,
			Type:            feature.Type,
			Payload:         append(json.RawMessage(nil), intent.Feature...),
			CreatedBy:       safeUUID(conn.userID),
			DocumentVersion: documentVersion,
			Success:         false,
			Diagnostics:     diagnostics,
		})
		if commitErr != nil {
			return commitErr
		}
		return h.sendFeatureRejected(conn, intent.ClientOperationID, featureID, diagnostics)
	}

	if intent.PreviewOnly {
		return h.sendPreviewChanged(conn, intent, feature)
	}

	if needsTopologyFallback(response) {
		topologyResponse, topologyErr := h.geometry.GetTopology(ctx, &geometryv1.GetTopologyRequest{
			Context:        kernelContext(conn.partID, featureID, documentVersion),
			BodyIds:        bodyIDsFromGeometry(response),
			ExistingBodies: bodyInputsFromGeometry(response),
		})
		if topologyErr != nil {
			return h.sendFeatureRejected(conn, intent.ClientOperationID, featureID, []model.Diagnostic3D{{
				Code:      "TOPOLOGY_BUILD_FAILED",
				Severity:  "error",
				Message:   topologyErr.Error(),
				FeatureID: featureID,
			}})
		}
		if topologyResponse == nil || !topologyResponse.GetSuccess() {
			diagnostics := diagnosticsFromGeometry(nil)
			if topologyResponse != nil {
				diagnostics = diagnosticsFromGeometry(topologyResponse.GetDiagnostics())
			}
			if len(diagnostics) == 0 {
				diagnostics = []model.Diagnostic3D{{
					Code:      "TOPOLOGY_BUILD_FAILED",
					Severity:  "error",
					Message:   "geometry service did not return topology for created bodies",
					FeatureID: featureID,
				}}
			}
			return h.sendFeatureRejected(conn, intent.ClientOperationID, featureID, diagnostics)
		}
		response.Topology = topologyResponse.GetTopology()
	}

	commit, err := commitFromBuildResponse(conn.partID, featureID, safeUUID(conn.userID), documentVersion, feature, intent.Feature, response)
	if err != nil {
		return h.sendFeatureRejected(conn, intent.ClientOperationID, featureID, []model.Diagnostic3D{{
			Code:      "INVALID_GEOMETRY_RESULT",
			Severity:  "error",
			Message:   err.Error(),
			FeatureID: featureID,
		}})
	}
	result, err := h.repo.CommitFeatureBuild(ctx, commit)
	if err != nil {
		return h.sendFeatureRejected(conn, intent.ClientOperationID, featureID, []model.Diagnostic3D{{
			Code:      "PERSISTENCE_FAILED",
			Severity:  "error",
			Message:   err.Error(),
			FeatureID: featureID,
		}})
	}

	h.broadcastJSON(conn.partID, lifecycleMessage(conn, MsgPart3DFeatureAccepted, result, "accepted"))
	h.broadcastJSON(conn.partID, lifecycleMessage(conn, MsgPart3DFeatureCommitted, result, "committed"))
	return nil
}

func (h *Parts3DWSHandler) callGeometryBuild(
	ctx context.Context,
	partID string,
	featureID string,
	documentVersion int64,
	feature part3DFeaturePayload,
) (*geometryv1.BuildFeatureResponse, error) {
	kernelCtx := kernelContext(partID, featureID, documentVersion)
	output := &geometryv1.OutputOptions{
		ReturnTopology: true,
		WriteBrep:      true,
		WriteGlb:       true,
		WriteMeshJson:  true,
	}

	switch feature.Type {
	case "extrude":
		return h.geometry.BuildExtrude(ctx, &geometryv1.BuildExtrudeRequest{
			Context:     kernelCtx,
			FeatureId:   featureID,
			SketchId:    feature.SketchID,
			SketchPlane: sketchPlane(feature.SketchPlane),
			Profile:     sketchProfile(feature),
			Parameters: &geometryv1.ExtrudeParameters{
				Depth:        feature.Depth,
				Direction:    extrudeDirection(feature.Direction),
				Operation:    solidOperation(feature.Operation),
				TargetBodyId: feature.TargetBodyID,
			},
			Output: output,
		})
	case "hole":
		center := &geometryv1.Vec2{}
		if feature.Center != nil {
			center = &geometryv1.Vec2{X: feature.Center.X, Y: feature.Center.Y}
		}
		return h.geometry.BuildHole(ctx, &geometryv1.BuildHoleRequest{
			Context:     kernelCtx,
			FeatureId:   featureID,
			SketchId:    feature.SketchID,
			SketchPlane: sketchPlane(feature.SketchPlane),
			Center:      center,
			Parameters: &geometryv1.HoleParameters{
				Diameter:     feature.Diameter,
				Depth:        feature.Depth,
				ThroughAll:   feature.ThroughAll,
				Direction:    extrudeDirection(feature.Direction),
				TargetBodyId: feature.TargetBodyID,
			},
			Output: output,
		})
	case "boolean":
		return h.geometry.BuildBoolean(ctx, &geometryv1.BuildBooleanRequest{
			Context:      kernelCtx,
			FeatureId:    featureID,
			Operation:    booleanOperation(feature.Operation),
			TargetBodyId: feature.TargetBodyID,
			ToolBodyIds:  feature.ToolBodyIDs,
			Output:       output,
		})
	case "fillet":
		return h.geometry.BuildFillet(ctx, &geometryv1.BuildFilletRequest{
			Context:      kernelCtx,
			FeatureId:    featureID,
			TargetBodyId: feature.TargetBodyID,
			EdgeRefs:     feature.EdgeRefs,
			Radius:       feature.Radius,
			Output:       output,
		})
	case "chamfer":
		return h.geometry.BuildChamfer(ctx, &geometryv1.BuildChamferRequest{
			Context:      kernelCtx,
			FeatureId:    featureID,
			TargetBodyId: feature.TargetBodyID,
			EdgeRefs:     feature.EdgeRefs,
			Distance:     feature.Distance,
			Output:       output,
		})
	case "pattern":
		req := &geometryv1.BuildPatternRequest{
			Context:          kernelCtx,
			FeatureId:        featureID,
			SourceFeatureIds: feature.SourceFeatureIDs,
			SourceBodyIds:    feature.SourceBodyIDs,
			Output:           output,
		}
		if feature.Linear != nil {
			req.Parameters = &geometryv1.BuildPatternRequest_Linear{
				Linear: &geometryv1.LinearPatternParameters{
					Direction: vec3(feature.Linear.Direction),
					Count:     feature.Linear.Count,
					Spacing:   feature.Linear.Spacing,
				},
			}
		}
		if feature.Circular != nil {
			req.Parameters = &geometryv1.BuildPatternRequest_Circular{
				Circular: &geometryv1.CircularPatternParameters{
					Axis: &geometryv1.Axis3D{
						Origin:    vec3(feature.Circular.Axis.Origin),
						Direction: vec3(feature.Circular.Axis.Direction),
					},
					Count:    feature.Circular.Count,
					AngleRad: feature.Circular.AngleDeg * math.Pi / 180,
				},
			}
		}
		return h.geometry.BuildPattern(ctx, req)
	default:
		return nil, fmt.Errorf("unsupported feature type %q", feature.Type)
	}
}

func kernelContext(partID string, requestID string, documentVersion int64) *geometryv1.KernelContext {
	return &geometryv1.KernelContext{
		RequestId:       requestID,
		PartId:          partID,
		DocumentId:      partID,
		DocumentVersion: documentVersion,
		StoragePrefix:   "parts/" + partID + "/versions/" + fmt.Sprint(documentVersion) + "/",
	}
}

func sketchProfile(feature part3DFeaturePayload) *geometryv1.SketchProfile {
	if feature.SketchProfile != nil {
		return feature.SketchProfile
	}
	return &geometryv1.SketchProfile{ProfileId: feature.ProfileID}
}

func (h *Parts3DWSHandler) ensureSketchPlane(ctx context.Context, feature *part3DFeaturePayload) error {
	if feature == nil || !requiresSketchPlane(feature.Type) || isUsableSketchPlane(feature.SketchPlane) {
		return nil
	}
	sketchID := strings.TrimSpace(feature.SketchID)
	if sketchID == "" {
		return errors.New("sketchId is required")
	}

	plane, err := h.repo.GetSketchPlane(ctx, sketchID)
	if err != nil {
		return fmt.Errorf("load sketch plane: %w", err)
	}
	if plane == nil {
		return errors.New("sketch plane not found")
	}

	feature.SketchPlane = part3DSketchPlaneFromSketch(*plane)
	return nil
}

func isUsableSketchPlane(plane *part3DSketchPlane) bool {
	if plane == nil {
		return false
	}
	return isFiniteVec3(plane.Origin) &&
		isFiniteVec3(plane.XAxis) &&
		isFiniteVec3(plane.Normal) &&
		vec3LengthSquared(plane.XAxis) > 0 &&
		vec3LengthSquared(plane.Normal) > 0
}

func isFiniteVec3(value part3DVec3) bool {
	return isFiniteFloat(value.X) && isFiniteFloat(value.Y) && isFiniteFloat(value.Z)
}

func isFiniteFloat(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func vec3LengthSquared(value part3DVec3) float64 {
	return value.X*value.X + value.Y*value.Y + value.Z*value.Z
}

func requiresSketchPlane(featureType string) bool {
	switch featureType {
	case "extrude", "hole":
		return true
	default:
		return false
	}
}

func (h *Parts3DWSHandler) ensureSketchProfile(ctx context.Context, feature *part3DFeaturePayload) error {
	if feature == nil || feature.Type != "extrude" || usableSketchProfile(feature.SketchProfile) {
		return nil
	}
	sketchID := strings.TrimSpace(feature.SketchID)
	if sketchID == "" {
		return errors.New("sketchId is required")
	}
	profileID := strings.TrimSpace(feature.ProfileID)
	if profileID == "" {
		return errors.New("profileId is required")
	}

	profilesRaw, entitiesRaw, err := h.repo.GetSketchProfileData(ctx, sketchID)
	if err != nil {
		return fmt.Errorf("load sketch profile: %w", err)
	}

	profile, err := sketchProfileFromState(profileID, profilesRaw, entitiesRaw)
	if err != nil {
		return err
	}
	feature.SketchProfile = profile
	return nil
}

func usableSketchProfile(profile *geometryv1.SketchProfile) bool {
	return profile != nil && len(profile.GetOuterLoop()) != 0
}

type sketchStateProfile struct {
	ID        string                   `json:"id"`
	OuterLoop sketchStateProfileLoop   `json:"outerLoop"`
	InnerLoop []sketchStateProfileLoop `json:"innerLoops"`
}

type sketchStateProfileLoop struct {
	EntityIDs []string `json:"entityIds"`
}

type sketchStateEntity struct {
	ID            string  `json:"id"`
	Type          string  `json:"type"`
	X             float64 `json:"x"`
	Y             float64 `json:"y"`
	CenterPointID string  `json:"centerPointId"`
	Radius        float64 `json:"radius"`
	StartPointID  string  `json:"startPointId"`
	EndPointID    string  `json:"endPointId"`
	Clockwise     bool    `json:"clockwise"`
}

func sketchProfileFromState(
	profileID string,
	profilesRaw json.RawMessage,
	entitiesRaw json.RawMessage,
) (*geometryv1.SketchProfile, error) {
	var profiles []sketchStateProfile
	if err := json.Unmarshal(profilesRaw, &profiles); err != nil {
		return nil, fmt.Errorf("decode sketch profiles: %w", err)
	}
	var entities map[string]sketchStateEntity
	if err := json.Unmarshal(entitiesRaw, &entities); err != nil {
		return nil, fmt.Errorf("decode sketch entities: %w", err)
	}

	for _, profile := range profiles {
		if profile.ID != profileID {
			continue
		}
		outerLoop, err := profileCurves(profile.OuterLoop.EntityIDs, entities)
		if err != nil {
			return nil, err
		}
		innerLoops := make([]*geometryv1.ProfileLoop, 0, len(profile.InnerLoop))
		for _, loop := range profile.InnerLoop {
			curves, loopErr := profileCurves(loop.EntityIDs, entities)
			if loopErr != nil {
				return nil, loopErr
			}
			innerLoops = append(innerLoops, &geometryv1.ProfileLoop{Curves: curves})
		}
		return &geometryv1.SketchProfile{
			ProfileId:  profile.ID,
			OuterLoop:  outerLoop,
			InnerLoops: innerLoops,
		}, nil
	}

	return nil, fmt.Errorf("profile %q not found", profileID)
}

func profileCurves(entityIDs []string, entities map[string]sketchStateEntity) ([]*geometryv1.ProfileCurve, error) {
	curves := make([]*geometryv1.ProfileCurve, 0, len(entityIDs))
	for _, entityID := range entityIDs {
		entity, ok := entities[entityID]
		if !ok {
			return nil, fmt.Errorf("profile entity %q not found", entityID)
		}
		curve, err := profileCurve(entity, entities)
		if err != nil {
			return nil, err
		}
		curves = append(curves, curve)
	}
	return curves, nil
}

func profileCurve(
	entity sketchStateEntity,
	entities map[string]sketchStateEntity,
) (*geometryv1.ProfileCurve, error) {
	switch entity.Type {
	case "line":
		start, err := point2(entity.StartPointID, entities)
		if err != nil {
			return nil, err
		}
		end, err := point2(entity.EndPointID, entities)
		if err != nil {
			return nil, err
		}
		return &geometryv1.ProfileCurve{
			CurveId: entity.ID,
			Curve: &geometryv1.ProfileCurve_Line{
				Line: &geometryv1.LineSegment2D{Start: start, End: end},
			},
		}, nil
	case "circle":
		center, err := point2(entity.CenterPointID, entities)
		if err != nil {
			return nil, err
		}
		return &geometryv1.ProfileCurve{
			CurveId: entity.ID,
			Curve: &geometryv1.ProfileCurve_Circle{
				Circle: &geometryv1.Circle2D{Center: center, Radius: entity.Radius},
			},
		}, nil
	case "arc":
		centerEntity, ok := entities[entity.CenterPointID]
		if !ok {
			return nil, fmt.Errorf("arc center point %q not found", entity.CenterPointID)
		}
		center := &geometryv1.Vec2{X: centerEntity.X, Y: centerEntity.Y}
		start, err := point2(entity.StartPointID, entities)
		if err != nil {
			return nil, err
		}
		end, err := point2(entity.EndPointID, entities)
		if err != nil {
			return nil, err
		}
		return &geometryv1.ProfileCurve{
			CurveId: entity.ID,
			Curve: &geometryv1.ProfileCurve_Arc{
				Arc: &geometryv1.ArcSegment2D{
					Center:        center,
					Radius:        entity.Radius,
					StartAngleRad: math.Atan2(start.GetY()-center.GetY(), start.GetX()-center.GetX()),
					EndAngleRad:   math.Atan2(end.GetY()-center.GetY(), end.GetX()-center.GetX()),
					Clockwise:     entity.Clockwise,
				},
			},
		}, nil
	default:
		return nil, fmt.Errorf("profile entity %q has unsupported type %q", entity.ID, entity.Type)
	}
}

func point2(pointID string, entities map[string]sketchStateEntity) (*geometryv1.Vec2, error) {
	point, ok := entities[pointID]
	if !ok {
		return nil, fmt.Errorf("point %q not found", pointID)
	}
	if point.Type != "point" {
		return nil, fmt.Errorf("entity %q is %q, want point", pointID, point.Type)
	}
	return &geometryv1.Vec2{X: point.X, Y: point.Y}, nil
}

func part3DSketchPlaneFromSketch(plane model.SketchPlane) *part3DSketchPlane {
	xAxis := part3DVec3{X: plane.XAxis.X, Y: plane.XAxis.Y, Z: plane.XAxis.Z}
	normal := part3DVec3{X: plane.Normal.X, Y: plane.Normal.Y, Z: plane.Normal.Z}

	return &part3DSketchPlane{
		Kind:   "custom",
		Origin: part3DVec3{X: plane.Origin.X, Y: plane.Origin.Y, Z: plane.Origin.Z},
		XAxis:  xAxis,
		YAxis:  cross(normal, xAxis),
		Normal: normal,
	}
}

func commitFromBuildResponse(
	partID string,
	featureID string,
	createdBy string,
	documentVersion int64,
	feature part3DFeaturePayload,
	rawFeature json.RawMessage,
	response *geometryv1.BuildFeatureResponse,
) (model.Feature3DCommit, error) {
	topology := topologyFromGeometry(response.GetTopology())
	if len(response.GetBodies()) != 0 && len(topology) == 0 {
		return model.Feature3DCommit{}, errors.New("geometry service returned bodies without topology")
	}
	commit := model.Feature3DCommit{
		FeatureID:       featureID,
		PartID:          partID,
		SketchID:        feature.SketchID,
		Type:            feature.Type,
		Payload:         append(json.RawMessage(nil), rawFeature...),
		CreatedBy:       createdBy,
		DocumentVersion: documentVersion,
		Success:         response.GetSuccess(),
		Diagnostics:     diagnosticsFromGeometry(response.GetDiagnostics()),
		Bodies:          bodiesFromGeometry(featureID, response),
		Representations: representationsFromGeometry(response),
		Topology:        topology,
	}
	return commit, nil
}

func needsTopologyFallback(response *geometryv1.BuildFeatureResponse) bool {
	return len(response.GetBodies()) != 0 && len(topologyFromGeometry(response.GetTopology())) == 0
}

func bodyIDsFromGeometry(response *geometryv1.BuildFeatureResponse) []string {
	bodyIDs := make([]string, 0, len(response.GetBodies()))
	for _, body := range response.GetBodies() {
		if body.GetBodyId() != "" {
			bodyIDs = append(bodyIDs, body.GetBodyId())
		}
	}
	return bodyIDs
}

func bodyInputsFromGeometry(response *geometryv1.BuildFeatureResponse) []*geometryv1.BodyInput {
	inputs := make([]*geometryv1.BodyInput, 0, len(response.GetBodies()))
	for _, body := range response.GetBodies() {
		bodyID := body.GetBodyId()
		brep := brepArtifact(body.GetArtifacts())
		if bodyID == "" || brep == nil {
			continue
		}
		inputs = append(inputs, &geometryv1.BodyInput{
			BodyId: bodyID,
			Brep:   brep,
		})
	}
	return inputs
}

func brepArtifact(artifacts []*geometryv1.ArtifactRef) *geometryv1.ArtifactRef {
	for _, artifact := range artifacts {
		if artifact.GetKind() == "brep" {
			return artifact
		}
	}
	return nil
}

func bodiesFromGeometry(featureID string, response *geometryv1.BuildFeatureResponse) []model.Body3DCommit {
	byID := map[string]model.Body3DCommit{}
	for _, body := range response.GetTopology().GetBodies() {
		if body.GetBodyId() == "" {
			continue
		}
		byID[body.GetBodyId()] = model.Body3DCommit{
			ID:                 body.GetBodyId(),
			Name:               "Body " + body.GetBodyId(),
			CreatedByFeatureID: featureID,
			StableRef:          body.GetStableRef(),
			Active:             true,
		}
	}
	for _, body := range response.GetBodies() {
		if body.GetBodyId() == "" {
			continue
		}
		item := byID[body.GetBodyId()]
		item.ID = body.GetBodyId()
		item.Name = "Body " + body.GetBodyId()
		item.Active = true
		item.CreatedByFeatureID = body.GetCreatedByFeatureId()
		if item.CreatedByFeatureID == "" {
			item.CreatedByFeatureID = featureID
		}
		byID[body.GetBodyId()] = item
	}

	result := make([]model.Body3DCommit, 0, len(byID))
	for _, body := range byID {
		result = append(result, body)
	}
	return result
}

func representationsFromGeometry(response *geometryv1.BuildFeatureResponse) []model.Representation3DCommit {
	result := make([]model.Representation3DCommit, 0)
	for _, body := range response.GetBodies() {
		for _, artifact := range body.GetArtifacts() {
			result = append(result, representationFromArtifact(body.GetBodyId(), artifact))
		}
	}
	for _, artifact := range response.GetArtifacts() {
		result = append(result, representationFromArtifact("", artifact))
	}
	return result
}

func representationFromArtifact(bodyID string, artifact *geometryv1.ArtifactRef) model.Representation3DCommit {
	if artifact == nil {
		return model.Representation3DCommit{}
	}
	return model.Representation3DCommit{
		BodyID:      bodyID,
		Kind:        artifact.GetKind(),
		StorageKey:  artifact.GetStorageKey(),
		ContentType: artifact.GetContentType(),
		SizeBytes:   artifact.GetSizeBytes(),
		SHA256:      artifact.GetSha256(),
	}
}

func topologyFromGeometry(topology *geometryv1.TopologySummary) []model.TopologyRef3DCommit {
	if topology == nil {
		return nil
	}
	result := make([]model.TopologyRef3DCommit, 0)
	refs := newTopologyRefAllocator()
	for _, body := range topology.GetBodies() {
		bodyID := body.GetBodyId()
		if bodyID == "" {
			continue
		}
		result = append(result, model.TopologyRef3DCommit{
			BodyID:    bodyID,
			RefKind:   "body",
			RefID:     bodyID,
			StableRef: body.GetStableRef(),
		})
		for _, shell := range body.GetShells() {
			shellRefID := refs.unique(bodyID, "shell", shell.GetShellId())
			result = append(result, model.TopologyRef3DCommit{
				BodyID:      bodyID,
				RefKind:     "shell",
				RefID:       shellRefID,
				StableRef:   shell.GetStableRef(),
				ParentRefID: bodyID,
			})
			for _, face := range shell.GetFaces() {
				faceRefID := refs.unique(bodyID, "face", face.GetFaceId())
				payload := json.RawMessage(`{}`)
				facePayload := map[string]json.RawMessage{}
				if face.GetPlane() != nil {
					if plane, err := protojson.Marshal(face.GetPlane()); err == nil {
						facePayload["plane"] = plane
					}
				}
				if face.GetCylinder() != nil {
					if cylinder, err := protojson.Marshal(face.GetCylinder()); err == nil {
						facePayload["cylinder"] = cylinder
					}
				}
				if len(facePayload) != 0 {
					payload, _ = json.Marshal(facePayload)
				}
				result = append(result, model.TopologyRef3DCommit{
					BodyID:             bodyID,
					RefKind:            "face",
					RefID:              faceRefID,
					StableRef:          face.GetStableRef(),
					ParentRefID:        shellRefID,
					SurfaceOrCurveType: face.GetSurfaceType(),
					Payload:            payload,
				})
				for _, loop := range face.GetLoops() {
					loopRefID := refs.unique(bodyID, "loop", topologyPath(faceRefID, loop.GetLoopId()))
					loopPayload, _ := json.Marshal(map[string]any{
						"originalLoopId": loop.GetLoopId(),
						"role":           loop.GetRole(),
						"closed":         loop.GetClosed(),
					})
					result = append(result, model.TopologyRef3DCommit{
						BodyID:      bodyID,
						RefKind:     "loop",
						RefID:       loopRefID,
						StableRef:   loop.GetStableRef(),
						ParentRefID: faceRefID,
						Payload:     loopPayload,
					})
					for _, edge := range loop.GetEdges() {
						edgePayload := map[string]any{
							"originalEdgeId": edge.GetEdgeId(),
							"startVertexId":  edge.GetStartVertexId(),
							"endVertexId":    edge.GetEndVertexId(),
							"orientation":    edge.GetOrientation(),
						}
						if edge.GetCircle() != nil {
							if circle, err := protojson.Marshal(edge.GetCircle()); err == nil {
								edgePayload["circle"] = json.RawMessage(circle)
							}
						}
						payload, _ := json.Marshal(edgePayload)
						result = append(result, model.TopologyRef3DCommit{
							BodyID:             bodyID,
							RefKind:            "edge",
							RefID:              refs.unique(bodyID, "edge", topologyPath(loopRefID, edge.GetEdgeId())),
							StableRef:          edge.GetStableRef(),
							ParentRefID:        loopRefID,
							SurfaceOrCurveType: edge.GetCurveType(),
							Payload:            payload,
						})
					}
				}
			}
		}
		for _, vertex := range body.GetVertices() {
			payload := json.RawMessage(`{}`)
			if vertex.GetPoint() != nil {
				if point, err := protojson.Marshal(vertex.GetPoint()); err == nil {
					payload, _ = json.Marshal(map[string]json.RawMessage{"point": point})
				}
			}
			vertexRefID := refs.unique(bodyID, "vertex", vertex.GetVertexId())
			result = append(result, model.TopologyRef3DCommit{
				BodyID:    bodyID,
				RefKind:   "vertex",
				RefID:     vertexRefID,
				StableRef: vertex.GetStableRef(),
				Payload:   payload,
			})
		}
	}
	return result
}

type topologyRefAllocator struct {
	seen map[string]int
}

func newTopologyRefAllocator() *topologyRefAllocator {
	return &topologyRefAllocator{seen: map[string]int{}}
}

func (a *topologyRefAllocator) unique(bodyID string, refKind string, refID string) string {
	if refID == "" {
		refID = refKind
	}
	key := bodyID + "\x00" + refKind + "\x00" + refID
	a.seen[key]++
	if a.seen[key] == 1 {
		return refID
	}
	return fmt.Sprintf("%s#%d", refID, a.seen[key])
}

func topologyPath(parentID string, refID string) string {
	if parentID == "" {
		return refID
	}
	if refID == "" {
		return parentID
	}
	return parentID + "/" + refID
}

func diagnosticsFromGeometry(diagnostics []*geometryv1.Diagnostic) []model.Diagnostic3D {
	result := make([]model.Diagnostic3D, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		result = append(result, model.Diagnostic3D{
			Code:      diagnostic.GetCode(),
			Severity:  diagnostic.GetSeverity(),
			Message:   diagnostic.GetMessage(),
			FeatureID: diagnostic.GetFeatureId(),
			BodyID:    diagnostic.GetBodyId(),
		})
	}
	return result
}

func lifecycleMessage(
	conn *part3DWSConnection,
	messageType string,
	result *model.Feature3DCommitResult,
	status string,
) map[string]any {
	return map[string]any{
		"messageId":       newUUID(),
		"type":            messageType,
		"partId":          conn.partID,
		"documentVersion": result.DocumentVersion,
		"actorUserId":     conn.userID,
		"timestamp":       time.Now().UTC().Format(time.RFC3339Nano),
		"featureId":       result.FeatureID,
		"orderIndex":      result.OrderIndex,
		"status":          status,
	}
}

func (h *Parts3DWSHandler) sendFeatureRejected(
	conn *part3DWSConnection,
	clientOperationID string,
	featureID string,
	diagnostics []model.Diagnostic3D,
) error {
	msg := map[string]any{
		"messageId":         newUUID(),
		"type":              MsgPart3DFeatureRejected,
		"partId":            conn.partID,
		"documentVersion":   int64(0),
		"actorUserId":       conn.userID,
		"timestamp":         time.Now().UTC().Format(time.RFC3339Nano),
		"clientOperationId": clientOperationID,
		"diagnostics":       diagnostics,
	}
	if featureID != "" {
		msg["featureId"] = featureID
	}
	h.broadcastJSON(conn.partID, msg)
	return nil
}

func (h *Parts3DWSHandler) sendPreviewChanged(
	conn *part3DWSConnection,
	intent part3DFeatureIntent,
	feature part3DFeaturePayload,
) error {
	h.broadcastJSON(conn.partID, map[string]any{
		"messageId":         newUUID(),
		"type":              MsgPart3DPreviewChanged,
		"partId":            conn.partID,
		"documentVersion":   intent.DocumentVersion,
		"actorUserId":       conn.userID,
		"timestamp":         time.Now().UTC().Format(time.RFC3339Nano),
		"clientOperationId": intent.ClientOperationID,
		"feature":           feature,
	})
	return nil
}

func (h *Parts3DWSHandler) broadcastJSON(partID string, value any) {
	body, err := json.Marshal(value)
	if err != nil {
		h.logger.Warn("encode 3d websocket message failed", "error", err)
		return
	}
	h.broadcast(partID, body)
}

func part3DErrorMessage(partID, code, message string) []byte {
	body, _ := json.Marshal(map[string]any{
		"messageId": newUUID(),
		"type":      "error",
		"partId":    partID,
		"timestamp": time.Now().UTC().Format(time.RFC3339Nano),
		"error": map[string]string{
			"code":    code,
			"message": message,
		},
	})
	return body
}

func sketchPlane(plane *part3DSketchPlane) *geometryv1.SketchPlane {
	if plane == nil {
		return nil
	}
	return &geometryv1.SketchPlane{
		Kind:   plane.Kind,
		Origin: vec3(plane.Origin),
		XAxis:  vec3(plane.XAxis),
		YAxis:  vec3(plane.YAxis),
		Normal: vec3(plane.Normal),
	}
}

func vec3(value part3DVec3) *geometryv1.Vec3 {
	return &geometryv1.Vec3{X: value.X, Y: value.Y, Z: value.Z}
}

func cross(a, b part3DVec3) part3DVec3 {
	return part3DVec3{
		X: a.Y*b.Z - a.Z*b.Y,
		Y: a.Z*b.X - a.X*b.Z,
		Z: a.X*b.Y - a.Y*b.X,
	}
}

func extrudeDirection(value string) geometryv1.ExtrudeDirection {
	switch value {
	case "forward":
		return geometryv1.ExtrudeDirection_EXTRUDE_DIRECTION_FORWARD
	case "backward":
		return geometryv1.ExtrudeDirection_EXTRUDE_DIRECTION_BACKWARD
	case "symmetric":
		return geometryv1.ExtrudeDirection_EXTRUDE_DIRECTION_SYMMETRIC
	case "through_all":
		return geometryv1.ExtrudeDirection_EXTRUDE_DIRECTION_THROUGH_ALL
	default:
		return geometryv1.ExtrudeDirection_EXTRUDE_DIRECTION_UNSPECIFIED
	}
}

func solidOperation(value string) geometryv1.SolidOperation {
	switch value {
	case "new_body":
		return geometryv1.SolidOperation_SOLID_OPERATION_NEW_BODY
	case "join":
		return geometryv1.SolidOperation_SOLID_OPERATION_JOIN
	case "cut":
		return geometryv1.SolidOperation_SOLID_OPERATION_CUT
	case "intersect":
		return geometryv1.SolidOperation_SOLID_OPERATION_INTERSECT
	default:
		return geometryv1.SolidOperation_SOLID_OPERATION_UNSPECIFIED
	}
}

func booleanOperation(value string) geometryv1.BooleanOperation {
	switch value {
	case "unite":
		return geometryv1.BooleanOperation_BOOLEAN_OPERATION_UNITE
	case "subtract":
		return geometryv1.BooleanOperation_BOOLEAN_OPERATION_SUBTRACT
	case "intersect":
		return geometryv1.BooleanOperation_BOOLEAN_OPERATION_INTERSECT
	default:
		return geometryv1.BooleanOperation_BOOLEAN_OPERATION_UNSPECIFIED
	}
}

func isKnownFeatureType(value string) bool {
	switch value {
	case "extrude", "hole", "boolean", "fillet", "chamfer", "pattern":
		return true
	default:
		return false
	}
}

func newUUID() string {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		now := time.Now().UnixNano()
		return fmt.Sprintf("00000000-0000-4000-8000-%012x", now&0xffffffffffff)
	}
	id[6] = (id[6] & 0x0f) | 0x40
	id[8] = (id[8] & 0x3f) | 0x80

	var encoded [36]byte
	hex.Encode(encoded[0:8], id[0:4])
	encoded[8] = '-'
	hex.Encode(encoded[9:13], id[4:6])
	encoded[13] = '-'
	hex.Encode(encoded[14:18], id[6:8])
	encoded[18] = '-'
	hex.Encode(encoded[19:23], id[8:10])
	encoded[23] = '-'
	hex.Encode(encoded[24:36], id[10:16])
	return string(encoded[:])
}

func safeUUID(value string) string {
	if isValidUUID(value) {
		return value
	}
	return ""
}
