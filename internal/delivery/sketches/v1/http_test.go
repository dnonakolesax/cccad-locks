package v1

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dnonakolesax/cccad-locks/internal/model"
)

type stubSketchesService struct {
	getCalled    bool
	createCalled bool
	snapshotErr  error
}

func (s *stubSketchesService) Create(
	_ context.Context,
	_ string,
	_ *model.CreateSketchRequest,
) (*model.SketchMetadata, error) {
	s.createCalled = true
	return &model.SketchMetadata{}, nil
}

func (s *stubSketchesService) ListAvailable(_ context.Context) ([]model.AvailableSketch, error) {
	return nil, nil
}

func (s *stubSketchesService) Get(_ context.Context, _ string) (*model.SketchDocument, error) {
	s.getCalled = true
	return &model.SketchDocument{}, nil
}

func (s *stubSketchesService) Snapshot(_ context.Context, _ string, _ int64) (*model.SketchSnapshot, error) {
	if s.snapshotErr != nil {
		return nil, s.snapshotErr
	}
	return &model.SketchSnapshot{}, nil
}

func (s *stubSketchesService) RevertToVersion(_ context.Context, _ string, _ int64) (*model.SketchDocument, error) {
	return &model.SketchDocument{}, nil
}

func (s *stubSketchesService) UpdateMetadata(
	_ context.Context,
	_ string,
	_ *model.UpdateSketchMetadataRequest,
) (*model.SketchMetadata, error) {
	return &model.SketchMetadata{}, nil
}

func (s *stubSketchesService) Delete(_ context.Context, _ string) error {
	return nil
}

func TestGetRejectsInvalidSketchID(t *testing.T) {
	service := &stubSketchesService{}
	mux := http.NewServeMux()
	NewSketchesHandler(service).RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/sketches", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if service.getCalled {
		t.Fatal("service.Get was called for invalid sketch id")
	}
	if !strings.Contains(rec.Body.String(), "sketchId must be a valid uuid") {
		t.Fatalf("body = %q, want invalid uuid message", rec.Body.String())
	}
}

func TestCreateRejectsInvalidWorkspaceID(t *testing.T) {
	service := &stubSketchesService{}
	mux := http.NewServeMux()
	NewSketchesHandler(service).RegisterRoutes(mux)

	req := httptest.NewRequest(
		http.MethodPost,
		"/workspaces/sketches/sketches",
		strings.NewReader(`{"name":"test","unit":"mm"}`),
	)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if service.createCalled {
		t.Fatal("service.Create was called for invalid workspace id")
	}
	if !strings.Contains(rec.Body.String(), "workspaceId must be a valid uuid") {
		t.Fatalf("body = %q, want invalid uuid message", rec.Body.String())
	}
}

func TestCreateRequiresPlane(t *testing.T) {
	service := &stubSketchesService{}
	mux := http.NewServeMux()
	NewSketchesHandler(service).RegisterRoutes(mux)

	req := httptest.NewRequest(
		http.MethodPost,
		"/workspaces/11111111-1111-1111-1111-111111111111/sketches",
		strings.NewReader(`{"name":"test","unit":"mm"}`),
	)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusBadRequest {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusBadRequest, rec.Body.String())
	}
	if service.createCalled {
		t.Fatal("service.Create was called without plane")
	}
	if !strings.Contains(rec.Body.String(), "plane is required") {
		t.Fatalf("body = %q, want plane required message", rec.Body.String())
	}
}

func TestSnapshotUnavailableReturnsNotFound(t *testing.T) {
	service := &stubSketchesService{
		snapshotErr: errors.New("get sketch snapshot returned no rows"),
	}
	mux := http.NewServeMux()
	NewSketchesHandler(service).RegisterRoutes(mux)

	req := httptest.NewRequest(
		http.MethodGet,
		"/snapshots/11111111-1111-1111-1111-111111111111/22",
		nil,
	)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code != http.StatusNotFound {
		t.Fatalf("status = %d, want %d; body: %s", rec.Code, http.StatusNotFound, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), "SNAPSHOT_NOT_FOUND") {
		t.Fatalf("body = %q, want SNAPSHOT_NOT_FOUND", rec.Body.String())
	}
}
