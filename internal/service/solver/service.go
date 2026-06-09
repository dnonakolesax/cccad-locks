package solver

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"

	"github.com/dnonakolesax/cccad-locks/internal/model"
	solverv1 "github.com/dnonakolesax/cccad-locks/internal/proto/solver/v1"
	"github.com/mailru/easyjson"
)

const (
	defaultSolverTolerance     = 1e-6
	defaultSolverMaxIterations = 100
)

var errSkippedModelItem = errors.New("skipped model item")

type SketchRepository interface {
	Get(ctx context.Context, sketchID string) (*model.SketchDocument, error)
}

type Client interface {
	ApplyIntent(ctx context.Context, req *solverv1.ApplyIntentRequest) (*solverv1.ApplyIntentResponse, error)
	Analyze(ctx context.Context, req *solverv1.AnalyzeRequest) (*solverv1.AnalyzeResponse, error)
}

type Service struct {
	sketches SketchRepository
	client   Client
}

func NewService(sketches SketchRepository, client Client) *Service {
	return &Service{
		sketches: sketches,
		client:   client,
	}
}

func (s *Service) Preview(
	ctx context.Context,
	sketchID string,
	request *model.SolvePreviewRequest,
) (*model.SolvePreviewResponse, error) {
	sketchID = strings.TrimSpace(sketchID)
	if sketchID == "" {
		return nil, errors.New("sketchID is required")
	}
	if request == nil {
		return nil, errors.New("request is required")
	}
	if request.BaseVersion < 0 {
		return nil, errors.New("baseVersion must be greater than or equal to 0")
	}
	if len(request.Intent) == 0 {
		return nil, errors.New("intent is required")
	}

	document, err := s.loadDocument(ctx, sketchID)
	if err != nil {
		return nil, err
	}
	if document.Version != request.BaseVersion {
		return nil, fmt.Errorf(
			"baseVersion %d does not match current sketch version %d",
			request.BaseVersion,
			document.Version,
		)
	}

	solverRequest, err := buildApplyIntentRequest(document, request)
	if err != nil {
		return nil, err
	}

	result, err := s.client.ApplyIntent(ctx, solverRequest)
	if err != nil {
		return nil, fmt.Errorf("apply solver intent: %w", err)
	}
	if result == nil {
		return nil, errors.New("solver returned nil apply intent response")
	}

	patch, err := solutionPatch(result.GetSolution())
	if err != nil {
		return nil, err
	}

	return &model.SolvePreviewResponse{
		Status:            solveStatusInfo(result.GetStatus(), result.GetDegreesOfFreedom()),
		Patch:             patch,
		AffectedEntityIDs: append([]string(nil), result.GetAffectedEntityIds()...),
		Diagnostics:       solverDiagnostics(result.GetDiagnostics()),
	}, nil
}

func (s *Service) Analyze(ctx context.Context, sketchID string) (*model.AnalyzeSketchResponse, error) {
	sketchID = strings.TrimSpace(sketchID)
	if sketchID == "" {
		return nil, errors.New("sketchID is required")
	}

	document, err := s.loadDocument(ctx, sketchID)
	if err != nil {
		return nil, err
	}

	result, err := s.client.Analyze(ctx, &solverv1.AnalyzeRequest{
		Model:   sketchModel(document),
		Options: defaultSolverOptions(),
	})
	if err != nil {
		return nil, fmt.Errorf("analyze sketch: %w", err)
	}
	if result == nil {
		return nil, errors.New("solver returned nil analyze response")
	}

	return &model.AnalyzeSketchResponse{
		Status:           solveStatusInfo(result.GetStatus(), result.GetDegreesOfFreedom()),
		DegreesOfFreedom: int(result.GetDegreesOfFreedom()),
		Components:       constraintComponents(result.GetComponents()),
		Diagnostics:      solverDiagnostics(result.GetDiagnostics()),
	}, nil
}

