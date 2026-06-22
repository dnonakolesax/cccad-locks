package sketches

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"

	"github.com/dnonakolesax/cccad-locks/internal/auth"
	"github.com/dnonakolesax/cccad-locks/internal/model"
	"github.com/mailru/easyjson"
)

const defaultUnit = "mm"

type Repository interface {
	Create(
		ctx context.Context,
		workspaceID string,
		request *model.CreateSketchRequest,
		createdByUserID string,
	) (*model.SketchMetadata, error)
	ListAvailable(ctx context.Context, userID string) ([]model.AvailableSketch, error)
	Get(ctx context.Context, sketchID string) (*model.SketchDocument, error)
	Snapshot(ctx context.Context, sketchID string, version int64, userID string) (*model.SketchSnapshot, error)
	DeletedEntityGeometry(
		ctx context.Context,
		sketchID string,
		entityID string,
		userID string,
	) (*model.DeletedSketchEntityGeometry, error)
	RevertToVersion(ctx context.Context, sketchID string, version int64, userID string) (*model.SketchDocument, error)
	UpdateMetadata(
		ctx context.Context,
		sketchID string,
		request *model.UpdateSketchMetadataRequest,
	) (*model.SketchMetadata, error)
	Delete(ctx context.Context, sketchID string) error
}

type RevertNotifier interface {
	BeginRevert(
		ctx context.Context,
		sketchID string,
		targetVersion int64,
		actorUserID string,
	) func(document *model.SketchDocument, err error)
}

type Service struct {
	repo           Repository
	revertNotifier RevertNotifier
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) SetRevertNotifier(notifier RevertNotifier) {
	s.revertNotifier = notifier
}

func (s *Service) Create(
	ctx context.Context,
	workspaceID string,
	request *model.CreateSketchRequest,
) (*model.SketchMetadata, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return nil, errors.New("workspaceID is required")
	}
	if request == nil {
		return nil, errors.New("request is required")
	}

	request.Name = strings.TrimSpace(request.Name)
	request.Unit = strings.TrimSpace(request.Unit)
	if request.Name == "" {
		return nil, errors.New("name is required")
	}
	if request.Unit == "" {
		request.Unit = defaultUnit
	}
	if !isValidUnit(request.Unit) {
		return nil, errors.New("unit must be mm, cm, m, or inch")
	}
	if request.Plane == nil {
		return nil, errors.New("plane is required")
	}
	if !isValidPlane(*request.Plane) {
		return nil, errors.New("plane origin, normal, and xAxis must contain finite coordinates with non-zero normal and xAxis")
	}

	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, errors.New("authenticated user id is required")
	}

	return s.repo.Create(ctx, workspaceID, request, userID)
}

func (s *Service) ListAvailable(ctx context.Context) ([]model.AvailableSketch, error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, errors.New("authenticated user id is required")
	}

	return s.repo.ListAvailable(ctx, userID)
}

func (s *Service) Get(ctx context.Context, sketchID string) (*model.SketchDocument, error) {
	sketchID = strings.TrimSpace(sketchID)
	if sketchID == "" {
		return nil, errors.New("sketchID is required")
	}

	return s.repo.Get(ctx, sketchID)
}

func (s *Service) Snapshot(ctx context.Context, sketchID string, version int64) (*model.SketchSnapshot, error) {
	sketchID = strings.TrimSpace(sketchID)
	if sketchID == "" {
		return nil, errors.New("sketchID is required")
	}
	if version < 0 {
		return nil, errors.New("version must be greater than or equal to 0")
	}

	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, errors.New("authenticated user id is required")
	}

	return s.repo.Snapshot(ctx, sketchID, version, userID)
}

func (s *Service) DeletedEntityGeometry(
	ctx context.Context,
	sketchID string,
	entityID string,
) (*model.DeletedSketchEntityGeometry, error) {
	sketchID = strings.TrimSpace(sketchID)
	if sketchID == "" {
		return nil, errors.New("sketchID is required")
	}
	entityID = strings.TrimSpace(entityID)
	if entityID == "" {
		return nil, errors.New("entityID is required")
	}

	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, errors.New("authenticated user id is required")
	}

	geometry, err := s.repo.DeletedEntityGeometry(ctx, sketchID, entityID, userID)
	if err != nil {
		return nil, err
	}
	if err := enrichDeletedEntityGeometry(geometry); err != nil {
		return nil, err
	}

	return geometry, nil
}

func (s *Service) RevertToVersion(
	ctx context.Context,
	sketchID string,
	version int64,
) (document *model.SketchDocument, err error) {
	sketchID = strings.TrimSpace(sketchID)
	if sketchID == "" {
		return nil, errors.New("sketchID is required")
	}
	if version < 0 {
		return nil, errors.New("version must be greater than or equal to 0")
	}

	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, errors.New("authenticated user id is required")
	}

	var finish func(*model.SketchDocument, error)
	if s.revertNotifier != nil {
		finish = s.revertNotifier.BeginRevert(ctx, sketchID, version, userID)
	}
	if finish != nil {
		defer func() {
			finish(document, err)
		}()
	}

	return s.repo.RevertToVersion(ctx, sketchID, version, userID)
}

