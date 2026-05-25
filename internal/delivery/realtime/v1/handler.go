package v1

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/dnonakolesax/cccad-locks/internal/auth"
	"github.com/dnonakolesax/cccad-locks/internal/model"
	"github.com/gorilla/websocket"
	"github.com/mailru/easyjson"
)

const (
	defaultRoutePrefix     = "/api/v1/sketches/"
	defaultWSRouteSuffix   = "/ws"
	defaultWriteWait       = 10 * time.Second
	defaultPongWait        = 60 * time.Second
	defaultPingPeriod      = 25 * time.Second
	defaultMaxMessageBytes = 1 << 20 // 1 MiB
	defaultWSBufferSize    = 4096
	websocketLoopErrors    = 2
)

type Handler struct {
	service      Service
	userResolver UserResolver
	logger       *slog.Logger
	routePrefix  string

	upgrader websocket.Upgrader

	writeWait       time.Duration
	pongWait        time.Duration
	pingPeriod      time.Duration
	maxMessageBytes int64
}

type HandlerOption func(*Handler)

func WithLogger(logger *slog.Logger) HandlerOption {
	return func(h *Handler) {
		if logger != nil {
			h.logger = logger
		}
	}
}

func WithCheckOrigin(fn func(r *http.Request) bool) HandlerOption {
	return func(h *Handler) {
		h.upgrader.CheckOrigin = fn
	}
}

func WithRoutePrefix(prefix string) HandlerOption {
	return func(h *Handler) {
		if prefix != "" {
			h.routePrefix = prefix
		}
	}
}

func NewHandler(service Service, userResolver UserResolver, opts ...HandlerOption) *Handler {
	h := &Handler{
		service:      service,
		userResolver: userResolver,
		logger:       slog.Default(),
		routePrefix:  defaultRoutePrefix,
		upgrader: websocket.Upgrader{
			ReadBufferSize:  defaultWSBufferSize,
			WriteBufferSize: defaultWSBufferSize,
			CheckOrigin: func(_ *http.Request) bool {
				// In production, replace through WithCheckOrigin and check allowed frontend origins.
				return true
			},
		},
		writeWait:       defaultWriteWait,
		pongWait:        defaultPongWait,
		pingPeriod:      defaultPingPeriod,
		maxMessageBytes: defaultMaxMessageBytes,
	}

	for _, opt := range opts {
		opt(h)
	}

	return h
}

// RegisterRoutes registers the realtime websocket endpoint on the provided mux.
// It expects paths like: /api/v1/sketches/{sketchId}/ws.
func (h *Handler) RegisterRoutes(mux *http.ServeMux) {
	if h.routePrefix == "/" {
		mux.HandleFunc("GET /{sketchId}/ws", h.handleSketchWebSocket)
		return
	}
	mux.HandleFunc(h.routePrefix, h.handleSketchWebSocket)
}

func (h *Handler) handleSketchWebSocket(w http.ResponseWriter, r *http.Request) {
	sketchID, ok := h.extractSketchID(r.URL.Path)
	if !ok {
		http.NotFound(w, r)
		return
	}

	identity, err := h.resolveUser(r)
	if err != nil {
		http.Error(w, http.StatusText(http.StatusUnauthorized), http.StatusUnauthorized)
		return
	}

	ws, err := h.upgrader.Upgrade(w, r, nil)
	if err != nil {
		h.logger.WarnContext(r.Context(), "websocket upgrade failed", "error", err)
		return
	}

	openReq := model.OpenRealtimeSessionRequest{
		SketchID:    sketchID,
		UserID:      identity.UserID,
		UserName:    identity.DisplayName,
		ClientID:    r.URL.Query().Get("clientId"),
		RemoteAddr:  r.RemoteAddr,
		UserAgent:   r.UserAgent(),
		AccessToken: identity.AccessToken,
	}

	conn, err := h.service.OpenConnection(r.Context(), openReq)
	if err != nil {
		_ = ws.WriteControl(
			websocket.CloseMessage,
			websocket.FormatCloseMessage(websocket.ClosePolicyViolation, "connection rejected"),
			time.Now().Add(h.writeWait),
		)
		_ = ws.Close()
		h.logger.WarnContext(
			r.Context(),
			"realtime connection rejected",
			"sketchId",
			sketchID,
			"userId",
			identity.UserID,
			"error",
			err,
		)
		return
	}

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	defer func() {
		_ = conn.Close(context.Background(), "transport_closed")
		_ = ws.Close()
	}()

	errc := make(chan error, websocketLoopErrors)
	go h.writeLoop(ctx, ws, conn, errc)
	go h.readLoop(ctx, ws, conn, errc)

	loopErr := <-errc
	if loopErr != nil && !isNormalWSError(loopErr) {
		h.logger.WarnContext(
			r.Context(),
			"realtime websocket closed",
			"connectionId",
			conn.ID(),
			"sketchId",
			sketchID,
			"error",
			loopErr,
		)
	}
	cancel()
}

