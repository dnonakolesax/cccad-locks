package application

import (
	"net/http"
	"net/http/httptest"
	"testing"

	commentsDelivery "github.com/dnonakolesax/cccad-locks/internal/delivery/comments/v1"
	locksDelivery "github.com/dnonakolesax/cccad-locks/internal/delivery/locks/v1"
	operationsDelivery "github.com/dnonakolesax/cccad-locks/internal/delivery/operations/v1"
	parts3dDelivery "github.com/dnonakolesax/cccad-locks/internal/delivery/parts3d/v1"
	permissionsDelivery "github.com/dnonakolesax/cccad-locks/internal/delivery/permissions/v1"
	realtimeDelivery "github.com/dnonakolesax/cccad-locks/internal/delivery/realtime/v1"
	sketchesDelivery "github.com/dnonakolesax/cccad-locks/internal/delivery/sketches/v1"
	solverDelivery "github.com/dnonakolesax/cccad-locks/internal/delivery/solver/v1"
	workspacesDelivery "github.com/dnonakolesax/cccad-locks/internal/delivery/workspaces/v1"
)

func TestAPIMountPathPlacesBasePathAfterVersion(t *testing.T) {
	tests := map[string]string{
		"/sketches":  "/api/v1/sketches",
		"sketches":   "/api/v1/sketches",
		"/sketches/": "/api/v1/sketches",
		"/":          "/api/v1",
		"":           "/api/v1",
	}

	for basePath, want := range tests {
		if got := apiMountPath(basePath); got != want {
			t.Fatalf("apiMountPath(%q) = %q, want %q", basePath, got, want)
		}
	}
}

func TestStripPrefixWithRootMapsExactMountToRoot(t *testing.T) {
	router := http.NewServeMux()
	router.HandleFunc("GET /", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	})

	handler := stripPrefixWithRoot("/api/v1/sketches", router)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/sketches", nil)
	rec := httptest.NewRecorder()

	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusNoContent {
		t.Fatalf("status = %d, want %d", rec.Code, http.StatusNoContent)
	}
}

func TestRegisterRoutesHasNoServeMuxPatternConflicts(t *testing.T) {
	app := &App{
		layers: &Layers{
			CommentsHTTP:    commentsDelivery.NewCommentsHandler(nil),
			PermissionsHTTP: permissionsDelivery.NewPermissionsHandler(nil),
			LocksHTTP:       locksDelivery.NewLocksHandler(nil),
			OperationsHTTP:  operationsDelivery.NewOperationsHandler(nil),
			Parts3DHTTP:     parts3dDelivery.NewParts3DHandler(nil),
			Parts3DWS:       parts3dDelivery.NewParts3DWSHandler(),
			SketchesHTTP:    sketchesDelivery.NewSketchesHandler(nil),
			RealtimeWS:      realtimeDelivery.NewHandler(nil, nil, realtimeDelivery.WithRoutePrefix("/")),
			SolverHTTP:      solverDelivery.NewSolverHandler(nil),
			WorkspacesHTTP:  workspacesDelivery.NewWorkspacesHandler(nil),
		},
	}

	router := http.NewServeMux()
	app.registerRoutes(router)
}
