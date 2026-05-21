package operations

import (
	"context"
	"testing"

	"github.com/dnonakolesax/cccad-locks/internal/auth"
	"github.com/dnonakolesax/cccad-locks/internal/model"
)

type repositoryStub struct {
	listUserID       string
	listSketchID     string
	listAfterVersion int64
	listLimit        int
	listResult       *model.SketchOperationPage
}

func (r *repositoryStub) List(
	_ context.Context,
	userID string,
	sketchID string,
	afterVersion int64,
	limit int,
) (*model.SketchOperationPage, error) {
	r.listUserID = userID
	r.listSketchID = sketchID
	r.listAfterVersion = afterVersion
	r.listLimit = limit
	return r.listResult, nil
}

func TestServiceListUsesAuthenticatedUser(t *testing.T) {
	repo := &repositoryStub{
		listResult: &model.SketchOperationPage{SketchID: "sketch-id"},
	}
	service := NewService(repo)
	ctx := auth.ContextWithUserID(context.Background(), "user-id")

	page, err := service.List(ctx, " sketch-id ", 12, 50)
	if err != nil {
		t.Fatalf("List returned error: %v", err)
	}
	if page == nil {
		t.Fatal("List returned nil page")
	}
	if repo.listUserID != "user-id" {
		t.Fatalf("List userID = %q, want %q", repo.listUserID, "user-id")
	}
	if repo.listSketchID != "sketch-id" {
		t.Fatalf("List sketchID = %q, want %q", repo.listSketchID, "sketch-id")
	}
	if repo.listAfterVersion != 12 {
		t.Fatalf("List afterVersion = %d, want 12", repo.listAfterVersion)
	}
	if repo.listLimit != 50 {
		t.Fatalf("List limit = %d, want 50", repo.listLimit)
	}
}

func TestServiceListRequiresAuthenticatedUser(t *testing.T) {
	service := NewService(&repositoryStub{})

	if _, err := service.List(context.Background(), "sketch-id", 0, 50); err == nil {
		t.Fatal("List returned nil error without authenticated user")
	}
}

func TestServiceListRejectsInvalidArguments(t *testing.T) {
	service := NewService(&repositoryStub{})
	ctx := auth.ContextWithUserID(context.Background(), "user-id")

	tests := map[string]struct {
		sketchID     string
		afterVersion int64
		limit        int
	}{
		"blank sketch id":  {sketchID: " ", afterVersion: 0, limit: 1},
		"negative version": {sketchID: "sketch-id", afterVersion: -1, limit: 1},
		"zero limit":       {sketchID: "sketch-id", afterVersion: 0, limit: 0},
	}

	for name, tt := range tests {
		t.Run(name, func(t *testing.T) {
			if _, err := service.List(ctx, tt.sketchID, tt.afterVersion, tt.limit); err == nil {
				t.Fatal("List returned nil error")
			}
		})
	}
}
