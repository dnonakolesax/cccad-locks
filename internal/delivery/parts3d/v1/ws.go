package v1

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"github.com/dnonakolesax/cccad-locks/internal/auth"
	"github.com/dnonakolesax/cccad-locks/internal/model"
	geometryv1 "github.com/dnonakolesax/cccad-locks/internal/proto/geometry/v1"
	"github.com/gorilla/websocket"
)

const (
	part3DWriteWait       = 10 * time.Second
	part3DPongWait        = 60 * time.Second
	part3DPingPeriod      = 25 * time.Second
	part3DMaxMessageBytes = 1 << 20
	part3DWSBufferSize    = 4096
	part3DUUIDLength      = 36
	part3DUUIDDash1       = 8
	part3DUUIDDash2       = 13
	part3DUUIDDash3       = 18
	part3DUUIDDash4       = 23
)

const (
	MsgPart3DFeatureIntent     = "part.3d.feature.intent"
	MsgPart3DFeatureAccepted   = "part.3d.feature.accepted"
	MsgPart3DFeatureRejected   = "part.3d.feature.rejected"
	MsgPart3DFeatureCommitted  = "part.3d.feature.committed"
	MsgPart3DFeatureSuppressed = "part.3d.feature.suppressed"
	MsgPart3DFeatureDeleted    = "part.3d.feature.deleted"
	MsgPart3DRebuildStarted    = "part.3d.rebuild.started"
	MsgPart3DRebuildCompleted  = "part.3d.rebuild.completed"
	MsgPart3DRebuildFailed     = "part.3d.rebuild.failed"
	MsgPart3DTopologyUpdated   = "part.3d.topology.updated"
	MsgPart3DSelectionChanged  = "part.3d.selection.changed"
	MsgPart3DPreviewChanged    = "part.3d.preview.changed"
)

var knownPart3DMessageTypes = map[string]struct{}{
	MsgPart3DFeatureIntent:     {},
	MsgPart3DFeatureAccepted:   {},
	MsgPart3DFeatureRejected:   {},
	MsgPart3DFeatureCommitted:  {},
	MsgPart3DFeatureSuppressed: {},
	MsgPart3DFeatureDeleted:    {},
	MsgPart3DRebuildStarted:    {},
	MsgPart3DRebuildCompleted:  {},
	MsgPart3DRebuildFailed:     {},
	MsgPart3DTopologyUpdated:   {},
	MsgPart3DSelectionChanged:  {},
	MsgPart3DPreviewChanged:    {},
}

type Parts3DWSHandler struct {
	logger   *slog.Logger
	upgrader websocket.Upgrader
	geometry part3DGeometryClient
	repo     part3DFeatureRepository

	mu          sync.RWMutex
	nextConnID  int64
	connections map[int64]*part3DWSConnection
}

type part3DGeometryClient interface {
	BuildExtrude(context.Context, *geometryv1.BuildExtrudeRequest) (*geometryv1.BuildFeatureResponse, error)
	BuildHole(context.Context, *geometryv1.BuildHoleRequest) (*geometryv1.BuildFeatureResponse, error)
	BuildFillet(context.Context, *geometryv1.BuildFilletRequest) (*geometryv1.BuildFeatureResponse, error)
	BuildChamfer(context.Context, *geometryv1.BuildChamferRequest) (*geometryv1.BuildFeatureResponse, error)
	BuildPattern(context.Context, *geometryv1.BuildPatternRequest) (*geometryv1.BuildFeatureResponse, error)
	BuildBoolean(context.Context, *geometryv1.BuildBooleanRequest) (*geometryv1.BuildFeatureResponse, error)
}

type part3DFeatureRepository interface {
	CommitFeatureBuild(context.Context, model.Feature3DCommit) (*model.Feature3DCommitResult, error)
}

type Parts3DWSHandlerOption func(*Parts3DWSHandler)

