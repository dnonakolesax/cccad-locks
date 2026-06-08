package v1

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/dnonakolesax/cccad-locks/internal/model"
	parts3dService "github.com/dnonakolesax/cccad-locks/internal/service/parts3d"
)

type Parts3DService interface {
	Create(ctx context.Context, workspaceID string, request *model.CreatePart3DRequest) (*model.Part3D, error)
	ListByWorkspace(ctx context.Context, workspaceID string) (*model.Part3DList, error)
	ListFeatures(ctx context.Context, partID string, includeSuppressed bool) (*model.Feature3DList, error)
	ListBodies(ctx context.Context, partID string) (*model.Body3DList, error)
	GetTopology(ctx context.Context, partID string, bodyID *string) (*model.TopologySummary3D, error)
	GetFacePlane(ctx context.Context, partID string, bodyID string, faceID string) (*model.FacePlane3D, error)
}

type Parts3DHandler struct {
	service Parts3DService
}

func NewParts3DHandler(service Parts3DService) *Parts3DHandler {
	return &Parts3DHandler{service: service}
}

func (h *Parts3DHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /workspaces/{workspaceId}/parts", h.Create)
	mux.HandleFunc("GET /workspaces/{workspaceId}/parts", h.ListByWorkspace)
	mux.HandleFunc("GET /parts/{partId}/features", h.ListFeatures)
	mux.HandleFunc("GET /parts/{partId}/bodies", h.ListBodies)
	mux.HandleFunc("GET /parts/{partId}/topology", h.GetTopology)
	mux.HandleFunc("GET /parts/{partId}/bodies/{bodyId}/faces/{faceId}/plane", h.GetFacePlane)
}

func (h *Parts3DHandler) Create(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceId")
	if strings.TrimSpace(workspaceID) == "" {
		writeJSONError(w, http.StatusBadRequest, "INVALID_OPERATION", "workspaceId is required")
		return
	}

	var request model.CreatePart3DRequest
	if err := readJSON(r, &request); err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_OPERATION", err.Error())
		return
	}
	if strings.TrimSpace(request.Name) == "" {
		writeJSONError(w, http.StatusBadRequest, "INVALID_OPERATION", "name is required")
		return
	}

	response, err := h.service.Create(r.Context(), workspaceID, &request)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if response == nil {
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "3d parts service returned nil part")
		return
	}

	writeJSON(w, http.StatusCreated, response)
}

func (h *Parts3DHandler) ListByWorkspace(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceId")
	if strings.TrimSpace(workspaceID) == "" {
		writeJSONError(w, http.StatusBadRequest, "INVALID_OPERATION", "workspaceId is required")
		return
	}

	response, err := h.service.ListByWorkspace(r.Context(), workspaceID)
	if err != nil {
		writeServiceError(w, err)
		return
	}
	if response == nil {
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "3d parts service returned nil part list")
		return
	}

	writeJSON(w, http.StatusOK, response)
}

func (h *Parts3DHandler) ListFeatures(w http.ResponseWriter, r *http.Request) {
	includeSuppressed, err := boolQueryDefault(r, "includeSuppressed", true)
	if err != nil {
		writeJSONError(w, http.StatusBadRequest, "INVALID_OPERATION", err.Error())
		return
	}

	response, err := h.service.ListFeatures(r.Context(), r.PathValue("partId"), includeSuppressed)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, response)
}

func (h *Parts3DHandler) ListBodies(w http.ResponseWriter, r *http.Request) {
	response, err := h.service.ListBodies(r.Context(), r.PathValue("partId"))
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, response)
}

func (h *Parts3DHandler) GetTopology(w http.ResponseWriter, r *http.Request) {
	var bodyID *string
	if value := strings.TrimSpace(r.URL.Query().Get("bodyId")); value != "" {
		bodyID = &value
	}

	response, err := h.service.GetTopology(r.Context(), r.PathValue("partId"), bodyID)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, response)
}

func (h *Parts3DHandler) GetFacePlane(w http.ResponseWriter, r *http.Request) {
	response, err := h.service.GetFacePlane(
		r.Context(),
		r.PathValue("partId"),
		r.PathValue("bodyId"),
		r.PathValue("faceId"),
	)
	if err != nil {
		writeServiceError(w, err)
		return
	}

	writeJSON(w, http.StatusOK, response)
}

func boolQueryDefault(r *http.Request, name string, fallback bool) (bool, error) {
	value := strings.TrimSpace(r.URL.Query().Get(name))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, errors.New(name + " must be a boolean")
	}
	return parsed, nil
}

func readJSON(r *http.Request, value any) error {
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

	return json.Unmarshal(body, value)
}

func writeServiceError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, parts3dService.ErrFacePlaneNotFound):
		writeJSONError(w, http.StatusNotFound, "INVALID_REFERENCE", err.Error())
	case strings.Contains(err.Error(), "must be a valid uuid"),
		strings.Contains(err.Error(), "is required"):
		writeJSONError(w, http.StatusBadRequest, "INVALID_OPERATION", err.Error())
	default:
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
	}
}

func writeJSON(w http.ResponseWriter, status int, value any) {
	body, err := json.Marshal(value)
	if err != nil {
		writeJSONError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write(body)
}

func writeJSONError(w http.ResponseWriter, status int, code, message string) {
	writeJSON(w, status, &model.ErrorEnvelope{
		Error: model.ErrorObject{
			Code:    code,
			Message: message,
		},
	})
}
