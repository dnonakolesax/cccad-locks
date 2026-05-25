package workspaces

import (
	"context"
	"errors"
	"strings"

	"github.com/dnonakolesax/cccad-locks/internal/auth"
	"github.com/dnonakolesax/cccad-locks/internal/model"
)

type Repository interface {
	Create(ctx context.Context, request *model.CreateWorkspaceRequest, createdByUserID string) (*model.Workspace, error)
	ListAvailable(ctx context.Context, userID string) ([]model.Workspace, error)
	Update(
		ctx context.Context,
		workspaceID string,
		request *model.UpdateWorkspaceRequest,
		actorUserID string,
	) (*model.Workspace, error)
	Delete(ctx context.Context, workspaceID string, actorUserID string) error
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) Create(ctx context.Context, request *model.CreateWorkspaceRequest) (*model.Workspace, error) {
	if request == nil {
		return nil, errors.New("request is required")
	}

	request.Name = strings.TrimSpace(request.Name)
	request.Description = strings.TrimSpace(request.Description)
	if request.Name == "" {
		return nil, errors.New("name is required")
	}

	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, errors.New("authenticated user id is required")
	}

	return s.repo.Create(ctx, request, userID)
}

func (s *Service) ListAvailable(ctx context.Context) ([]model.Workspace, error) {
	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, errors.New("authenticated user id is required")
	}

	return s.repo.ListAvailable(ctx, userID)
}

func (s *Service) Update(
	ctx context.Context,
	workspaceID string,
	request *model.UpdateWorkspaceRequest,
) (*model.Workspace, error) {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return nil, errors.New("workspaceID is required")
	}
	if request == nil {
		return nil, errors.New("request is required")
	}
	if request.Name == nil && request.Description == nil {
		return nil, errors.New("name or description is required")
	}
	if request.Name != nil {
		name := strings.TrimSpace(*request.Name)
		if name == "" {
			return nil, errors.New("name must not be empty")
		}
		request.Name = &name
	}
	if request.Description != nil {
		description := strings.TrimSpace(*request.Description)
		request.Description = &description
	}

	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, errors.New("authenticated user id is required")
	}

	return s.repo.Update(ctx, workspaceID, request, userID)
}

func (s *Service) Delete(ctx context.Context, workspaceID string) error {
	workspaceID = strings.TrimSpace(workspaceID)
	if workspaceID == "" {
		return errors.New("workspaceID is required")
	}

	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return errors.New("authenticated user id is required")
	}

	return s.repo.Delete(ctx, workspaceID, userID)
}
