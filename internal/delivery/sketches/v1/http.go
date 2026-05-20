package v1

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/dnonakolesax/cccad-locks/internal/model"
	"github.com/mailru/easyjson"
)

type SketchesService interface {
	Create(ctx context.Context, workspaceID string, request *model.CreateSketchRequest) (*model.SketchMetadata, error)
	ListAvailable(ctx context.Context) ([]model.AvailableSketch, error)
	Get(ctx context.Context, sketchID string) (*model.SketchDocument, error)
	UpdateMetadata(ctx context.Context, sketchID string, request *model.UpdateSketchMetadataRequest) (*model.SketchMetadata, error)
	Delete(ctx context.Context, sketchID string) error
}

type SketchesHandler struct {
	service SketchesService
}

func NewSketchesHandler(service SketchesService) *SketchesHandler {
	return &SketchesHandler{
		service: service,
	}
}

func (h *SketchesHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /workspaces/{workspaceId}/sketches", h.Create)
	mux.HandleFunc("GET /", h.ListAvailable)
	mux.HandleFunc("GET /{sketchId}", h.Get)
	mux.HandleFunc("PATCH /{sketchId}", h.UpdateMetadata)
	mux.HandleFunc("DELETE /{sketchId}", h.Delete)
}

func (h *SketchesHandler) Create(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceId")
	if strings.TrimSpace(workspaceID) == "" {
		writeError(w, http.StatusBadRequest, "INVALID_OPERATION", "workspaceId is required")
		return
	}
	if !isValidUUID(workspaceID) {
		writeError(w, http.StatusBadRequest, "INVALID_OPERATION", "workspaceId must be a valid uuid")
		return
	}

	var request model.CreateSketchRequest
	if err := readJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_OPERATION", err.Error())
		return
	}
	if strings.TrimSpace(request.Name) == "" {
		writeError(w, http.StatusBadRequest, "INVALID_OPERATION", "name is required")
		return
	}
	if request.Unit != "" && !isValidLengthUnit(request.Unit) {
		writeError(w, http.StatusBadRequest, "INVALID_OPERATION", "unit must be mm, cm, m, or inch")
		return
	}

	metadata, err := h.service.Create(r.Context(), workspaceID, &request)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	if metadata == nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "sketches service returned nil metadata")
		return
	}

	writeJSON(w, http.StatusCreated, metadata)
}

func (h *SketchesHandler) ListAvailable(w http.ResponseWriter, r *http.Request) {
	sketches, err := h.service.ListAvailable(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, &model.AvailableSketchList{Sketches: sketches})
}

func (h *SketchesHandler) Get(w http.ResponseWriter, r *http.Request) {
	sketchID := r.PathValue("sketchId")
	if strings.TrimSpace(sketchID) == "" {
		writeError(w, http.StatusBadRequest, "INVALID_OPERATION", "sketchId is required")
		return
	}
	if !isValidUUID(sketchID) {
		writeError(w, http.StatusBadRequest, "INVALID_OPERATION", "sketchId must be a valid uuid")
		return
	}

	document, err := h.service.Get(r.Context(), sketchID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	if document == nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "sketches service returned nil document")
		return
	}

	writeJSON(w, http.StatusOK, document)
}

func (h *SketchesHandler) UpdateMetadata(w http.ResponseWriter, r *http.Request) {
	sketchID := r.PathValue("sketchId")
	if strings.TrimSpace(sketchID) == "" {
		writeError(w, http.StatusBadRequest, "INVALID_OPERATION", "sketchId is required")
		return
	}
	if !isValidUUID(sketchID) {
		writeError(w, http.StatusBadRequest, "INVALID_OPERATION", "sketchId must be a valid uuid")
		return
	}

	var request model.UpdateSketchMetadataRequest
	if err := readJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_OPERATION", err.Error())
		return
	}
	if request.Name != nil && strings.TrimSpace(*request.Name) == "" {
		writeError(w, http.StatusBadRequest, "INVALID_OPERATION", "name must not be empty")
		return
	}
	if request.Unit != nil && !isValidLengthUnit(*request.Unit) {
		writeError(w, http.StatusBadRequest, "INVALID_OPERATION", "unit must be mm, cm, m, or inch")
		return
	}

	metadata, err := h.service.UpdateMetadata(r.Context(), sketchID, &request)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	if metadata == nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "sketches service returned nil metadata")
		return
	}

	writeJSON(w, http.StatusOK, metadata)
}

func (h *SketchesHandler) Delete(w http.ResponseWriter, r *http.Request) {
	sketchID := r.PathValue("sketchId")
	if strings.TrimSpace(sketchID) == "" {
		writeError(w, http.StatusBadRequest, "INVALID_OPERATION", "sketchId is required")
		return
	}
	if !isValidUUID(sketchID) {
		writeError(w, http.StatusBadRequest, "INVALID_OPERATION", "sketchId must be a valid uuid")
		return
	}

	if err := h.service.Delete(r.Context(), sketchID); err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

func readJSON(r *http.Request, v easyjson.Unmarshaler) error {
	defer r.Body.Close()

	const maxBodySize = 1 << 20

	body, err := io.ReadAll(io.LimitReader(r.Body, maxBodySize+1))
	if err != nil {
		return err
	}
	if len(body) > maxBodySize {
		return fmt.Errorf("request body exceeds %d bytes", maxBodySize)
	}
	if len(strings.TrimSpace(string(body))) == 0 {
		return errors.New("request body is required")
	}

	return easyjson.Unmarshal(body, v)
}

func writeJSON(w http.ResponseWriter, status int, v easyjson.Marshaler) {
	body, err := easyjson.Marshal(v)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	//nolint:gosec // JSON response is generated by easyjson and written with application/json content type.
	_, _ = w.Write(body)
}

func writeError(w http.ResponseWriter, status int, code, message string) {
	body, err := easyjson.Marshal(&model.ErrorEnvelope{
		Error: model.ErrorObject{
			Code:    code,
			Message: message,
		},
	})
	if err != nil {
		http.Error(w, http.StatusText(http.StatusInternalServerError), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func isValidLengthUnit(unit string) bool {
	switch unit {
	case "mm", "cm", "m", "inch":
		return true
	default:
		return false
	}
}

func isValidUUID(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != 36 {
		return false
	}

	for i, r := range value {
		switch i {
		case 8, 13, 18, 23:
			if r != '-' {
				return false
			}
		default:
			if !isHex(r) {
				return false
			}
		}
	}

	return true
}

func isHex(r rune) bool {
	return (r >= '0' && r <= '9') ||
		(r >= 'a' && r <= 'f') ||
		(r >= 'A' && r <= 'F')
}