func (s *Service) loadDocument(ctx context.Context, sketchID string) (*model.SketchDocument, error) {
	if s.sketches == nil {
		return nil, errors.New("sketch repository is required")
	}
	if s.client == nil {
		return nil, errors.New("solver client is required")
	}

	document, err := s.sketches.Get(ctx, sketchID)
	if err != nil {
		return nil, fmt.Errorf("get sketch for solver: %w", err)
	}
	if document == nil {
		return nil, errors.New("sketch repository returned nil document")
	}

	return document, nil
}

func buildApplyIntentRequest(
	document *model.SketchDocument,
	request *model.SolvePreviewRequest,
) (*solverv1.ApplyIntentRequest, error) {
	intent, err := userIntent(request.Intent)
	if err != nil {
		return nil, err
	}

	options, err := solverOptions(request.Options)
	if err != nil {
		return nil, err
	}

	return &solverv1.ApplyIntentRequest{
		Model:   sketchModel(document),
		Intent:  intent,
		Options: options,
	}, nil
}

func sketchModel(document *model.SketchDocument) *solverv1.SketchModel {
	return &solverv1.SketchModel{
		Entities:    entities(document.Entities),
		Constraints: constraints(document.Constraints),
		Dimensions:  dimensions(document.Dimensions),
	}
}

func BuildSketchModel(document *model.SketchDocument) *solverv1.SketchModel {
	return sketchModel(document)
}

func entities(raw map[string]easyjson.RawMessage) []*solverv1.Entity {
	keys := sortedKeys(raw)
	result := make([]*solverv1.Entity, 0, len(keys))
	for _, key := range keys {
		entity, err := entity(raw[key])
		if err == nil {
			result = append(result, entity)
		}
	}
	return result
}

func entity(raw easyjson.RawMessage) (*solverv1.Entity, error) {
	var data struct {
		ID             string  `json:"id"`
		Type           string  `json:"type"`
		DeletedAtOpID  *string `json:"deletedAtOpId"`
		IsConstruction bool    `json:"isConstruction"`
		X              float64 `json:"x"`
		Y              float64 `json:"y"`
		Fixed          bool    `json:"fixed"`
		StartPointID   string  `json:"startPointId"`
		EndPointID     string  `json:"endPointId"`
		CenterPointID  string  `json:"centerPointId"`
		Radius         float64 `json:"radius"`
		Clockwise      bool    `json:"clockwise"`
		Branch         string  `json:"branch"`
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("decode entity: %w", err)
	}
	if data.DeletedAtOpID != nil {
		return nil, errSkippedModelItem
	}

	result := &solverv1.Entity{Id: data.ID}
	switch data.Type {
	case "point":
		result.Kind = &solverv1.Entity_Point{Point: &solverv1.Point{X: data.X, Y: data.Y, Fixed: data.Fixed}}
	case "line":
		result.Kind = &solverv1.Entity_Line{Line: &solverv1.Line{
			StartPointId: data.StartPointID,
			EndPointId:   data.EndPointID,
		}}
	case "circle":
		result.Kind = &solverv1.Entity_Circle{Circle: &solverv1.Circle{
			CenterPointId: data.CenterPointID,
			Radius:        data.Radius,
		}}
	case "arc":
		result.Kind = &solverv1.Entity_Arc{Arc: &solverv1.Arc{
			CenterPointId: data.CenterPointID,
			StartPointId:  data.StartPointID,
			EndPointId:    data.EndPointID,
			Clockwise:     data.Clockwise,
			Branch:        arcBranch(data.Branch),
		}}
	default:
		return nil, fmt.Errorf("unsupported entity type %q", data.Type)
	}

	return result, nil
}

func constraints(raw map[string]easyjson.RawMessage) []*solverv1.Constraint {
	keys := sortedKeys(raw)
	result := make([]*solverv1.Constraint, 0, len(keys))
	for _, key := range keys {
		constraint, err := constraint(raw[key])
		if err == nil {
			result = append(result, constraint)
		}
	}
	return result
}

