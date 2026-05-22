package realtime

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/dnonakolesax/cccad-locks/internal/model"
)

type permissionsStub struct {
	permissions []model.Permission
}

func (p permissionsStub) List(context.Context, string) ([]model.Permission, error) {
	return p.permissions, nil
}

type sketchesStub struct {
	document *model.SketchDocument
}

func (s sketchesStub) Get(context.Context, string) (*model.SketchDocument, error) {
	return s.document, nil
}

func TestSessionJoinRepliesWithActiveUsersAndBroadcastsJoin(t *testing.T) {
	service := testService()
	ctx := context.Background()

	first, err := service.OpenConnection(ctx, model.OpenRealtimeSessionRequest{
		SketchID: "sketch-id",
		UserID:   "user-1",
		UserName: "User 1",
	})
	if err != nil {
		t.Fatalf("OpenConnection first returned error: %v", err)
	}
	if err := first.HandleClientMessage(ctx, joinMessage("req-1", "client-1", 6)); err != nil {
		t.Fatalf("first session.join returned error: %v", err)
	}
	firstJoined := nextMessage(t, first)
	if firstJoined.Type != msgSessionJoined {
		t.Fatalf("first message type = %q, want %q", firstJoined.Type, msgSessionJoined)
	}

	second, err := service.OpenConnection(ctx, model.OpenRealtimeSessionRequest{
		SketchID: "sketch-id",
		UserID:   "user-2",
		UserName: "User 2",
	})
	if err != nil {
		t.Fatalf("OpenConnection second returned error: %v", err)
	}
	if err := second.HandleClientMessage(ctx, joinMessage("req-2", "client-2", 7)); err != nil {
		t.Fatalf("second session.join returned error: %v", err)
	}

	secondJoined := nextMessage(t, second)
	if secondJoined.Type != msgSessionJoined {
		t.Fatalf("second message type = %q, want %q", secondJoined.Type, msgSessionJoined)
	}
	var joinedPayload model.SessionJoinedPayload
	decodePayload(t, secondJoined, &joinedPayload)
	if len(joinedPayload.ActiveUsers) != 1 || joinedPayload.ActiveUsers[0].UserID != "user-1" {
		t.Fatalf("activeUsers = %#v, want user-1", joinedPayload.ActiveUsers)
	}
	if joinedPayload.MissingOpsAvailable {
		t.Fatal("missingOpsAvailable = true, want false")
	}

	userJoined := nextMessage(t, first)
	if userJoined.Type != msgSessionUserJoined {
		t.Fatalf("broadcast type = %q, want %q", userJoined.Type, msgSessionUserJoined)
	}
	var joinedUser model.UserPresenceSummary
	decodePayload(t, userJoined, &joinedUser)
	if joinedUser.UserID != "user-2" || joinedUser.ClientID != "client-2" {
		t.Fatalf("joined user = %#v, want user-2/client-2", joinedUser)
	}
}

func TestSessionPingRepliesWithPong(t *testing.T) {
	service := testService()
	conn, err := service.OpenConnection(context.Background(), model.OpenRealtimeSessionRequest{
		SketchID: "sketch-id",
		UserID:   "user-1",
	})
	if err != nil {
		t.Fatalf("OpenConnection returned error: %v", err)
	}

	if err := conn.HandleClientMessage(context.Background(), model.ClientRealtimeMessage{
		Type:      msgSessionPing,
		RequestID: "req-ping",
		SketchID:  "sketch-id",
		Payload:   json.RawMessage(`{"clientVersion":6}`),
	}); err != nil {
		t.Fatalf("session.ping returned error: %v", err)
	}

	msg := nextMessage(t, conn)
	if msg.Type != msgSessionPong || msg.RequestID != "req-ping" {
		t.Fatalf("message = %#v, want session.pong req-ping", msg)
	}
	var payload model.SessionPongPayload
	decodePayload(t, msg, &payload)
	if payload.CurrentVersion != 7 {
		t.Fatalf("currentVersion = %d, want 7", payload.CurrentVersion)
	}
}

func TestCloseBroadcastsUserLeft(t *testing.T) {
	service := testService()
	ctx := context.Background()

	first, err := service.OpenConnection(ctx, model.OpenRealtimeSessionRequest{SketchID: "sketch-id", UserID: "user-1"})
	if err != nil {
		t.Fatalf("OpenConnection first returned error: %v", err)
	}
	if err := first.HandleClientMessage(ctx, joinMessage("req-1", "client-1", 7)); err != nil {
		t.Fatalf("first join returned error: %v", err)
	}
	_ = nextMessage(t, first)

	second, err := service.OpenConnection(ctx, model.OpenRealtimeSessionRequest{SketchID: "sketch-id", UserID: "user-2"})
	if err != nil {
		t.Fatalf("OpenConnection second returned error: %v", err)
	}
	if err := second.HandleClientMessage(ctx, joinMessage("req-2", "client-2", 7)); err != nil {
		t.Fatalf("second join returned error: %v", err)
	}
	_ = nextMessage(t, second)
	_ = nextMessage(t, first)

	if err := second.Close(ctx, "transport_closed"); err != nil {
		t.Fatalf("Close returned error: %v", err)
	}

	left := nextMessage(t, first)
	if left.Type != msgSessionUserLeft {
		t.Fatalf("message type = %q, want %q", left.Type, msgSessionUserLeft)
	}
	var payload model.SessionUserLeftPayload
	decodePayload(t, left, &payload)
	if payload.UserID != "user-2" || payload.ClientID != "client-2" || payload.Reason != closeReasonDisconnect {
		t.Fatalf("left payload = %#v, want user-2/client-2/disconnect", payload)
	}
}

func testService() *Service {
	return NewService(
		permissionsStub{permissions: []model.Permission{
			{SketchID: "sketch-id", UserID: "user-1", Role: "editor"},
			{SketchID: "sketch-id", UserID: "user-2", Role: "reader"},
		}},
		sketchesStub{document: &model.SketchDocument{ID: "sketch-id", Version: 7}},
	)
}

func joinMessage(requestID, clientID string, lastSeenVersion int64) model.ClientRealtimeMessage {
	payload, _ := json.Marshal(model.SessionJoinPayload{
		LastSeenVersion:          lastSeenVersion,
		ClientID:                 clientID,
		SupportedProtocolVersion: protocolVersion,
	})
	return model.ClientRealtimeMessage{
		Type:      msgSessionJoin,
		RequestID: requestID,
		SketchID:  "sketch-id",
		Payload:   payload,
	}
}

func nextMessage(t *testing.T, conn *Connection) model.ServerRealtimeMessage {
	t.Helper()
	select {
	case msg := <-conn.Outbound():
		return msg
	default:
		t.Fatal("expected outbound message")
		return model.ServerRealtimeMessage{}
	}
}

func decodePayload(t *testing.T, msg model.ServerRealtimeMessage, out any) {
	t.Helper()
	if err := json.Unmarshal(msg.Payload, out); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
}
