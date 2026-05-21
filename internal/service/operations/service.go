package operations

import (
	"context"
	"errors"
	"strings"

	"github.com/dnonakolesax/cccad-locks/internal/auth"
	"github.com/dnonakolesax/cccad-locks/internal/model"
)

type Repository interface {
	List(
		ctx context.Context,
		userID string,
		sketchID string,
		afterVersion int64,
		limit int,
	) (*model.SketchOperationPage, error)
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) List(
	ctx context.Context,
	sketchID string,
	afterVersion int64,
	limit int,
) (*model.SketchOperationPage, error) {
	sketchID = strings.TrimSpace(sketchID)
	if sketchID == "" {
		return nil, errors.New("sketchID is required")
	}
	if afterVersion < 0 {
		return nil, errors.New("afterVersion must be greater than or equal to 0")
	}
	if limit < 1 {
		return nil, errors.New("limit must be greater than 0")
	}

	userID, ok := auth.UserIDFromContext(ctx)
	if !ok {
		return nil, errors.New("authenticated user id is required")
	}

	return s.repo.List(ctx, userID, sketchID, afterVersion, limit)
}

func (s *Service) Submit(
	_ context.Context,
	_ string,
	_ *model.SubmitOperationRequest,
) (*model.SubmitOperationResponse, error) {
	return nil, errors.New("submit operation is not implemented")
}