func (s *Service) UpdateMetadata(
	ctx context.Context,
	sketchID string,
	request *model.UpdateSketchMetadataRequest,
) (*model.SketchMetadata, error) {
	sketchID = strings.TrimSpace(sketchID)
	if sketchID == "" {
		return nil, errors.New("sketchID is required")
	}
	if request == nil {
		return nil, errors.New("request is required")
	}
	if request.Name == nil && request.Unit == nil {
		return nil, errors.New("name or unit is required")
	}
	if request.Name != nil {
		trimmedName := strings.TrimSpace(*request.Name)
		if trimmedName == "" {
			return nil, errors.New("name must not be empty")
		}
		request.Name = &trimmedName
	}
	if request.Unit != nil {
		trimmedUnit := strings.TrimSpace(*request.Unit)
		if !isValidUnit(trimmedUnit) {
			return nil, errors.New("unit must be mm, cm, m, or inch")
		}
		request.Unit = &trimmedUnit
	}

	return s.repo.UpdateMetadata(ctx, sketchID, request)
}

func (s *Service) Delete(ctx context.Context, sketchID string) error {
	sketchID = strings.TrimSpace(sketchID)
	if sketchID == "" {
		return errors.New("sketchID is required")
	}

	return s.repo.Delete(ctx, sketchID)
}

func isValidUnit(unit string) bool {
	switch unit {
	case "mm", "cm", "m", "inch":
		return true
	default:
		return false
	}
}

func isValidPlane(plane model.SketchPlane) bool {
	return isFiniteVector(plane.Origin) &&
		isFiniteVector(plane.Normal) &&
		isFiniteVector(plane.XAxis) &&
		vectorLengthSquared(plane.Normal) > 0 &&
		vectorLengthSquared(plane.XAxis) > 0
}

func isFiniteVector(vector model.Vector3) bool {
	return isFinite(vector.X) && isFinite(vector.Y) && isFinite(vector.Z)
}

func isFinite(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0)
}

func vectorLengthSquared(vector model.Vector3) float64 {
	return vector.X*vector.X + vector.Y*vector.Y + vector.Z*vector.Z
}

func enrichDeletedEntityGeometry(geometry *model.DeletedSketchEntityGeometry) error {
	if geometry == nil {
		return nil
	}

	candidateIDs := make(map[string]struct{}, len(geometry.HistoricalEntities)+len(geometry.HistoricalMaterializedGeometry))
	for id := range geometry.HistoricalEntities {
		candidateIDs[id] = struct{}{}
	}
	for id := range geometry.HistoricalMaterializedGeometry {
		candidateIDs[id] = struct{}{}
	}

	seen := map[string]struct{}{geometry.EntityID: {}}
	queue := []json.RawMessage{
		json.RawMessage(geometry.Entity),
		json.RawMessage(geometry.MaterializedGeometry),
	}

	for len(queue) > 0 {
		raw := queue[0]
		queue = queue[1:]

		ids, err := referencedEntityIDs(raw, candidateIDs)
		if err != nil {
			return err
		}
		for _, id := range ids {
			if _, exists := seen[id]; exists {
				continue
			}
			seen[id] = struct{}{}

			entity, hasEntity := geometry.HistoricalEntities[id]
			materialized, hasMaterialized := geometry.HistoricalMaterializedGeometry[id]
			if !hasEntity && !hasMaterialized {
				continue
			}

			if hasEntity {
				if geometry.RelatedEntities == nil {
					geometry.RelatedEntities = make(map[string]easyjson.RawMessage)
				}
				geometry.RelatedEntities[id] = cloneRaw(entity)
				queue = append(queue, json.RawMessage(entity))
			}
			if geometry.RelatedMaterializedGeometry == nil {
				geometry.RelatedMaterializedGeometry = make(map[string]easyjson.RawMessage)
			}
			if hasMaterialized {
				geometry.RelatedMaterializedGeometry[id] = cloneRaw(materialized)
				queue = append(queue, json.RawMessage(materialized))
			} else {
				geometry.RelatedMaterializedGeometry[id] = cloneRaw(entity)
			}
		}
	}

	geometry.HistoricalEntities = nil
	geometry.HistoricalMaterializedGeometry = nil

	return nil
}

func referencedEntityIDs(raw json.RawMessage, candidateIDs map[string]struct{}) ([]string, error) {
	if len(raw) == 0 {
		return nil, nil
	}

	var value any
	if err := json.Unmarshal(raw, &value); err != nil {
		return nil, fmt.Errorf("decode deleted entity geometry references: %w", err)
	}

	ids := make([]string, 0)
	seen := make(map[string]struct{})
	collectReferencedEntityIDs(value, candidateIDs, seen, &ids, "")

	return ids, nil
}

func collectReferencedEntityIDs(
	value any,
	candidateIDs map[string]struct{},
	seen map[string]struct{},
	ids *[]string,
	parentKey string,
) {
	switch typed := value.(type) {
	case map[string]any:
		for key, child := range typed {
			collectReferencedEntityIDs(child, candidateIDs, seen, ids, key)
		}
	case []any:
		for _, child := range typed {
			collectReferencedEntityIDs(child, candidateIDs, seen, ids, parentKey)
		}
	case string:
		if parentKey == "id" {
			return
		}
		if _, exists := candidateIDs[typed]; !exists {
			return
		}
		if _, exists := seen[typed]; exists {
			return
		}
		seen[typed] = struct{}{}
		*ids = append(*ids, typed)
	}
}

func cloneRaw(raw easyjson.RawMessage) easyjson.RawMessage {
	return append(easyjson.RawMessage(nil), raw...)
}
