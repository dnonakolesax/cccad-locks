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

const (
	uuidLength = 36
	uuidDash1  = 8
	uuidDash2  = 13
	uuidDash3  = 18
	uuidDash4  = 23
)

type CommentsService interface {
	List(ctx context.Context, filter model.CommentListFilter) (*model.CommentListResponse, error)
	Get(ctx context.Context, commentID string) (*model.CadComment, error)
	SubscribeDocument(ctx context.Context, documentID string) (model.CommentSubscription, error)
	Create(ctx context.Context, workspaceID string, request *model.CreateCommentRequest) (*model.CadComment, error)
	Update(ctx context.Context, commentID string, request *model.UpdateCommentRequest) (*model.CadComment, error)
	Delete(ctx context.Context, commentID string) error
	ChangeStatus(
		ctx context.Context,
		commentID string,
		request *model.ChangeCommentStatusRequest,
	) (*model.CadComment, error)
	ReplaceAssignees(
		ctx context.Context,
		commentID string,
		request *model.ReplaceCommentAssigneesRequest,
	) (*model.CadComment, error)
	StatusHistory(ctx context.Context, commentID string) ([]model.CommentStatusHistoryItem, error)
	EditHistory(ctx context.Context, commentID string) ([]model.CommentEditHistoryItem, error)
}

type CommentsHandler struct {
	service CommentsService
}

func NewCommentsHandler(service CommentsService) *CommentsHandler {
	return &CommentsHandler{service: service}
}

func (h *CommentsHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /workspaces/{workspaceId}/comments", h.List)
	mux.HandleFunc("POST /workspaces/{workspaceId}/comments", h.Create)
	mux.HandleFunc("GET /realtime/ws/documents/{documentId}/comments", h.DocumentCommentsWebSocket)
	mux.HandleFunc("GET /api/v1/sketches/realtime/ws/documents/{documentId}/comments", h.DocumentCommentsWebSocket)
	mux.HandleFunc("GET /wsc/comments/{commentId}", h.Get)
	mux.HandleFunc("PATCH /wsc/comments/{commentId}", h.Update)
	mux.HandleFunc("DELETE /wsc/comments/{commentId}", h.Delete)
	mux.HandleFunc("POST /wsc/comments/{commentId}/status", h.ChangeStatus)
	mux.HandleFunc("PUT /wsc/comments/{commentId}/assignees", h.ReplaceAssignees)
	mux.HandleFunc("GET /wsc/comments/{commentId}/status-history", h.StatusHistory)
	mux.HandleFunc("GET /wsc/comments/{commentId}/edit-history", h.EditHistory)
}

func (h *CommentsHandler) List(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceId")
	if !validateUUIDParam(w, "workspaceId", workspaceID) {
		return
	}

	query := r.URL.Query()
	filter := model.CommentListFilter{
		WorkspaceID:    workspaceID,
		SketchID:       strings.TrimSpace(query.Get("sketchId")),
		PartID:         strings.TrimSpace(query.Get("partId")),
		TargetType:     strings.TrimSpace(query.Get("targetType")),
		TargetID:       strings.TrimSpace(query.Get("targetId")),
		Kind:           strings.TrimSpace(query.Get("kind")),
		Status:         strings.TrimSpace(query.Get("status")),
		AssigneeUserID: strings.TrimSpace(query.Get("assigneeUserId")),
		IncludeDeleted: query.Get("includeDeleted") == "true",
		Limit:          parseIntDefault(query.Get("limit"), 50),
		Offset:         parseIntDefault(query.Get("offset"), 0),
	}
	if filter.SketchID != "" && !isValidUUID(filter.SketchID) {
		writeError(w, http.StatusBadRequest, "INVALID_OPERATION", "sketchId must be a valid uuid")
		return
	}
	if filter.PartID != "" && !isValidUUID(filter.PartID) {
		writeError(w, http.StatusBadRequest, "INVALID_OPERATION", "partId must be a valid uuid")
		return
	}

	response, err := h.service.List(r.Context(), filter)
	if err != nil {
		writeError(w, statusFromError(err), codeFromStatus(statusFromError(err)), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, response)
}

func (h *CommentsHandler) Create(w http.ResponseWriter, r *http.Request) {
	workspaceID := r.PathValue("workspaceId")
	if !validateUUIDParam(w, "workspaceId", workspaceID) {
		return
	}

	var request model.CreateCommentRequest
	if err := readJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_OPERATION", err.Error())
		return
	}
	if request.PartID != nil && !isValidUUID(*request.PartID) {
		writeError(w, http.StatusBadRequest, "INVALID_OPERATION", "partId must be a valid uuid")
		return
	}
	if request.SketchID != nil && !isValidUUID(*request.SketchID) {
		writeError(w, http.StatusBadRequest, "INVALID_OPERATION", "sketchId must be a valid uuid")
		return
	}

	comment, err := h.service.Create(r.Context(), workspaceID, &request)
	if err != nil {
		writeError(w, statusFromError(err), codeFromStatus(statusFromError(err)), err.Error())
		return
	}
	writeJSON(w, http.StatusCreated, comment)
}

