package parts3d

import (
	"context"
	"testing"

	"github.com/dnonakolesax/cccad-locks/internal/auth"
	"github.com/dnonakolesax/cccad-locks/internal/model"
)

type stubRepository struct {
	createWorkspaceID string
	createUserID      string
	createRequest     *model.CreatePart3DRequest
}

func (r *stubRepository) Create(
	_ context.Context,
	workspaceID string,
	request *model.CreatePart3DRequest,
	createdByUserID string,
) (*model.Part3D, error) {
	r.createWorkspaceID = workspaceID
	r.createUserID = createdByUserID
	r.createRequest = request
	return &model.Part3D{
		ID:              "22222222-2222-2222-2222-222222222222",
		WorkspaceID:     workspaceID,
		Name:            request.Name,
		CreatedByUserID: createdByUserID,
	}, nil
}

func (r *stubRepository) ListFeatures(context.Context, string, bool) ([]model.Feature3D, error) {
	return nil, nil
}

func (r *stubRepository) ListBodies(context.Context, string) ([]model.Body3D, error) {
	return nil, nil
}

func (r *stubRepository) GetTopology(context.Context, string, *string) (*model.TopologySummary3D, error) {
	return nil, nil
}

func (r *stubRepository) GetFacePlane(context.Context, string, string, string) (*model.FacePlane3D, error) {
	return nil, nil
}

func TestCreateRequiresAuthenticatedUser(t *testing.T) {
	repo := &stubRepository{}
	service := NewService(repo)

	_, err := service.Create(
		context.Background(),
		"11111111-1111-1111-1111-111111111111",
		&model.CreatePart3DRequest{Name: "Bracket"},
	)
	if err == nil {
		t.Fatal("Create returned nil error without authenticated user")
	}
	if repo.createRequest != nil {
		t.Fatal("repository Create was called without authenticated user")
	}
}

func TestCreateTrimsNameAndPassesUser(t *testing.T) {
	repo := &stubRepository{}
	service := NewService(repo)
	ctx := auth.ContextWithUserID(context.Background(), "keycloak-sub-1")

	part, err := service.Create(
		ctx,
		"11111111-1111-1111-1111-111111111111",
		&model.CreatePart3DRequest{Name: "  Bracket  "},
	)
	if err != nil {
		t.Fatalf("Create returned error: %v", err)
	}
	if part == nil {
		t.Fatal("Create returned nil part")
	}
	if repo.createWorkspaceID != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("workspaceID = %q", repo.createWorkspaceID)
	}
	if repo.createUserID != "keycloak-sub-1" {
		t.Fatalf("createdByUserID = %q", repo.createUserID)
	}
	if repo.createRequest.Name != "Bracket" {
		t.Fatalf("name = %q, want trimmed name", repo.createRequest.Name)
	}
}