func WithParts3DWSLogger(logger *slog.Logger) Parts3DWSHandlerOption {
	return func(h *Parts3DWSHandler) {
		if logger != nil {
			h.logger = logger
		}
	}
}

func WithParts3DFeatureProcessor(
	geometry part3DGeometryClient,
	repo part3DFeatureRepository,
) Parts3DWSHandlerOption {
	return func(h *Parts3DWSHandler) {
		h.geometry = geometry
		h.repo = repo
	}
}

func NewParts3DWSHandler(opts ...Parts3DWSHandlerOption) *Parts3DWSHandler {
	h := &Parts3DWSHandler{
		logger: slog.Default(),
		upgrader: websocket.Upgrader{
			ReadBufferSize:  part3DWSBufferSize,
			WriteBufferSize: part3DWSBufferSize,
			CheckOrigin: func(_ *http.Request) bool {
				return true
			},
		},
		connections: map[int64]*part3DWSConnection{},
	}
	for _, opt := range opts {
		opt(h)
	}
	return h
}

func (h *Parts3DWSHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /parts/{partId}/ws", h.HandleWebSocket)
}

func (h *Parts3DWSHandler) HandleWebSocket(w http.ResponseWriter, r *http.Request) {
	partID := strings.TrimSpace(r.PathValue("partId"))
	if !isValidUUID(partID) {
		writeJSONError(w, http.StatusBadRequest, "INVALID_OPERATION", "partId must be a valid uuid")
		return
	}

	userID, ok := auth.UserIDFromContext(r.Context())
	if !ok {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	ws, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.WarnContext(r.Context(), "3d websocket upgrade failed", slog.String("error", err.Error()))
		return
	}

	conn := h.addConnection(partID, userID, ws)
	defer h.removeConnection(conn.id)
	defer func() { _ = ws.Close() }()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	errc := make(chan error, 2)
	go h.writeLoop(ctx, conn, errc)
	go h.readLoop(ctx, conn, errc)

	if loopErr := <-errc; loopErr != nil && !isNormalPart3DWSError(loopErr) {
		h.logger.WarnContext(
			r.Context(),
			"3d websocket closed",
			slog.Int64("connectionId", conn.id),
			slog.String("partId", partID),
			slog.String("error", loopErr.Error()),
		)
	}
	cancel()
}

func (h *Parts3DWSHandler) addConnection(partID, userID string, ws *websocket.Conn) *part3DWSConnection {
	h.mu.Lock()
	defer h.mu.Unlock()

	h.nextConnID++
	conn := &part3DWSConnection{
		id:       h.nextConnID,
		partID:   partID,
		userID:   userID,
		ws:       ws,
		outbound: make(chan []byte, 32),
	}
	h.connections[conn.id] = conn
	return conn
}

func (h *Parts3DWSHandler) removeConnection(connID int64) {
	h.mu.Lock()
	conn := h.connections[connID]
	delete(h.connections, connID)
	h.mu.Unlock()

	if conn != nil {
		close(conn.outbound)
	}
}

func (h *Parts3DWSHandler) readLoop(ctx context.Context, conn *part3DWSConnection, errc chan<- error) {
	defer func() { errc <- nil }()

	conn.ws.SetReadLimit(part3DMaxMessageBytes)
	_ = conn.ws.SetReadDeadline(time.Now().Add(part3DPongWait))
	conn.ws.SetPongHandler(func(string) error {
		return conn.ws.SetReadDeadline(time.Now().Add(part3DPongWait))
	})

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		messageType, data, err := conn.ws.ReadMessage()
		if err != nil {
			errc <- err
			return
		}
		if messageType != websocket.TextMessage && messageType != websocket.BinaryMessage {
			continue
		}

		msg, err := validatePart3DMessage(conn.partID, data)
		if err != nil {
			errc <- err
			return
		}

		if msg.Type == MsgPart3DFeatureIntent {
			if err := h.handleFeatureIntent(ctx, conn, data); err != nil {
				h.logger.WarnContext(ctx,
					"3d feature intent failed",
					slog.Int64("connectionId", conn.id),
					slog.String("partId", conn.partID),
					slog.String("error", err.Error()))
			}
			continue
		}
		if strings.HasPrefix(msg.Type, "part.3d.feature.") {
			_ = h.writeMessage(
				conn.ws,
				websocket.TextMessage,
				part3DErrorMessage(conn.partID, "UNSUPPORTED_MESSAGE", "feature lifecycle messages are server-owned"),
			)
			continue
		}

		h.broadcast(conn.partID, msg.raw)
	}
}