func (h *CommentsHandler) Get(w http.ResponseWriter, r *http.Request) {
	commentID := r.PathValue("commentId")
	if !validateUUIDParam(w, "commentId", commentID) {
		return
	}
	comment, err := h.service.Get(r.Context(), commentID)
	if err != nil {
		writeError(w, statusFromError(err), codeFromStatus(statusFromError(err)), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, comment)
}

func (h *CommentsHandler) Update(w http.ResponseWriter, r *http.Request) {
	commentID := r.PathValue("commentId")
	if !validateUUIDParam(w, "commentId", commentID) {
		return
	}
	var request model.UpdateCommentRequest
	if err := readJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_OPERATION", err.Error())
		return
	}
	comment, err := h.service.Update(r.Context(), commentID, &request)
	if err != nil {
		writeError(w, statusFromError(err), codeFromStatus(statusFromError(err)), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, comment)
}

func (h *CommentsHandler) Delete(w http.ResponseWriter, r *http.Request) {
	commentID := r.PathValue("commentId")
	if !validateUUIDParam(w, "commentId", commentID) {
		return
	}
	if err := h.service.Delete(r.Context(), commentID); err != nil {
		writeError(w, statusFromError(err), codeFromStatus(statusFromError(err)), err.Error())
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (h *CommentsHandler) ChangeStatus(w http.ResponseWriter, r *http.Request) {
	commentID := r.PathValue("commentId")
	if !validateUUIDParam(w, "commentId", commentID) {
		return
	}
	var request model.ChangeCommentStatusRequest
	if err := readJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_OPERATION", err.Error())
		return
	}
	comment, err := h.service.ChangeStatus(r.Context(), commentID, &request)
	if err != nil {
		writeError(w, statusFromError(err), codeFromStatus(statusFromError(err)), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, comment)
}

func (h *CommentsHandler) ReplaceAssignees(w http.ResponseWriter, r *http.Request) {
	commentID := r.PathValue("commentId")
	if !validateUUIDParam(w, "commentId", commentID) {
		return
	}
	var request model.ReplaceCommentAssigneesRequest
	if err := readJSON(r, &request); err != nil {
		writeError(w, http.StatusBadRequest, "INVALID_OPERATION", err.Error())
		return
	}
	comment, err := h.service.ReplaceAssignees(r.Context(), commentID, &request)
	if err != nil {
		writeError(w, statusFromError(err), codeFromStatus(statusFromError(err)), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, comment)
}

func (h *CommentsHandler) StatusHistory(w http.ResponseWriter, r *http.Request) {
	commentID := r.PathValue("commentId")
	if !validateUUIDParam(w, "commentId", commentID) {
		return
	}
	items, err := h.service.StatusHistory(r.Context(), commentID)
	if err != nil {
		writeError(w, statusFromError(err), codeFromStatus(statusFromError(err)), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, &model.CommentStatusHistoryResponse{Items: items})
}

func (h *CommentsHandler) EditHistory(w http.ResponseWriter, r *http.Request) {
	commentID := r.PathValue("commentId")
	if !validateUUIDParam(w, "commentId", commentID) {
		return
	}
	items, err := h.service.EditHistory(r.Context(), commentID)
	if err != nil {
		writeError(w, statusFromError(err), codeFromStatus(statusFromError(err)), err.Error())
		return
	}
	writeJSON(w, http.StatusOK, &model.CommentEditHistoryResponse{Items: items})
}

func validateUUIDParam(w http.ResponseWriter, name string, value string) bool {
	if strings.TrimSpace(value) == "" {
		writeError(w, http.StatusBadRequest, "INVALID_OPERATION", name+" is required")
		return false
	}
	if !isValidUUID(value) {
		writeError(w, http.StatusBadRequest, "INVALID_OPERATION", name+" must be a valid uuid")
		return false
	}
	return true
}

func parseIntDefault(value string, fallback int) int {
	if strings.TrimSpace(value) == "" {
		return fallback
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return fallback
	}
	return parsed
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

func statusFromError(err error) int {
	if err == nil {
		return http.StatusOK
	}
	message := err.Error()
	switch {
	case strings.Contains(message, "authenticated user id"):
		return http.StatusUnauthorized
	case strings.Contains(message, "returned no rows"):
		return http.StatusNotFound
	case strings.Contains(message, "required"),
		strings.Contains(message, "invalid"),
		strings.Contains(message, "must"):
		return http.StatusBadRequest
	default:
		return http.StatusInternalServerError
	}
}

func codeFromStatus(status int) string {
	switch status {
	case http.StatusUnauthorized:
		return "AUTH_REQUIRED"
	case http.StatusNotFound:
		return "SKETCH_NOT_FOUND"
	case http.StatusBadRequest:
		return "INVALID_OPERATION"
	default:
		return "INTERNAL_ERROR"
	}
}

func isValidUUID(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != uuidLength {
		return false
	}

	for i, r := range value {
		switch i {
		case uuidDash1, uuidDash2, uuidDash3, uuidDash4:
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
