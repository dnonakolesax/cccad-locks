package permissions

import (
	"context"
	"errors"
	"strings"

	"github.com/dnonakolesax/cccad-locks/internal/model"
)

type Repository interface {
	List(ctx context.Context, sketchID string) ([]model.Permission, error)
	Put(ctx context.Context, permission *model.Permission) (*model.Permission, error)
	Delete(ctx context.Context, userID, sketchID string) error
}

type Service struct {
	repo Repository
}

func NewService(repo Repository) *Service {
	return &Service{repo: repo}
}

func (s *Service) List(ctx context.Context, sketchID string) ([]model.Permission, error) {
	if strings.TrimSpace(sketchID) == "" {
		return nil, errors.New("sketchID is required")
	}

	return s.repo.List(ctx, sketchID)
}

func (s *Service) Put(ctx context.Context, permission *model.Permission) (*model.Permission, error) {
	if permission == nil {
		return nil, errors.New("permission is required")
	}
	permission.SketchID = strings.TrimSpace(permission.SketchID)
	permission.UserID = strings.TrimSpace(permission.UserID)
	permission.Role = strings.TrimSpace(permission.Role)
	if permission.SketchID == "" || permission.UserID == "" {
		return nil, errors.New("sketchID and userID are required")
	}
	if !isValidRole(permission.Role) {
		return nil, errors.New("role must be reader, editor, or admin")
	}

	return s.repo.Put(ctx, permission)
}

func (s *Service) Delete(ctx context.Context, userID, sketchID string) error {
	userID = strings.TrimSpace(userID)
	sketchID = strings.TrimSpace(sketchID)
	if userID == "" || sketchID == "" {
		return errors.New("sketchID and userID are required")
	}

	return s.repo.Delete(ctx, userID, sketchID)
}

func isValidRole(role string) bool {
	switch role {
	case "reader", "editor", "admin":
		return true
	default:
		return false
	}
}