func constraint(raw easyjson.RawMessage) (*solverv1.Constraint, error) {
	var data struct {
		ID         string `json:"id"`
		Type       string `json:"type"`
		Status     string `json:"status"`
		PointAID   string `json:"pointAId"`
		PointBID   string `json:"pointBId"`
		LineID     string `json:"lineId"`
		LineAID    string `json:"lineAId"`
		LineBID    string `json:"lineBId"`
		EntityID   string `json:"entityId"`
		EntityAID  string `json:"entityAId"`
		EntityBID  string `json:"entityBId"`
		MidpointID string `json:"midpointId"`
		CircleAID  string `json:"circleAId"`
		CircleBID  string `json:"circleBId"`
		Branch     string `json:"branch"`
		Kind       string `json:"kind"`
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("decode constraint: %w", err)
	}
	if data.Status == statusDeleted {
		return nil, errSkippedModelItem
	}

	result := &solverv1.Constraint{Id: data.ID, Status: constraintStatus(data.Status)}
	if result.GetStatus() == solverv1.ConstraintStatus_CONSTRAINT_STATUS_UNSPECIFIED {
		result.Status = solverv1.ConstraintStatus_CONSTRAINT_STATUS_ACTIVE
	}

	switch data.Type {
	case "coincident":
		result.Kind = &solverv1.Constraint_Coincident{Coincident: &solverv1.CoincidentConstraint{
			PointAId: data.PointAID,
			PointBId: data.PointBID,
		}}
	case "horizontal":
		result.Kind = &solverv1.Constraint_Horizontal{Horizontal: &solverv1.HorizontalConstraint{LineId: data.LineID}}
	case "vertical":
		result.Kind = &solverv1.Constraint_Vertical{Vertical: &solverv1.VerticalConstraint{LineId: data.LineID}}
	case "parallel":
		result.Kind = &solverv1.Constraint_Parallel{Parallel: &solverv1.ParallelConstraint{
			LineAId: data.LineAID,
			LineBId: data.LineBID,
		}}
	case "perpendicular":
		result.Kind = &solverv1.Constraint_Perpendicular{Perpendicular: &solverv1.PerpendicularConstraint{
			LineAId: data.LineAID,
			LineBId: data.LineBID,
		}}
	case "tangent":
		result.Kind = &solverv1.Constraint_Tangent{Tangent: &solverv1.TangentConstraint{
			EntityAId: data.EntityAID,
			EntityBId: data.EntityBID,
			Branch:    tangentBranch(data.Branch),
		}}
	case "equal":
		result.Kind = &solverv1.Constraint_Equal{Equal: &solverv1.EqualConstraint{
			EntityAId: data.EntityAID,
			EntityBId: data.EntityBID,
			Kind:      equalKind(data.Kind),
		}}
	case "fixed":
		result.Kind = &solverv1.Constraint_Fixed{Fixed: &solverv1.FixedConstraint{EntityId: data.EntityID}}
	case "midpoint":
		result.Kind = &solverv1.Constraint_Midpoint{Midpoint: &solverv1.MidpointConstraint{
			MidpointId: data.MidpointID,
			PointAId:   data.PointAID,
			PointBId:   data.PointBID,
		}}
	case "concentric":
		result.Kind = &solverv1.Constraint_Concentric{Concentric: &solverv1.ConcentricConstraint{
			CircleAId: data.CircleAID,
			CircleBId: data.CircleBID,
		}}
	default:
		return nil, fmt.Errorf("unsupported constraint type %q", data.Type)
	}

	return result, nil
}

func dimensions(raw map[string]easyjson.RawMessage) []*solverv1.Dimension {
	keys := sortedKeys(raw)
	result := make([]*solverv1.Dimension, 0, len(keys))
	for _, key := range keys {
		dimension, err := dimension(raw[key])
		if err == nil {
			result = append(result, dimension)
		}
	}
	return result
}

