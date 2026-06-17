package v1

import (
	"context"
	"log/slog"
	"net/http"
	"time"

	"github.com/dnonakolesax/cccad-locks/internal/model"
	"github.com/gorilla/websocket"
	"github.com/mailru/easyjson"
)

const (
	commentWSWriteWait       = 10 * time.Second
	commentWSPongWait        = 60 * time.Second
	commentWSPingPeriod      = 25 * time.Second
	commentWSMaxMessageBytes = 1 << 20
	commentWSBufferSize      = 4096
)

var commentWSUpgrader = websocket.Upgrader{
	ReadBufferSize:  commentWSBufferSize,
	WriteBufferSize: commentWSBufferSize,
	CheckOrigin: func(_ *http.Request) bool {
		return true
	},
}

func (h *CommentsHandler) DocumentCommentsWebSocket(w http.ResponseWriter, r *http.Request) {
	documentID := r.PathValue("documentId")
	if !validateUUIDParam(w, "documentId", documentID) {
		return
	}

	subscription, err := h.service.SubscribeDocument(r.Context(), documentID)
	if err != nil {
		http.Error(w, http.StatusText(statusFromError(err)), statusFromError(err))
		return
	}
	defer subscription.Close()

	ws, err := commentWSUpgrader.Upgrade(w, r, nil)
	if err != nil {
		slog.Default().WarnContext(r.Context(), "comments websocket upgrade failed", "error", err)
		return
	}
	defer ws.Close()

	ctx, cancel := context.WithCancel(r.Context())
	defer cancel()

	errc := make(chan error, 2)
	go commentsWSReadLoop(ctx, ws, errc)
	go commentsWSWriteLoop(ctx, ws, subscription, errc)

	if err := <-errc; err != nil && !websocket.IsCloseError(
		err,
		websocket.CloseNormalClosure,
		websocket.CloseGoingAway,
		websocket.CloseNoStatusReceived,
	) {
		slog.Default().WarnContext(
			r.Context(),
			"comments websocket closed",
			"subscriptionId", subscription.ID(),
			"documentId", subscription.DocumentID(),
			"error", err,
		)
	}
	cancel()
}

func commentsWSReadLoop(ctx context.Context, ws *websocket.Conn, errc chan<- error) {
	defer func() { errc <- nil }()

	ws.SetReadLimit(commentWSMaxMessageBytes)
	_ = ws.SetReadDeadline(time.Now().Add(commentWSPongWait))
	ws.SetPongHandler(func(string) error {
		return ws.SetReadDeadline(time.Now().Add(commentWSPongWait))
	})

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		messageType, _, err := ws.ReadMessage()
		if err != nil {
			errc <- err
			return
		}
		if messageType == websocket.CloseMessage {
			return
		}
	}
}

func commentsWSWriteLoop(ctx context.Context, ws *websocket.Conn, subscription model.CommentSubscription, errc chan<- error) {
	ticker := time.NewTicker(commentWSPingPeriod)
	defer ticker.Stop()
	defer func() { errc <- nil }()

	for {
		select {
		case <-ctx.Done():
			return
		case event, ok := <-subscription.Events():
			if !ok {
				_ = ws.WriteControl(
					websocket.CloseMessage,
					websocket.FormatCloseMessage(websocket.CloseNormalClosure, "subscription closed"),
					time.Now().Add(commentWSWriteWait),
				)
				return
			}
			body, err := easyjson.Marshal(&event)
			if err != nil {
				slog.Default().WarnContext(ctx, "failed to encode comment realtime event", "error", err)
				continue
			}
			if err := commentsWSWriteMessage(ws, websocket.TextMessage, body); err != nil {
				errc <- err
				return
			}
		case <-ticker.C:
			if err := ws.WriteControl(websocket.PingMessage, []byte("ping"), time.Now().Add(commentWSWriteWait)); err != nil {
				errc <- err
				return
			}
		}
	}
}

func commentsWSWriteMessage(ws *websocket.Conn, messageType int, body []byte) error {
	_ = ws.SetWriteDeadline(time.Now().Add(commentWSWriteWait))
	return ws.WriteMessage(messageType, body)
}
