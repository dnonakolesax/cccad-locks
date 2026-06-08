package parts3d

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/dnonakolesax/cccad-locks/internal/auth"
	"github.com/dnonakolesax/cccad-locks/internal/model"
)

const (
	uuidLength = 36
	uuidDash1  = 8
	uuidDash2  = 13
	uuidDash3  = 18
	uuidDash4  = 23
)

type Repository interface {
	Create(ctx context.Context, workspaceID string, request *model.CreatePart3DRequest, createdByUserID string) (*model.Part3D, error)
	ListByWorkspace(ctx context.Context, workspaceID string) ([]model.Part3D, error)
	Delete(ctx context.Context, partID string) error
	ListFeatures(ctx context.Context, partID string, includeSuppressed bool) ([]model.Feature3D, error)
	ListBodies(ctx context.Context, partID string) ([]model.Body3D, error)
	ListRepresentations(ctx context.Context, partID string, kind *string) ([]model.Representation3D, error)
	GetTopology(ctx context.Context, partID string, bodyID *string) (*model.TopologySummary3D, error)
	GetFacePlane(ctx context.Context, partID string, bodyID string, faceID string) (*model.FacePlane3D, error)
}

type Service struct {
	repo Repository
}

var ErrFacePlaneNotFound = errors.New("face plane not found")

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Create(
	ctx context.Context,
	workspaceID string,
	request *model.CreatePart3DRequest,
) (*model.Part3D, error) {
	if err := validateUUID("workspaceID", workspaceID); err != nil {
		return nil, err
	}
	if request == nil {
		return nil, errors.New("request is required")
	}
	request.Name = strings.TrimSpace(request.Name)
	if request.Name == "" {
		return nil, errors.New("name is required")
	}
	if s.repo == nil {
		return nil, errors.New("3d parts repository is required")
	}

	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, errors.New("authenticated user id is required")
	}

	return s.repo.Create(ctx, workspaceID, request, userID)
}

func (s *Service) ListByWorkspace(ctx context.Context, workspaceID string) (*model.Part3DList, error) {
	if err := validateUUID("workspaceID", workspaceID); err != nil {
		return nil, err
	}
	if s.repo == nil {
		return nil, errors.New("3d parts repository is required")
	}

	parts, err := s.repo.ListByWorkspace(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	if parts == nil {
		parts = []model.Part3D{}
	}

	return &model.Part3DList{Parts: parts}, nil
}

func (s *Service) Delete(ctx context.Context, partID string) error {
	if err := validateUUID("partID", partID); err != nil {
		return err
	}
	if s.repo == nil {
		return errors.New("3d parts repository is required")
	}

	if _, ok := auth.UserIDFromContext(ctx); !ok {
		return errors.New("authenticated user id is required")
	}

	return s.repo.Delete(ctx, partID)
}

func (s *Service) ListFeatures(
	ctx context.Context,
	partID string,
	includeSuppressed bool,
) (*model.Feature3DList, error) {
	if err := validateUUID("partID", partID); err != nil {
		return nil, err
	}
	if s.repo == nil {
		return nil, errors.New("3d parts repository is required")
	}

	features, err := s.repo.ListFeatures(ctx, partID, includeSuppressed)
	if err != nil {
		return nil, err
	}
	if features == nil {
		features = []model.Feature3D{}
	}

	return &model.Feature3DList{Features: features}, nil
}

func (s *Service) ListBodies(ctx context.Context, partID string) (*model.Body3DList, error) {
	if err := validateUUID("partID", partID); err != nil {
		return nil, err
	}
	if s.repo == nil {
		return nil, errors.New("3d parts repository is required")
	}

	bodies, err := s.repo.ListBodies(ctx, partID)
	if err != nil {
		return nil, err
	}
	if bodies == nil {
		bodies = []model.Body3D{}
	}

	return &model.Body3DList{Bodies: bodies}, nil
}

func (s *Service) ListRepresentations(
	ctx context.Context,
	partID string,
	kind *string,
) (*model.Representation3DList, error) {
	if err := validateUUID("partID", partID); err != nil {
		return nil, err
	}
	if kind != nil {
		trimmed := strings.TrimSpace(*kind)
		if !isValidRepresentationKind(trimmed) {
			return nil, errors.New("kind must be one of brep, glb, mesh_json, step, stl")
		}
		kind = &trimmed
	}
	if s.repo == nil {
		return nil, errors.New("3d parts repository is required")
	}

	representations, err := s.repo.ListRepresentations(ctx, partID, kind)
	if err != nil {
		return nil, err
	}
	if representations == nil {
		representations = []model.Representation3D{}
	}

	return &model.Representation3DList{Representations: representations}, nil
}

func (s *Service) GetTopology(
	ctx context.Context,
	partID string,
	bodyID *string,
) (*model.TopologySummary3D, error) {
	if err := validateUUID("partID", partID); err != nil {
		return nil, err
	}
	if bodyID != nil {
		trimmed := strings.TrimSpace(*bodyID)
		if err := validateUUID("bodyID", trimmed); err != nil {
			return nil, err
		}
		bodyID = &trimmed
	}
	if s.repo == nil {
		return nil, errors.New("3d parts repository is required")
	}

	topology, err := s.repo.GetTopology(ctx, partID, bodyID)
	if err != nil {
		return nil, err
	}
	if topology == nil {
		topology = &model.TopologySummary3D{}
	}
	if topology.Bodies == nil {
		topology.Bodies = []model.TopologyBody3D{}
	}

	return topology, nil
}

func (s *Service) GetFacePlane(
	ctx context.Context,
	partID string,
	bodyID string,
	faceID string,
) (*model.FacePlane3D, error) {
	if err := validateUUID("partID", partID); err != nil {
		return nil, err
	}
	if err := validateUUID("bodyID", bodyID); err != nil {
		return nil, err
	}
	faceID = strings.TrimSpace(faceID)
	if faceID == "" {
		return nil, errors.New("faceID is required")
	}
	if s.repo == nil {
		return nil, errors.New("3d parts repository is required")
	}

	response, err := s.repo.GetFacePlane(ctx, partID, bodyID, faceID)
	if err != nil {
		return nil, err
	}
	if response == nil {
		return nil, fmt.Errorf("%w: %s", ErrFacePlaneNotFound, faceID)
	}
	if response.SurfaceType == "" {
		response.SurfaceType = "unknown"
	}

	return response, nil
}

func validateUUID(field, value string) error {
	value = strings.TrimSpace(value)
	if value == "" {
		return errors.New(field + " is required")
	}
	if !isValidUUID(value) {
		return errors.New(field + " must be a valid uuid")
	}
	return nil
}

func isValidUUID(value string) bool {
	if len(value) != uuidLength {
		return false
	}
	for i, r := range value {
		switch i {
		case uuidDash1, uuidDash2, uuidDash3, uuidDash4:
			if r != '-' {
				return false
			}
		default:
			if !isHex(r) {
				return false
			}
		}
	}
	return true
}

func isHex(r rune) bool {
	return ('0' <= r && r <= '9') || ('a' <= r && r <= 'f') || ('A' <= r && r <= 'F')
}

func isValidRepresentationKind(value string) bool {
	switch value {
	case "brep", "glb", "mesh_json", "step", "stl":
		return true
	default:
		return false
	}
}