func dimension(raw easyjson.RawMessage) (*solverv1.Dimension, error) {
	var data struct {
		ID          string  `json:"id"`
		Type        string  `json:"type"`
		Value       float64 `json:"value"`
		Driving     bool    `json:"driving"`
		Status      string  `json:"status"`
		RefAID      string  `json:"refAId"`
		RefBID      string  `json:"refBId"`
		RefKind     string  `json:"refKind"`
		EntityID    string  `json:"entityId"`
		LineAID     string  `json:"lineAId"`
		LineBID     string  `json:"lineBId"`
		Orientation string  `json:"orientation"`
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("decode dimension: %w", err)
	}
	if data.Status == statusDeleted {
		return nil, errSkippedModelItem
	}

	result := &solverv1.Dimension{Id: data.ID, Driving: data.Driving, Status: constraintStatus(data.Status)}
	if result.GetStatus() == solverv1.ConstraintStatus_CONSTRAINT_STATUS_UNSPECIFIED {
		result.Status = solverv1.ConstraintStatus_CONSTRAINT_STATUS_ACTIVE
	}

	switch data.Type {
	case "distance":
		result.Kind = &solverv1.Dimension_Distance{Distance: &solverv1.DistanceDimension{
			RefAId:  data.RefAID,
			RefBId:  data.RefBID,
			Value:   data.Value,
			RefKind: distanceReferenceKind(data.RefKind),
		}}
	case "radius":
		result.Kind = &solverv1.Dimension_Radius{Radius: &solverv1.RadiusDimension{
			EntityId: data.EntityID,
			Value:    data.Value,
		}}
	case "diameter":
		result.Kind = &solverv1.Dimension_Diameter{Diameter: &solverv1.DiameterDimension{
			EntityId: data.EntityID,
			Value:    data.Value,
		}}
	case "angle":
		result.Kind = &solverv1.Dimension_Angle{Angle: &solverv1.AngleDimension{
			LineAId:     data.LineAID,
			LineBId:     data.LineBID,
			ValueRad:    data.Value,
			Orientation: angleOrientation(data.Orientation),
		}}
	default:
		return nil, fmt.Errorf("unsupported dimension type %q", data.Type)
	}

	return result, nil
}

