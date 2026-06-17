package v1

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestRealtimeRouteExtractsSketchIDFromServeMuxPathValue(t *testing.T) {
	handler := NewHandler(nil, nil, WithRoutePrefix("/"))
	mux := http.NewServeMux()
	handler.RegisterRoutes(mux)

	req := httptest.NewRequest(http.MethodGet, "/realtime/c1a78051-a5d5-47c8-8247-5046570039e4/ws?lastSeenVersion=22", nil)
	rec := httptest.NewRecorder()

	mux.ServeHTTP(rec, req)

	if rec.Code == http.StatusNotFound {
		t.Fatalf("status = %d, route should match and reach auth handling", rec.Code)
	}
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want %d without auth context", rec.Code, http.StatusUnauthorized)
	}
}
