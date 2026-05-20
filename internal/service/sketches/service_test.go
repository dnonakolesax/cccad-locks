package sketches

import (
	"context"
	"testing"

	"github.com/dnonakolesax/cccad-locks/internal/auth"
	"github.com/dnonakolesax/cccad-locks/internal/model"
)

type repositoryStub struct {
	listAvailableUserID string
	listAvailableResult []model.AvailableSketch
}

func (r *repositoryStub) Create(
	context.Context,
	string,
	*model.CreateSketchRequest,
	string,
) (*model.SketchMetadata, error) {
	return nil, nil
}

func (r *repositoryStub) ListAvailable(
	_ context.Context,
	userID string,
) ([]model.AvailableSketch, error) {
	r.listAvailableUserID = userID
	return r.listAvailableResult, nil
}

func (r *repositoryStub) Get(context.Context, string) (*model.SketchDocument, error) {
	return nil, nil
}

func (r *repositoryStub) UpdateMetadata(
	context.Context,
	string,
	*model.UpdateSketchMetadataRequest,
) (*model.SketchMetadata, error) {
	return nil, nil
}

func (r *repositoryStub) Delete(context.Context, string) error {
	return nil
}

func TestServiceListAvailableUsesAuthenticatedUser(t *testing.T) {
	repo := &repositoryStub{
		listAvailableResult: []model.AvailableSketch{{ID: "sketch-id", Role: "reader"}},
	}
	service := NewService(repo)
	ctx := auth.ContextWithUserID(context.Background(), "user-id")

	sketches, err := service.ListAvailable(ctx)
	if err != nil {
		t.Fatalf("ListAvailable returned error: %v", err)
	}
	if repo.listAvailableUserID != "user-id" {
		t.Fatalf("ListAvailable userID = %q, want %q", repo.listAvailableUserID, "user-id")
	}
	if len(sketches) != 1 || sketches[0].ID != "sketch-id" {
		t.Fatalf("ListAvailable result = %#v, want sketch-id", sketches)
	}
}

func TestServiceListAvailableRequiresAuthenticatedUser(t *testing.T) {
	service := NewService(&repositoryStub{})

	if _, err := service.ListAvailable(context.Background()); err == nil {
		t.Fatal("ListAvailable returned nil error without authenticated user")
	}
}