func userIntent(raw easyjson.RawMessage) (*solverv1.UserIntent, error) {
	var data struct {
		Type              string          `json:"type"`
		PointID           string          `json:"pointId"`
		EntityID          string          `json:"entityId"`
		DimensionID       string          `json:"dimensionId"`
		FeatureID         string          `json:"featureId"`
		Line1ID           string          `json:"line1Id"`
		Line2ID           string          `json:"line2Id"`
		CornerPointID     string          `json:"cornerPointId"`
		CreatedPoint1ID   string          `json:"createdPoint1Id"`
		CreatedPoint2ID   string          `json:"createdPoint2Id"`
		CreatedPointID    string          `json:"createdPointId"`
		CreatedArcID      string          `json:"createdArcId"`
		CreatedLineID     string          `json:"createdLineId"`
		CreatedEntityIDs  []string        `json:"createdEntityIds"`
		SourceEntityIDs   []string        `json:"sourceEntityIds"`
		MirrorLineID      string          `json:"mirrorLineId"`
		Target            vec2JSON        `json:"target"`
		PickPoint         vec2JSON        `json:"pickPoint"`
		Direction         vec2JSON        `json:"direction"`
		Delta             vec2JSON        `json:"delta"`
		Value             float64         `json:"value"`
		Weight            float64         `json:"weight"`
		Radius            float64         `json:"radius"`
		Distance1         float64         `json:"distance1"`
		Distance2         float64         `json:"distance2"`
		Spacing           float64         `json:"spacing"`
		Count             int32           `json:"count"`
		CenterPointID     string          `json:"centerPointId"`
		TotalAngleRad     float64         `json:"totalAngleRad"`
		Endpoint          string          `json:"endpoint"`
		BoundaryEntityIDs []string        `json:"boundaryEntityIds"`
		TargetEntityIDs   []string        `json:"targetEntityIds"`
		Copy              bool            `json:"copy"`
		KeepConstraints   bool            `json:"keepConstraints"`
		RotateInstances   bool            `json:"rotateInstances"`
		Trim              bool            `json:"trim"`
		Clockwise         bool            `json:"clockwise"`
		Constraint        json.RawMessage `json:"constraint"`
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("decode solver intent: %w", err)
	}
	if data.Weight == 0 {
		data.Weight = 1
	}

	result := &solverv1.UserIntent{}
	switch data.Type {
	case "move_point":
		result.Kind = &solverv1.UserIntent_MovePoint{MovePoint: &solverv1.MovePointIntent{
			PointId: data.PointID,
			Target:  data.Target.proto(),
			Weight:  data.Weight,
		}}
	case "move_entity":
		result.Kind = &solverv1.UserIntent_MoveEntity{MoveEntity: &solverv1.MoveEntityIntent{
			EntityId: data.EntityID,
			Delta:    data.Delta.proto(),
			Weight:   data.Weight,
		}}
	case "set_dimension":
		result.Kind = &solverv1.UserIntent_SetDimension{SetDimension: &solverv1.SetDimensionIntent{
			DimensionId: data.DimensionID,
			Value:       data.Value,
		}}
	case "add_constraint":
		constraint, err := constraint(easyjson.RawMessage(data.Constraint))
		if err != nil {
			return nil, err
		}
		if constraint.GetStatus() == solverv1.ConstraintStatus_CONSTRAINT_STATUS_UNSPECIFIED {
			constraint.Status = solverv1.ConstraintStatus_CONSTRAINT_STATUS_ACTIVE
		}
		result.Kind = &solverv1.UserIntent_AddConstraint{AddConstraint: &solverv1.AddConstraintIntent{
			Constraint: constraint,
		}}
	case "ApplyFillet":
		result.Kind = &solverv1.UserIntent_ApplyFillet{ApplyFillet: &solverv1.ApplyFilletIntent{
			FeatureId:       data.FeatureID,
			Line1Id:         data.Line1ID,
			Line2Id:         data.Line2ID,
			CornerPointId:   data.CornerPointID,
			CreatedPoint1Id: data.CreatedPoint1ID,
			CreatedPoint2Id: data.CreatedPoint2ID,
			CreatedArcId:    data.CreatedArcID,
			Radius:          data.Radius,
			Trim:            data.Trim,
			Clockwise:       data.Clockwise,
		}}
	case "ApplyChamfer":
		createdArcID := data.CreatedArcID
		if createdArcID == "" {
			createdArcID = data.CreatedLineID
		}
		result.Kind = &solverv1.UserIntent_ApplyChamfer{ApplyChamfer: &solverv1.ApplyChamferIntent{
			FeatureId:       data.FeatureID,
			Line1Id:         data.Line1ID,
			Line2Id:         data.Line2ID,
			CornerPointId:   data.CornerPointID,
			CreatedPoint1Id: data.CreatedPoint1ID,
			CreatedPoint2Id: data.CreatedPoint2ID,
			CreatedArcId:    createdArcID,
			Distance1:       data.Distance1,
			Distance2:       data.Distance2,
			Trim:            data.Trim,
		}}
	case "UpdateFillet":
		result.Kind = &solverv1.UserIntent_UpdateFillet{UpdateFillet: &solverv1.UpdateFilletIntent{
			FeatureId: data.FeatureID,
			Radius:    data.Radius,
			Trim:      data.Trim,
			Clockwise: data.Clockwise,
		}}
	case "UpdateChamfer":
		result.Kind = &solverv1.UserIntent_UpdateChamfer{UpdateChamfer: &solverv1.UpdateChamferIntent{
			FeatureId: data.FeatureID,
			Distance1: data.Distance1,
			Distance2: data.Distance2,
			Trim:      data.Trim,
		}}
	case "split_entity":
		result.Kind = &solverv1.UserIntent_SplitEntity{SplitEntity: &solverv1.SplitEntityIntent{
			EntityId:         data.EntityID,
			PickPoint:        data.PickPoint.proto(),
			CreatedPointId:   data.CreatedPointID,
			CreatedEntityIds: append([]string(nil), data.CreatedEntityIDs...),
		}}
	case "break_entity_at_point":
		result.Kind = &solverv1.UserIntent_BreakEntityAtPoint{BreakEntityAtPoint: &solverv1.BreakEntityAtPointIntent{
			EntityId:         data.EntityID,
			PointId:          data.PointID,
			PickPoint:        data.PickPoint.proto(),
			CreatedEntityIds: append([]string(nil), data.CreatedEntityIDs...),
		}}
	case "trim_entity":
		result.Kind = &solverv1.UserIntent_TrimEntity{TrimEntity: &solverv1.TrimEntityIntent{
			EntityId:          data.EntityID,
			PickPoint:         data.PickPoint.proto(),
			BoundaryEntityIds: append([]string(nil), data.BoundaryEntityIDs...),
		}}
	case "extend_entity":
		result.Kind = &solverv1.UserIntent_ExtendEntity{ExtendEntity: &solverv1.ExtendEntityIntent{
			EntityId:        data.EntityID,
			Endpoint:        data.Endpoint,
			Target:          data.Target.proto(),
			TargetEntityIds: append([]string(nil), data.TargetEntityIDs...),
		}}
	case "mirror_entities":
		result.Kind = &solverv1.UserIntent_MirrorEntities{MirrorEntities: &solverv1.MirrorEntitiesIntent{
			FeatureId:        data.FeatureID,
			SourceEntityIds:  append([]string(nil), data.SourceEntityIDs...),
			MirrorLineId:     data.MirrorLineID,
			CreatedEntityIds: append([]string(nil), data.CreatedEntityIDs...),
			Copy:             data.Copy,
			KeepConstraints:  data.KeepConstraints,
		}}
	case "linear_pattern":
		result.Kind = &solverv1.UserIntent_LinearPattern{LinearPattern: &solverv1.LinearPatternIntent{
			FeatureId:        data.FeatureID,
			SourceEntityIds:  append([]string(nil), data.SourceEntityIDs...),
			Direction:        data.Direction.proto(),
			Spacing:          data.Spacing,
			Count:            data.Count,
			CreatedEntityIds: append([]string(nil), data.CreatedEntityIDs...),
			KeepConstraints:  data.KeepConstraints,
		}}
	case "circular_pattern":
		result.Kind = &solverv1.UserIntent_CircularPattern{CircularPattern: &solverv1.CircularPatternIntent{
			FeatureId:        data.FeatureID,
			SourceEntityIds:  append([]string(nil), data.SourceEntityIDs...),
			CenterPointId:    data.CenterPointID,
			TotalAngleRad:    data.TotalAngleRad,
			Count:            data.Count,
			CreatedEntityIds: append([]string(nil), data.CreatedEntityIDs...),
			RotateInstances:  data.RotateInstances,
			KeepConstraints:  data.KeepConstraints,
		}}
	default:
		return nil, fmt.Errorf("unsupported solver intent type %q", data.Type)
	}

	return result, nil
}

