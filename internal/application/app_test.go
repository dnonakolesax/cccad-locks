package application

import (
	"net/http"
	"net/http/httptest"
	"testing"
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
