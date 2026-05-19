package v1

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/dnonakolesax/cccad-locks/internal/auth"
	"github.com/dnonakolesax/cccad-locks/internal/model"
	"github.com/mailru/easyjson"
)

type PermissionsService interface {
	List(ctx context.Context, sketchID string) ([]model.Permission, error)
	Put(ctx context.Context, permission *model.Permission) (*model.Permission, error)
	Delete(ctx context.Context, userID, sketchID string) error
}

type PermissionsHandler struct {
	service PermissionsService
}

func NewPermissionsHandler(service PermissionsService) *PermissionsHandler {
	return &PermissionsHandler{
		service: service,
	}
}

func (h *PermissionsHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/v1/sketches/{sketchId}/permissions", h.Get)
	mux.HandleFunc("PUT /api/v1/sketches/{sketchId}/permissions/{userId}", h.Put)
	mux.HandleFunc("DELETE /api/v1/sketches/{sketchId}/permissions/{userId}", h.Delete)
}

func (h *PermissionsHandler) Get(w http.ResponseWriter, r *http.Request) {
	sketchID := r.PathValue("sketchId")
	if strings.TrimSpace(sketchID) == "" {
		writeError(w, http.StatusBadRequest, "INVALID_OPERATION", "sketchId is required")
		return
	}

	permissions, err := h.service.List(r.Context(), sketchID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}

	writeJSON(w, http.StatusOK, &model.PermissionList{
		SketchID:    sketchID,
		Permissions: permissions,
	})
}

func (h *PermissionsHandler) Put(w http.ResponseWriter, r *http.Request) {
	sketchID := r.PathValue("sketchId")
	userID := r.PathValue("userId")
	if strings.TrimSpace(sketchID) == "" || strings.TrimSpace(userID) == "" {
		writeError(w, http.StatusBadRequest, "INVALID_OPERATION", "sketchId and userId are required")
		return
	}

	var request model.SetPermissionRequest
	if err := readJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_OPERATION", err.Error())
		return
	}
	if !isValidRole(request.Role) {
		writeError(w, http.StatusBadRequest, "INVALID_OPERATION", "role must be reader, editor, or admin")
		return
	}

	grantedByUserID, _ := auth.UserIDFromContext(r.Context())
	permission, err := h.service.Put(r.Context(), &model.Permission{
		SketchID:        sketchID,
		UserID:          userID,
		Role:            request.Role,
		GrantedByUserID: &grantedByUserID,
	})
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	if permission == nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "permission service returned nil permission")
		return
	}

	writeJSON(w, http.StatusOK, permission)
}

func (h *PermissionsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	sketchID := r.PathValue("sketchId")
	userID := r.PathValue("userId")
	if strings.TrimSpace(sketchID) == "" || strings.TrimSpace(userID) == "" {
		writeError(w, http.StatusBadRequest, "INVALID_OPERATION", "sketchId and userId are required")
		return
	}

	if err := h.service.Delete(r.Context(), userID, sketchID); err != nil {
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

func isValidRole(role string) bool {
	switch role {
	case "reader", "editor", "admin":
		return true
	default:
		return false
	}
}