func UserIntent(raw easyjson.RawMessage) (*solverv1.UserIntent, error) {
	return userIntent(raw)
}

type vec2JSON struct {
	X float64 `json:"x"`
	Y float64 `json:"y"`
}

func (v vec2JSON) proto() *solverv1.Vec2 {
	return &solverv1.Vec2{X: v.X, Y: v.Y}
}

func solverOptions(raw easyjson.RawMessage) (*solverv1.SolverOptions, error) {
	options := defaultSolverOptions()
	if len(raw) == 0 {
		return options, nil
	}

	var data struct {
		Tolerance         *float64 `json:"tolerance"`
		MaxIterations     *int32   `json:"maxIterations"`
		Deterministic     *bool    `json:"deterministic"`
		ReturnDiagnostics *bool    `json:"returnDiagnostics"`
		SoftIntentWeight  *float64 `json:"softIntentWeight"`
		StabilityWeight   *float64 `json:"stabilityWeight"`
	}
	if err := json.Unmarshal(raw, &data); err != nil {
		return nil, fmt.Errorf("decode solver options: %w", err)
	}
	if data.Tolerance != nil {
		options.Tolerance = *data.Tolerance
	}
	if data.MaxIterations != nil {
		options.MaxIterations = *data.MaxIterations
	}
	if data.Deterministic != nil {
		options.Deterministic = *data.Deterministic
	}
	if data.ReturnDiagnostics != nil {
		options.ReturnDiagnostics = *data.ReturnDiagnostics
	}
	if data.SoftIntentWeight != nil {
		options.SoftIntentWeight = *data.SoftIntentWeight
	}
	if data.StabilityWeight != nil {
		options.StabilityWeight = *data.StabilityWeight
	}

	return options, nil
}

