package v1

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dnonakolesax/cccad-locks/internal/model"
)

type stubParts3DService struct {
	createCalled      bool
	createWorkspaceID string
	createRequest     *model.CreatePart3DRequest
	listCalled        bool
	listWorkspaceID   string
	deleteCalled      bool
	deletePartID      string
	listRepsPartID    string
	listRepsKind      *string
}

func (s *stubParts3DService) Create(
	_ context.Context,
	workspaceID string,
	request *model.CreatePart3DRequest,
) (*model.Part3D, error) {
	s.createCalled = true
	s.createWorkspaceID = workspaceID
	s.createRequest = request
	return &model.Part3D{
		ID:          "22222222-2222-2222-2222-222222222222",
		WorkspaceID: workspaceID,
		Name:        request.Name,
	}, nil
}

func (s *stubParts3DService) ListByWorkspace(
	_ context.Context,
	workspaceID string,
) (*model.Part3DList, error) {
	s.listCalled = true
	s.listWorkspaceID = workspaceID
	return &model.Part3DList{
		Parts: []model.Part3D{
			{
				ID:          "22222222-2222-2222-2222-222222222222",
				WorkspaceID: workspaceID,
				Name:        "Bracket",
			},
		},
	}, nil
}

func (s *stubParts3DService) Delete(_ context.Context, partID string) error {
	s.deleteCalled = true
	s.deletePartID = partID
	return nil
}

func (s *stubParts3DService) ListFeatures(context.Context, string, bool) (*model.Feature3DList, error) {
	return nil, nil
}

func (s *stubParts3DService) ListBodies(context.Context, string) (*model.Body3DList, error) {
	return nil, nil
}

func (s *stubParts3DService) ListRepresentations(
	_ context.Context,
	partID string,
	kind *string,
) (*model.Representation3DList, error) {
	s.listRepsPartID = partID
	s.listRepsKind = kind
	return &model.Representation3DList{
		Representations: []model.Representation3D{
			{
				ID:              "33333333-3333-3333-3333-333333333333",
				PartID:          partID,
				BodyID:          "44444444-4444-4444-4444-444444444444",
				Kind:            "glb",
				StorageKey:      "parts/part-1/body.glb",
				DocumentVersion: 7,
			},
		},
	}, nil
}

func (s *stubParts3DService) GetTopology(context.Context, string, *string) (*model.TopologySummary3D, error) {
	return nil, nil
}

func (s *stubParts3DService) GetFacePlane(context.Context, string, string, string) (*model.FacePlane3D, error) {
	return nil, nil
}

func TestCreateRejectsMissingPartName(t *testing.T) {
	service := &stubParts3DService{}
	mux := http.NewServeMux()
	NewParts3DHandler(service).RegisterRoutes(mux)

	req := httptest.NewRequest(
		http.MethodPost,
		"/workspaces/11111111-1111-1111-1111-111111111111/parts",
		strings.NewReader(`{"name":" "}`),
	)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if service.createCalled {
		t.Fatal("service.Create was called for missing part name")
	}
	if !strings.Contains(rec.Body.String(), "name is required") {
		t.Fatalf("body = %q, want name required message", rec.Body.String())
	}
}

func TestCreatePartCallsService(t *testing.T) {
	service := &stubParts3DService{}
	mux := http.NewServeMux()
	NewParts3DHandler(service).RegisterRoutes(mux)

	req := httptest.NewRequest(
		http.MethodPost,
		"/workspaces/11111111-1111-1111-1111-111111111111/parts",
		strings.NewReader(`{"name":"Bracket"}`),
	)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusCreated, rec.Body.String())
	}
	if !service.createCalled {
		t.Fatal("service.Create was not called")
	}
	if service.createWorkspaceID != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("workspaceID = %q", service.createWorkspaceID)
	}
	if service.createRequest == nil || service.createRequest.Name != "Bracket" {
		t.Fatalf("request = %#v", service.createRequest)
	}
}

func TestListPartsByWorkspaceCallsService(t *testing.T) {
	service := &stubParts3DService{}
	mux := http.NewServeMux()
	NewParts3DHandler(service).RegisterRoutes(mux)

	req := httptest.NewRequest(
		http.MethodGet,
		"/workspaces/11111111-1111-1111-1111-111111111111/parts",
		nil,
	)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if !service.listCalled {
		t.Fatal("service.ListByWorkspace was not called")
	}
	if service.listWorkspaceID != "11111111-1111-1111-1111-111111111111" {
		t.Fatalf("workspaceID = %q", service.listWorkspaceID)
	}
	if !strings.Contains(rec.Body.String(), `"parts"`) {
		t.Fatalf("body = %q, want parts response", rec.Body.String())
	}
}

func TestDeletePartCallsService(t *testing.T) {
	service := &stubParts3DService{}
	mux := http.NewServeMux()
	NewParts3DHandler(service).RegisterRoutes(mux)

	req := httptest.NewRequest(
		http.MethodDelete,
		"/parts/22222222-2222-2222-2222-222222222222",
		nil,
	)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusNoContent, rec.Body.String())
	}
	if !service.deleteCalled {
		t.Fatal("service.Delete was not called")
	}
	if service.deletePartID != "22222222-2222-2222-2222-222222222222" {
		t.Fatalf("partID = %q", service.deletePartID)
	}
}

func TestListRepresentationsCallsService(t *testing.T) {
	service := &stubParts3DService{}
	mux := http.NewServeMux()
	NewParts3DHandler(service).RegisterRoutes(mux)

	req := httptest.NewRequest(
		http.MethodGet,
		"/parts/22222222-2222-2222-2222-222222222222/representations?kind=glb",
		nil,
	)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusOK, rec.Body.String())
	}
	if service.listRepsPartID != "22222222-2222-2222-2222-222222222222" {
		t.Fatalf("partID = %q", service.listRepsPartID)
	}
	if service.listRepsKind == nil || *service.listRepsKind != "glb" {
		t.Fatalf("kind = %#v, want glb", service.listRepsKind)
	}
	if !strings.Contains(rec.Body.String(), `"representations"`) {
		t.Fatalf("body = %q, want representations response", rec.Body.String())
	}
}
