package v1

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"

	"github.com/dnonakolesax/cccad-locks/internal/model"
	"github.com/mailru/easyjson"
)

type OperationsService interface {
	List(ctx context.Context, sketchID string, afterVersion int64, limit int) (*model.SketchOperationPage, error)
	History(ctx context.Context, sketchID string, limit int) (*model.SketchOperationPage, error)
	Submit(
		ctx context.Context,
		sketchID string,
		request *model.SubmitOperationRequest,
	) (*model.SubmitOperationResponse, error)
}

type OperationsHandler struct {
	service OperationsService
}

const (
	defaultOperationLimit = 200
	minOperationLimit     = 1
	maxOperationLimit     = 1000
	minAfterVersion       = 0
)

func NewOperationsHandler(service OperationsService) *OperationsHandler {
	return &OperationsHandler{
		service: service,
	}
}

func (h *OperationsHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /{sketchId}/ops", h.List)
	mux.HandleFunc("GET /{sketchId}/model-history", h.History)
}

func (h *OperationsHandler) List(w http.ResponseWriter, r *http.Request) {
	sketchID := r.PathValue("sketchId")
	if strings.TrimSpace(sketchID) == "" {
		writeError(w, http.StatusBadRequest, "INVALID_OPERATION", "sketchId is required")
		return
	}

	afterVersion, err := parseInt64Query(r, "afterVersion", 0, minAfterVersion)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_OPERATION", err.Error())
		return
	}
	limit, err := parseIntQuery(r, "limit", defaultOperationLimit, minOperationLimit, maxOperationLimit)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_OPERATION", err.Error())
		return
	}

	page, err := h.service.List(r.Context(), sketchID, afterVersion, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	if page == nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "operations service returned nil page")
		return
	}

	writeJSON(w, http.StatusOK, page)
}

func (h *OperationsHandler) History(w http.ResponseWriter, r *http.Request) {
	sketchID := r.PathValue("sketchId")
	if strings.TrimSpace(sketchID) == "" {
		writeError(w, http.StatusBadRequest, "INVALID_OPERATION", "sketchId is required")
		return
	}

	limit, err := parseIntQuery(r, "limit", defaultOperationLimit, minOperationLimit, maxOperationLimit)
	if err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_OPERATION", err.Error())
		return
	}

	page, err := h.service.History(r.Context(), sketchID, limit)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	if page == nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", "operations service returned nil history page")
		return
	}

	writeJSON(w, http.StatusOK, page)
}

func (h *OperationsHandler) Submit(w http.ResponseWriter, r *http.Request) {
	sketchID := r.PathValue("sketchId")
	if strings.TrimSpace(sketchID) == "" {
		writeError(w, http.StatusBadRequest, "INVALID_OPERATION", "sketchId is required")
		return
	}

	var request model.SubmitOperationRequest
	if err := readJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_OPERATION", err.Error())
		return
	}
	if request.BaseVersion < 0 {
		writeError(w, http.StatusBadRequest, "INVALID_OPERATION", "baseVersion must be greater than or equal to 0")
		return
	}
	if strings.TrimSpace(request.ClientOpID) == "" {
		writeError(w, http.StatusBadRequest, "INVALID_OPERATION", "clientOpId is required")
		return
	}
	if len(request.Op) == 0 {
		writeError(w, http.StatusBadRequest, "INVALID_OPERATION", "op is required")
		return
	}

	response, err := h.service.Submit(r.Context(), sketchID, &request)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "INTERNAL_ERROR", err.Error())
		return
	}
	if response == nil {
		writeError(
			w,
			http.StatusInternalServerError,
			"INTERNAL_ERROR",
			"operations service returned nil submit response",
		)
		return
	}

	writeJSON(w, http.StatusOK, response)
}

func parseInt64Query(r *http.Request, name string, defaultValue, minValue int64) (int64, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(name))
	if raw == "" {
		return defaultValue, nil
	}

	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer", name)
	}
	if value < minValue {
		return 0, fmt.Errorf("%s must be greater than or equal to %d", name, minValue)
	}

	return value, nil
}

func parseIntQuery(r *http.Request, name string, defaultValue, minValue, maxValue int) (int, error) {
	raw := strings.TrimSpace(r.URL.Query().Get(name))
	if raw == "" {
		return defaultValue, nil
	}

	value, err := strconv.Atoi(raw)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer", name)
	}
	if value < minValue || value > maxValue {
		return 0, fmt.Errorf("%s must be between %d and %d", name, minValue, maxValue)
	}

	return value, nil
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
