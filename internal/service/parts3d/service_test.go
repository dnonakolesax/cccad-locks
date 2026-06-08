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
	listWorkspaceID   string
	deletePartID      string
	listRepsPartID    string
	listRepsKind      *string
	getRepPartID      string
	getRepID          string
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

func (r *stubRepository) ListByWorkspace(_ context.Context, workspaceID string) ([]model.Part3D, error) {
	r.listWorkspaceID = workspaceID
	return []model.Part3D{
		{
			ID:          "22222222-2222-2222-2222-222222222222",
			WorkspaceID: workspaceID,
			Name:        "Bracket",
		},
	}, nil
}

func (r *stubRepository) Delete(_ context.Context, partID string) error {
	r.deletePartID = partID
	return nil
}

func (r *stubRepository) ListFeatures(context.Context, string, bool) ([]model.Feature3D, error) {
	return nil, nil
}

func (r *stubRepository) ListBodies(context.Context, string) ([]model.Body3D, error) {
	return nil, nil
}

func (r *stubRepository) ListRepresentations(
	_ context.Context,
	partID string,
	kind *string,
) ([]model.Representation3D, error) {
	r.listRepsPartID = partID
	r.listRepsKind = kind
	return []model.Representation3D{
		{
			ID:              "33333333-3333-3333-3333-333333333333",
			PartID:          partID,
			Kind:            "glb",
			StorageKey:      "parts/part-1/body.glb",
			DocumentVersion: 7,
		},
	}, nil
}

func (r *stubRepository) GetRepresentation(
	_ context.Context,
	partID string,
	representationID string,
) (*model.Representation3D, error) {
	r.getRepPartID = partID
	r.getRepID = representationID
	return &model.Representation3D{
		ID:              representationID,
		PartID:          partID,
		Kind:            "glb",
		StorageKey:      "parts/part-1/body.glb",
		DocumentVersion: 7,
	}, nil
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

func TestListByWorkspacePassesWorkspaceID(t *testing.T) {
	repo := &stubRepository{}
	service := NewService(repo)

	response, err := service.ListByWorkspace(context.Background(), "11111111-1111-1111-1111-111111111111")
	if err != nil {
		t.Fatalf("ListByWorkspace returned error: %v", err)
	}
	if response == nil {
		t.Fatal("ListByWorkspace returned nil response")
	}
	if repo.listWorkspaceID != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("workspaceID = %q", repo.listWorkspaceID)
	}
	if len(response.Parts) != 1 {
		t.Fatalf("parts length = %d, want 1", len(response.Parts))
	}
}

func TestDeleteRequiresAuthenticatedUser(t *testing.T) {
	repo := &stubRepository{}
	service := NewService(repo)

	err := service.Delete(context.Background(), "22222222-2222-2222-2222-222222222222")
	if err == nil {
		t.Fatal("Delete returned nil error without authenticated user")
	}
	if repo.deletePartID != "" {
		t.Fatal("repository Delete was called without authenticated user")
	}
}

func TestDeletePassesPartID(t *testing.T) {
	repo := &stubRepository{}
	service := NewService(repo)
	ctx := auth.ContextWithUserID(context.Background(), "keycloak-sub-1")

	if err := service.Delete(ctx, "22222222-2222-2222-2222-222222222222"); err != nil {
		t.Fatalf("Delete returned error: %v", err)
	}
	if repo.deletePartID != "22222222-2222-2222-2222-222222222222" {
		t.Fatalf("partID = %q", repo.deletePartID)
	}
}

func TestListRepresentationsValidatesKindAndPassesFilter(t *testing.T) {
	repo := &stubRepository{}
	service := NewService(repo)
	kind := " glb "

	response, err := service.ListRepresentations(
		context.Background(),
		"22222222-2222-2222-2222-222222222222",
		&kind,
	)
	if err != nil {
		t.Fatalf("ListRepresentations returned error: %v", err)
	}
	if response == nil || len(response.Representations) != 1 {
		t.Fatalf("response = %#v", response)
	}
	if repo.listRepsPartID != "22222222-2222-2222-2222-222222222222" {
		t.Fatalf("partID = %q", repo.listRepsPartID)
	}
	if repo.listRepsKind == nil || *repo.listRepsKind != "glb" {
		t.Fatalf("kind = %#v, want trimmed glb", repo.listRepsKind)
	}
}

func TestListRepresentationsRejectsInvalidKind(t *testing.T) {
	repo := &stubRepository{}
	service := NewService(repo)
	kind := "obj"

	_, err := service.ListRepresentations(
		context.Background(),
		"22222222-2222-2222-2222-222222222222",
		&kind,
	)
	if err == nil {
		t.Fatal("ListRepresentations returned nil error for invalid kind")
	}
	if repo.listRepsPartID != "" {
		t.Fatal("repository ListRepresentations was called for invalid kind")
	}
}

func TestGetRepresentationPassesPartAndRepresentationID(t *testing.T) {
	repo := &stubRepository{}
	service := NewService(repo)

	response, err := service.GetRepresentation(
		context.Background(),
		"22222222-2222-2222-2222-222222222222",
		"33333333-3333-3333-3333-333333333333",
	)
	if err != nil {
		t.Fatalf("GetRepresentation returned error: %v", err)
	}
	if response == nil {
		t.Fatal("GetRepresentation returned nil response")
	}
	if repo.getRepPartID != "22222222-2222-2222-2222-222222222222" {
		t.Fatalf("partID = %q", repo.getRepPartID)
	}
	if repo.getRepID != "33333333-3333-3333-3333-333333333333" {
		t.Fatalf("representationID = %q", repo.getRepID)
	}
}

func TestGetRepresentationRejectsInvalidRepresentationID(t *testing.T) {
	repo := &stubRepository{}
	service := NewService(repo)

	_, err := service.GetRepresentation(
		context.Background(),
		"22222222-2222-2222-2222-222222222222",
		"not-a-uuid",
	)
	if err == nil {
		t.Fatal("GetRepresentation returned nil error for invalid representationID")
	}
	if repo.getRepID != "" {
		t.Fatal("repository GetRepresentation was called for invalid representationID")
	}
}
