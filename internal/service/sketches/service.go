package sketches

import (
	"context"
	"errors"
	"math"
	"strings"

	"github.com/dnonakolesax/cccad-locks/internal/auth"
	"github.com/dnonakolesax/cccad-locks/internal/model"
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
	UpdateMetadata(
		ctx context.Context,
		sketchID string,
		request *model.UpdateSketchMetadataRequest,
	) (*model.SketchMetadata, error)
	Delete(ctx context.Context, sketchID string) error
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
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
