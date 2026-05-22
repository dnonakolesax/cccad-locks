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
			ReadBufferSize:  4096,
			WriteBufferSize: 4096,
			CheckOrigin: func(r *http.Request) bool {
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
// It expects paths like: /api/v1/sketches/{sketchId}/ws
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
		h.logger.Warn("websocket upgrade failed", "error", err)
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
		h.logger.Warn("realtime connection rejected", "sketchId", sketchID, "userId", identity.UserID, "error", err)
		return
	}

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()
	defer func() {
		_ = conn.Close(context.Background(), "transport_closed")
		_ = ws.Close()
	}()

	errc := make(chan error, 2)
	go h.writeLoop(ctx, ws, conn, errc)
	go h.readLoop(ctx, ws, conn, errc)

	if err := <-errc; err != nil && !isNormalWSError(err) {
		h.logger.Warn("realtime websocket closed", "connectionId", conn.ID(), "sketchId", sketchID, "error", err)
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

		messageType, data, err := ws.ReadMessage()
		if err != nil {
			errc <- err
			return
		}
		if messageType != websocket.TextMessage && messageType != websocket.BinaryMessage {
			continue
		}

		var msg model.ClientRealtimeMessage
		if err := easyjson.Unmarshal(data, &msg); err != nil {
			h.logger.Warn("invalid realtime message", "connectionId", conn.ID(), "error", err)
			continue
		}

		if msg.SketchID == "" {
			msg.SketchID = conn.SketchID()
		}

		if err := conn.HandleClientMessage(ctx, msg); err != nil {
			h.logger.Warn(
				"realtime message handling failed",
				"connectionId", conn.ID(),
				"type", msg.Type,
				"requestId", msg.RequestID,
				"error", err,
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

			data, err := easyjson.Marshal(&msg)
			if err != nil {
				h.logger.Warn("failed to encode realtime message", "connectionId", conn.ID(), "type", msg.Type, "error", err)
				continue
			}

			if err := h.writeMessage(ws, websocket.TextMessage, data); err != nil {
				errc <- err
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

	bearer := bearerToken(r.Header.Get("Authorization"))
	if bearer == "" {
		// Browser WebSocket clients often cannot set Authorization header. Allow token query
		// parameter for development; prefer a secure cookie or subprotocol token in production.
		bearer = r.URL.Query().Get("access_token")
	}
	if bearer == "" {
		return UserIdentity{}, errors.New("missing bearer token")
	}
	if h.userResolver == nil {
		return UserIdentity{}, errors.New("user resolver is required")
	}
	return h.userResolver.Resolve(r.Context(), bearer)
}

func bearerToken(value string) string {
	const prefix = "Bearer "
	if !strings.HasPrefix(value, prefix) {
		return ""
	}
	return strings.TrimSpace(strings.TrimPrefix(value, prefix))
}

func isNormalWSError(err error) bool {
	return websocket.IsCloseError(
		err,
		websocket.CloseNormalClosure,
		websocket.CloseGoingAway,
		websocket.CloseNoStatusReceived,
	)
}