func defaultSolverOptions() *solverv1.SolverOptions {
	return &solverv1.SolverOptions{
		Tolerance:         defaultSolverTolerance,
		MaxIterations:     defaultSolverMaxIterations,
		Deterministic:     true,
		ReturnDiagnostics: true,
	}
}

type solverPatchProfileLoop struct {
	EntityIDs []string `json:"entityIds"`
}

type solverPatchProfile struct {
	ID              string                   `json:"id"`
	OuterLoop       solverPatchProfileLoop   `json:"outerLoop"`
	InnerLoops      []solverPatchProfileLoop `json:"innerLoops,omitempty"`
	Area            float64                  `json:"area"`
	ValidForExtrude bool                     `json:"validForExtrude"`
}

func solutionPatch(solution *solverv1.SketchSolution) (easyjson.RawMessage, error) {
	type patchEntity struct {
		ID            string   `json:"id"`
		Type          string   `json:"type"`
		X             *float64 `json:"x,omitempty"`
		Y             *float64 `json:"y,omitempty"`
		CenterPointID string   `json:"centerPointId,omitempty"`
		Radius        float64  `json:"radius,omitempty"`
		StartPointID  string   `json:"startPointId,omitempty"`
		EndPointID    string   `json:"endPointId,omitempty"`
		Clockwise     bool     `json:"clockwise,omitempty"`
		Branch        string   `json:"branch,omitempty"`
	}
	patch := struct {
		Entities map[string]patchEntity `json:"entities"`
		Profiles []solverPatchProfile   `json:"profiles,omitempty"`
	}{
		Entities: make(map[string]patchEntity),
	}

	for _, entity := range solution.GetEntities() {
		switch kind := entity.GetKind().(type) {
		case *solverv1.SolvedEntity_Point:
			x := kind.Point.GetX()
			y := kind.Point.GetY()
			patch.Entities[entity.GetId()] = patchEntity{
				ID:   entity.GetId(),
				Type: "point",
				X:    &x,
				Y:    &y,
			}
		case *solverv1.SolvedEntity_Line:
			patch.Entities[entity.GetId()] = patchEntity{
				ID:           entity.GetId(),
				Type:         "line",
				StartPointID: kind.Line.GetStartPointId(),
				EndPointID:   kind.Line.GetEndPointId(),
			}
		case *solverv1.SolvedEntity_Circle:
			patch.Entities[entity.GetId()] = patchEntity{
				ID:            entity.GetId(),
				Type:          "circle",
				CenterPointID: kind.Circle.GetCenterPointId(),
				Radius:        kind.Circle.GetRadius(),
			}
		case *solverv1.SolvedEntity_Arc:
			patch.Entities[entity.GetId()] = patchEntity{
				ID:            entity.GetId(),
				Type:          "arc",
				CenterPointID: kind.Arc.GetCenterPointId(),
				StartPointID:  kind.Arc.GetStartPointId(),
				EndPointID:    kind.Arc.GetEndPointId(),
				Clockwise:     kind.Arc.GetClockwise(),
				Branch:        arcBranchString(kind.Arc.GetBranch()),
			}
		}
	}

	for _, profile := range solution.GetProfiles() {
		patch.Profiles = append(patch.Profiles, solverPatchProfile{
			ID:              profile.GetId(),
			OuterLoop:       profileLoop(profile.GetOuterLoop()),
			InnerLoops:      profileLoops(profile.GetInnerLoops()),
			Area:            profile.GetArea(),
			ValidForExtrude: profile.GetValidForExtrude(),
		})
	}

	body, err := json.Marshal(patch)
	if err != nil {
		return nil, fmt.Errorf("encode solver patch: %w", err)
	}

	return easyjson.RawMessage(body), nil
}

