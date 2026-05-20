package sketches

import (
	"context"
	"errors"
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