func (h *Handler) readLoop(ctx context.Context, ws *websocket.Conn, conn Connection, errc chan<- error) {
	defer func() { errc <- nil }()

	ws.SetReadLimit(h.maxMessageBytes)
	_ = ws.SetReadDeadline(time.Now().Add(h.pongWait))
	ws.SetPongHandler(func(string) error {
		return ws.SetReadDeadline(time.Now().Add(h.pongWait))
	})

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		messageType, data, readErr := ws.ReadMessage()
		if readErr != nil {
			errc <- readErr
			return
		}
		if messageType != websocket.TextMessage && messageType != websocket.BinaryMessage {
			continue
		}

		var msg model.ClientRealtimeMessage
		if decodeErr := easyjson.Unmarshal(data, &msg); decodeErr != nil {
			h.logger.WarnContext(ctx, "invalid realtime message", "connectionId", conn.ID(), "error", decodeErr)
			continue
		}

		if msg.SketchID == "" {
			msg.SketchID = conn.SketchID()
		}

		if handleErr := conn.HandleClientMessage(ctx, msg); handleErr != nil {
			h.logger.WarnContext(
				ctx,
				"realtime message handling failed",
				"connectionId", conn.ID(),
				"type", msg.Type,
				"requestId", msg.RequestID,
				"error", handleErr,
			)
		}
	}
}

func (h *Handler) writeLoop(ctx context.Context, ws *websocket.Conn, conn Connection, errc chan<- error) {
	ticker := time.NewTicker(h.pingPeriod)
	defer ticker.Stop()
	defer func() { errc <- nil }()

	for {
		select {
		case <-ctx.Done():
			return

		case msg, ok := <-conn.Outbound():
			if !ok {
				_ = ws.WriteControl(
					websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.CloseNormalClosure, "session closed"),
					time.Now().Add(h.writeWait),
				)
				return
			}

			data, encodeErr := easyjson.Marshal(&msg)
			if encodeErr != nil {
				h.logger.WarnContext(
					ctx,
					"failed to encode realtime message",
					"connectionId",
					conn.ID(),
					"type",
					msg.Type,
					"error",
					encodeErr,
				)
				continue
			}

			if writeErr := h.writeMessage(ws, websocket.TextMessage, data); writeErr != nil {
				errc <- writeErr
				return
			}

		case <-ticker.C:
			if err := h.writeControl(ws, websocket.PingMessage, []byte("ping")); err != nil {
				errc <- err
				return
			}
		}
	}
}

func (h *Handler) writeMessage(ws *websocket.Conn, messageType int, data []byte) error {
	_ = ws.SetWriteDeadline(time.Now().Add(h.writeWait))
	return ws.WriteMessage(messageType, data)
}

func (h *Handler) writeControl(ws *websocket.Conn, messageType int, data []byte) error {
	return ws.WriteControl(messageType, data, time.Now().Add(h.writeWait))
}

func (h *Handler) extractSketchID(path string) (string, bool) {
	if !strings.HasPrefix(path, h.routePrefix) || !strings.HasSuffix(path, defaultWSRouteSuffix) {
		return "", false
	}

	middle := strings.TrimSuffix(strings.TrimPrefix(path, h.routePrefix), defaultWSRouteSuffix)
	middle = strings.Trim(middle, "/")
	if middle == "" || strings.Contains(middle, "/") {
		return "", false
	}

	return middle, true
}

func (h *Handler) resolveUser(r *http.Request) (UserIdentity, error) {
	if userID, ok := auth.UserIDFromContext(r.Context()); ok {
		return UserIdentity{UserID: userID}, nil
	}

	return UserIdentity{}, errors.New("authenticated user id is missing from request context")
}

func isNormalWSError(err error) bool {
	return websocket.IsCloseError(
		err,
		websocket.CloseNormalClosure,
		websocket.CloseGoingAway,
		websocket.CloseNoStatusReceived,
	)
}
