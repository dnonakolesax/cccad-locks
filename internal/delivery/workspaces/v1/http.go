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

type WorkspacesService interface {
	Create(ctx context.Context, request *model.CreateWorkspaceRequest) (*model.Workspace, error)
	ListAvailable(ctx context.Context) ([]model.Workspace, error)
	Update(ctx context.Context, workspaceID string, request *model.UpdateWorkspaceRequest) (*model.Workspace, error)
	Delete(ctx context.Context, workspaceID string) error
}

type WorkspacesHandler struct {
	service WorkspacesService
}

func NewWorkspacesHandler(service WorkspacesService) *WorkspacesHandler {
	return &WorkspacesHandler{
		service: service,
	}
}

func (h *WorkspacesHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("POST /workspaces", h.Create)
	mux.HandleFunc("GET /workspaces", h.ListAvailable)
	mux.HandleFunc("PATCH /workspaces/{workspaceId}", h.Update)
	mux.HandleFunc("DELETE /workspaces/{workspaceId}", h.Delete)
}

func (h *WorkspacesHandler) Create(w http.ResponseWriter, r *http.Request) {
	var request model.CreateWorkspaceRequest
	if err := readJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_OPERATION", err.Error())
		return
	}
	if strings.TrimSpace(request.Name) == "" {
		writeError(w, http.StatusBadRequest, "INVALID_OPERATION", "name is required")
		return
	}

	workspace, err := h.service.Create(r.Context(), &request)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	if workspace == nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "workspaces service returned nil workspace")
		return
	}

	writeJSON(w, http.StatusCreated, workspace)
}

func (h *WorkspacesHandler) ListAvailable(w http.ResponseWriter, r *http.Request) {
	workspaces, err := h.service.ListAvailable(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, &model.WorkspaceList{Workspaces: workspaces})
}

func (h *WorkspacesHandler) Update(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceId")
	if strings.TrimSpace(workspaceID) == "" {
		writeError(w, http.StatusBadRequest, "INVALID_OPERATION", "workspaceId is required")
		return
	}
	if !isValidUUID(workspaceID) {
		writeError(w, http.StatusBadRequest, "INVALID_OPERATION", "workspaceId must be a valid uuid")
		return
	}

	var request model.UpdateWorkspaceRequest
	if err := readJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_OPERATION", err.Error())
		return
	}
	if request.Name == nil && request.Description == nil {
		writeError(w, http.StatusBadRequest, "INVALID_OPERATION", "name or description is required")
		return
	}
	if request.Name != nil && strings.TrimSpace(*request.Name) == "" {
		writeError(w, http.StatusBadRequest, "INVALID_OPERATION", "name must not be empty")
		return
	}

	workspace, err := h.service.Update(r.Context(), workspaceID, &request)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	if workspace == nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "workspaces service returned nil workspace")
		return
	}

	writeJSON(w, http.StatusOK, workspace)
}

func (h *WorkspacesHandler) Delete(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceId")
	if strings.TrimSpace(workspaceID) == "" {
		writeError(w, http.StatusBadRequest, "INVALID_OPERATION", "workspaceId is required")
		return
	}
	if !isValidUUID(workspaceID) {
		writeError(w, http.StatusBadRequest, "INVALID_OPERATION", "workspaceId must be a valid uuid")
		return
	}

	if err := h.service.Delete(r.Context(), workspaceID); err != nil {
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