func (h *Parts3DWSHandler) writeLoop(ctx context.Context, conn *part3DWSConnection, errc chan<- error) {
	ticker := time.NewTicker(part3DPingPeriod)
	defer ticker.Stop()
	defer func() { errc <- nil }()

	for {
		select {
		case <-ctx.Done():
			return
		case data, ok := <-conn.outbound:
			if !ok {
				_ = h.writeControl(conn.ws, websocket.CloseMessage, []byte{})
				return
			}
			if err := h.writeMessage(conn.ws, websocket.TextMessage, data); err != nil {
				errc <- err
				return
			}
		case <-ticker.C:
			if err := h.writeControl(conn.ws, websocket.PingMessage, []byte("ping")); err != nil {
				errc <- err
				return
			}
		}
	}
}

func (h *Parts3DWSHandler) broadcast(partID string, data []byte) {
	h.mu.RLock()
	defer h.mu.RUnlock()

	for _, conn := range h.connections {
		if conn.partID != partID {
			continue
		}
		select {
		case conn.outbound <- data:
		default:
			h.logger.Warn("3d websocket outbound queue full",
				slog.Int64("connectionId", conn.id),
				slog.String("partId", conn.partID))
		}
	}
}

func (h *Parts3DWSHandler) writeMessage(ws *websocket.Conn, messageType int, data []byte) error {
	_ = ws.SetWriteDeadline(time.Now().Add(part3DWriteWait))
	return ws.WriteMessage(messageType, data)
}

func (h *Parts3DWSHandler) writeControl(ws *websocket.Conn, messageType int, data []byte) error {
	return ws.WriteControl(messageType, data, time.Now().Add(part3DWriteWait))
}

type part3DWSConnection struct {
	id       int64
	partID   string
	userID   string
	ws       *websocket.Conn
	outbound chan []byte
}

type part3DMessageEnvelope struct {
	Type   string          `json:"type"`
	PartID string          `json:"partId"`
	raw    json.RawMessage `json:"-"`
}

func validatePart3DMessage(pathPartID string, data []byte) (*part3DMessageEnvelope, error) {
	var msg part3DMessageEnvelope
	if err := json.Unmarshal(data, &msg); err != nil {
		return nil, err
	}
	msg.Type = strings.TrimSpace(msg.Type)
	msg.PartID = strings.TrimSpace(msg.PartID)
	if _, ok := knownPart3DMessageTypes[msg.Type]; !ok {
		return nil, errors.New("unsupported 3d websocket message type")
	}
	if msg.PartID != pathPartID {
		return nil, errors.New("message partId does not match websocket partId")
	}
	msg.raw = append(msg.raw[:0], data...)
	return &msg, nil
}

func isNormalPart3DWSError(err error) bool {
	return websocket.IsCloseError(
		err,
		websocket.CloseNormalClosure,
		websocket.CloseGoingAway,
		websocket.CloseNoStatusReceived,
	)
}

func isValidUUID(value string) bool {
	value = strings.TrimSpace(value)
	if len(value) != part3DUUIDLength {
		return false
	}
	for i, r := range value {
		switch i {
		case part3DUUIDDash1, part3DUUIDDash2, part3DUUIDDash3, part3DUUIDDash4:
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
	return ('0' <= r && r <= '9') || ('a' <= r && r <= 'f') || ('A' <= r && r <= 'F')
}