func SolutionPatch(solution *solverv1.SketchSolution) (easyjson.RawMessage, error) {
	return solutionPatch(solution)
}

func profileLoop(loop *solverv1.ProfileLoop) solverPatchProfileLoop {
	entityIDs := append([]string(nil), loop.GetEntityIds()...)
	if entityIDs == nil {
		entityIDs = []string{}
	}
	return solverPatchProfileLoop{
		EntityIDs: entityIDs,
	}
}

func profileLoops(loops []*solverv1.ProfileLoop) []solverPatchProfileLoop {
	result := make([]solverPatchProfileLoop, 0, len(loops))
	for _, loop := range loops {
		result = append(result, profileLoop(loop))
	}
	return result
}

func solveStatusInfo(status solverv1.SolveStatus, degreesOfFreedom int32) model.SolveStatusInfo {
	return model.SolveStatusInfo{
		Status:           solveStatusString(status),
		DegreesOfFreedom: int(degreesOfFreedom),
	}
}

func SolveStatusInfo(status solverv1.SolveStatus, degreesOfFreedom int32) model.SolveStatusInfo {
	return solveStatusInfo(status, degreesOfFreedom)
}

func solverDiagnostics(diagnostics []*solverv1.SolverDiagnostic) []model.SolverDiagnostic {
	result := make([]model.SolverDiagnostic, 0, len(diagnostics))
	for _, diagnostic := range diagnostics {
		result = append(result, model.SolverDiagnostic{
			Level:         diagnosticLevelString(diagnostic.GetLevel()),
			Code:          diagnostic.GetCode(),
			Message:       diagnostic.GetMessage(),
			EntityIDs:     append([]string(nil), diagnostic.GetEntityIds()...),
			ConstraintIDs: append([]string(nil), diagnostic.GetConstraintIds()...),
			DimensionIDs:  append([]string(nil), diagnostic.GetDimensionIds()...),
		})
	}
	return result
}

func SolverDiagnostics(diagnostics []*solverv1.SolverDiagnostic) []model.SolverDiagnostic {
	return solverDiagnostics(diagnostics)
}

func constraintComponents(components []*solverv1.ConstraintComponent) []model.ConstraintComponent {
	result := make([]model.ConstraintComponent, 0, len(components))
	for _, component := range components {
		result = append(result, model.ConstraintComponent{
			ID:               component.GetId(),
			EntityIDs:        append([]string(nil), component.GetEntityIds()...),
			ConstraintIDs:    append([]string(nil), component.GetConstraintIds()...),
			DegreesOfFreedom: int(component.GetDegreesOfFreedom()),
		})
	}
	return result
}

func sortedKeys(values map[string]easyjson.RawMessage) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}
