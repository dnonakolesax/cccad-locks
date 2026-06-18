package realtime

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/dnonakolesax/cccad-locks/internal/auth"
	"github.com/dnonakolesax/cccad-locks/internal/model"
	"github.com/mailru/easyjson"
)

type permissionsStub struct {
	permissions  []model.Permission
	putRequest   *model.Permission
	deleteUser   string
	deleteSketch string
}

func (p *permissionsStub) List(context.Context, string) ([]model.Permission, error) {
	return p.permissions, nil
}

func (p *permissionsStub) Put(_ context.Context, permission *model.Permission) (*model.Permission, error) {
	p.putRequest = permission
	return permission, nil
}

func (p *permissionsStub) Delete(_ context.Context, userID, sketchID string) error {
	p.deleteUser = userID
	p.deleteSketch = sketchID
	return nil
}

type sketchesStub struct {
	document *model.SketchDocument
}

func (s sketchesStub) Get(context.Context, string) (*model.SketchDocument, error) {
	return s.document, nil
}

type locksStub struct {
	acquireResponse *model.AcquireLockResponse
	refreshedLock   *model.SketchLock
	releasedLockID  string
}

func (l *locksStub) Acquire(
	_ context.Context,
	_ string,
	request *model.AcquireLockRequest,
) (*model.AcquireLockResponse, error) {
	if l.acquireResponse != nil {
		return l.acquireResponse, nil
	}
	return &model.AcquireLockResponse{
		Granted: true,
		Lock: &model.SketchLock{
			ID:          "lock-id",
			SketchID:    "sketch-id",
			OwnerUserID: "user-1",
			Scope:       append([]byte(nil), request.Scope...),
			Mode:        request.Mode,
			ExpiresAt:   "2026-05-22T12:00:00Z",
		},
	}, nil
}

func (l *locksStub) Refresh(
	_ context.Context,
	_ string,
	lockID string,
	request *model.RefreshLockRequest,
) (*model.SketchLock, error) {
	if l.refreshedLock != nil {
		return l.refreshedLock, nil
	}
	if request == nil || request.TTLMS != 5000 {
		return nil, model.ErrLockNotFound
	}
	return &model.SketchLock{
		ID:          lockID,
		SketchID:    "sketch-id",
		OwnerUserID: "user-1",
		Scope:       []byte(`{"type":"entity","entityId":"line-1"}`),
		Mode:        "edit",
		ExpiresAt:   "2026-05-22T12:00:05Z",
	}, nil
}

func (l *locksStub) Release(_ context.Context, _ string, lockID string) error {
	l.releasedLockID = lockID
	return nil
}

type operationsStub struct {
	submitResponse  *model.SubmitOperationResponse
	submitResponses []*model.SubmitOperationResponse
	submitUserID    string
	submitSketchID  string
	submitRequest   *model.SubmitOperationRequest
	submitRequests  []*model.SubmitOperationRequest
	listResult      *model.SketchOperationPage
	listUserID      string
}

func (o *operationsStub) List(
	ctx context.Context,
	sketchID string,
	afterVersion int64,
	limit int,
) (*model.SketchOperationPage, error) {
	o.listUserID, _ = auth.UserIDFromContext(ctx)
	if o.listResult != nil {
		return o.listResult, nil
	}
	return &model.SketchOperationPage{
		SketchID:             sketchID,
		FromVersionExclusive: afterVersion,
		ToVersion:            afterVersion,
		Ops:                  []model.CommittedOperation{},
	}, nil
}

func (o *operationsStub) Submit(
	ctx context.Context,
	sketchID string,
	request *model.SubmitOperationRequest,
) (*model.SubmitOperationResponse, error) {
	o.submitUserID, _ = auth.UserIDFromContext(ctx)
	o.submitSketchID = sketchID
	o.submitRequest = request
	o.submitRequests = append(o.submitRequests, request)
	if len(o.submitResponses) > 0 {
		response := o.submitResponses[0]
		o.submitResponses = o.submitResponses[1:]
		return response, nil
	}
	if o.submitResponse != nil {
		return o.submitResponse, nil
	}

	opID := "op-id"
	version := int64(8)
	return &model.SubmitOperationResponse{
		Accepted:         true,
		OpID:             &opID,
		Version:          &version,
		CurrentVersion:   version,
		Patch:            []byte(`{"entities":{}}`),
		SolveStatus:      []byte(`{"status":"ok","degreesOfFreedom":0}`),
		ChangedEntityIDs: []string{"line-1"},
	}, nil
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
	err = first.HandleClientMessage(ctx, joinMessage("req-1", "client-1", 6))
	if err != nil {
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
	err = second.HandleClientMessage(ctx, joinMessage("req-2", "client-2", 7))
	if err != nil {
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
	if joinedPayload.User.UserName != "User 2" || joinedPayload.ActiveUsers[0].UserName != "User 1" {
		t.Fatalf("joined usernames = user:%q active:%q, want User 2/User 1",
			joinedPayload.User.UserName,
			joinedPayload.ActiveUsers[0].UserName,
		)
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
	if joinedUser.UserName != "User 2" {
		t.Fatalf("joined userName = %q, want User 2", joinedUser.UserName)
	}
}

func TestBeginRevertAlertsBlocksMessagesAndBroadcastsSnapshot(t *testing.T) {
	service := testService()
	ctx := context.Background()

	conn, err := service.OpenConnection(ctx, model.OpenRealtimeSessionRequest{
		SketchID: "sketch-id",
		UserID:   "user-1",
		UserName: "User 1",
	})
	if err != nil {
		t.Fatalf("OpenConnection returned error: %v", err)
	}
	if err := conn.HandleClientMessage(ctx, joinMessage("req-join", "client-1", 7)); err != nil {
		t.Fatalf("session.join returned error: %v", err)
	}
	_ = nextMessage(t, conn)

	finish := service.BeginRevert(ctx, "sketch-id", 3, "user-admin")
	if finish == nil {
		t.Fatal("BeginRevert returned nil finish")
	}
	alert := nextMessage(t, conn)
	if alert.Type != msgSketchRevertStarted {
		t.Fatalf("alert type = %q, want %q", alert.Type, msgSketchRevertStarted)
	}

	err = conn.HandleClientMessage(ctx, model.ClientRealtimeMessage{
		Type:      msgSessionPing,
		RequestID: "req-ping-blocked",
		SketchID:  "sketch-id",
		Payload:   []byte(`{"clientVersion":7}`),
	})
	if !errors.Is(err, errSketchRevertInProgress) {
		t.Fatalf("blocked ping error = %v, want errSketchRevertInProgress", err)
	}
	blocked := nextMessage(t, conn)
	if blocked.Type != msgError || blocked.RequestID != "req-ping-blocked" {
		t.Fatalf("blocked message = %#v, want error req-ping-blocked", blocked)
	}
	var blockedPayload model.RealtimeErrorPayload
	decodePayload(t, blocked, &blockedPayload)
	if blockedPayload.Code != "SKETCH_REVERT_IN_PROGRESS" {
		t.Fatalf("blocked code = %q, want SKETCH_REVERT_IN_PROGRESS", blockedPayload.Code)
	}

	finish(&model.SketchDocument{
		ID:          "sketch-id",
		WorkspaceID: "workspace-id",
		Name:        "Sketch",
		Unit:        "mm",
		Version:     12,
		Entities: map[string]easyjson.RawMessage{
			"line-1": []byte(`{"type":"line"}`),
		},
		Constraints: map[string]easyjson.RawMessage{},
		Dimensions:  map[string]easyjson.RawMessage{},
		Groups:      map[string]easyjson.RawMessage{},
		SolveStatus: []byte(`{"status":"ok"}`),
	}, nil)
	snapshot := nextMessage(t, conn)
	if snapshot.Type != msgStateSnapshot {
		t.Fatalf("snapshot type = %q, want %q", snapshot.Type, msgStateSnapshot)
	}
	var snapshotPayload model.StateSnapshotPayload
	decodePayload(t, snapshot, &snapshotPayload)
	if snapshotPayload.Version != 12 {
		t.Fatalf("snapshot version = %d, want 12", snapshotPayload.Version)
	}

	err = conn.HandleClientMessage(ctx, model.ClientRealtimeMessage{
		Type:      msgSessionPing,
		RequestID: "req-ping-after",
		SketchID:  "sketch-id",
		Payload:   []byte(`{"clientVersion":12}`),
	})
	if err != nil {
		t.Fatalf("ping after revert returned error: %v", err)
	}
	pong := nextMessage(t, conn)
	if pong.Type != msgSessionPong {
		t.Fatalf("post-revert message = %#v, want session.pong", pong)
	}
	var pongPayload model.SessionPongPayload
	decodePayload(t, pong, &pongPayload)
	if pongPayload.CurrentVersion != 12 {
		t.Fatalf("pong currentVersion = %d, want 12", pongPayload.CurrentVersion)
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

	err = conn.HandleClientMessage(context.Background(), model.ClientRealtimeMessage{
		Type:      msgSessionPing,
		RequestID: "req-ping",
		SketchID:  "sketch-id",
		Payload:   json.RawMessage(`{"clientVersion":6}`),
	})
	if err != nil {
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

func TestPresenceCursorBroadcastsToOtherJoinedClients(t *testing.T) {
	service := testService()
	ctx := context.Background()

	first, err := service.OpenConnection(ctx, model.OpenRealtimeSessionRequest{
		SketchID: "sketch-id",
		UserID:   "user-1",
	})
	if err != nil {
		t.Fatalf("OpenConnection first returned error: %v", err)
	}
	err = first.HandleClientMessage(ctx, joinMessage("req-1", "client-1", 7))
	if err != nil {
		t.Fatalf("first join returned error: %v", err)
	}
	_ = nextMessage(t, first)

	second, err := service.OpenConnection(ctx, model.OpenRealtimeSessionRequest{
		SketchID: "sketch-id",
		UserID:   "user-2",
	})
	if err != nil {
		t.Fatalf("OpenConnection second returned error: %v", err)
	}
	err = second.HandleClientMessage(ctx, joinMessage("req-2", "client-2", 7))
	if err != nil {
		t.Fatalf("second join returned error: %v", err)
	}
	_ = nextMessage(t, second)
	_ = nextMessage(t, first)

	err = first.HandleClientMessage(ctx, model.ClientRealtimeMessage{
		Type:      msgPresenceCursor,
		RequestID: "req-cursor",
		SketchID:  "sketch-id",
		Payload: json.RawMessage(
			`{"x":12.5,"y":-3,"viewport":{"scale":2,"offsetX":10,"offsetY":20}}`,
		),
	})
	if err != nil {
		t.Fatalf("presence.cursor returned error: %v", err)
	}

	msg := nextMessage(t, second)
	if msg.Type != msgPresenceCursor || msg.RequestID != "req-cursor" {
		t.Fatalf("message = %#v, want presence.cursor req-cursor", msg)
	}
	var payload map[string]any
	decodePayload(t, msg, &payload)
	cursorWorld, _ := payload["cursorWorld"].(map[string]any)
	if payload["actorUserId"] != "user-1" ||
		payload["userId"] != "user-1" ||
		payload["clientId"] != "client-1" ||
		payload["x"] != 12.5 ||
		cursorWorld["x"] != 12.5 ||
		cursorWorld["y"] != -3.0 {
		t.Fatalf("presence payload = %#v, want user-1/client-1 cursor at 12.5,-3", payload)
	}
	assertNoMessage(t, first)
}

func TestPresenceCursorAcceptsCursorWorld(t *testing.T) {
	service := testService()
	ctx := context.Background()
	first, err := service.OpenConnection(ctx, model.OpenRealtimeSessionRequest{
		SketchID: "sketch-id",
		UserID:   "user-1",
	})
	if err != nil {
		t.Fatalf("OpenConnection first returned error: %v", err)
	}
	if err := first.HandleClientMessage(ctx, joinMessage("req-1", "client-1", 7)); err != nil {
		t.Fatalf("first join returned error: %v", err)
	}
	_ = nextMessage(t, first)

	second, err := service.OpenConnection(ctx, model.OpenRealtimeSessionRequest{
		SketchID: "sketch-id",
		UserID:   "user-2",
	})
	if err != nil {
		t.Fatalf("OpenConnection second returned error: %v", err)
	}
	if err := second.HandleClientMessage(ctx, joinMessage("req-2", "client-2", 7)); err != nil {
		t.Fatalf("second join returned error: %v", err)
	}
	_ = nextMessage(t, second)
	_ = nextMessage(t, first)

	err = first.HandleClientMessage(ctx, model.ClientRealtimeMessage{
		Type:      msgPresenceCursor,
		RequestID: "req-cursor",
		SketchID:  "sketch-id",
		Payload: json.RawMessage(
			`{"cursorWorld":{"x":0,"y":0},"ttlMs":3000}`,
		),
	})
	if err != nil {
		t.Fatalf("presence.cursor returned error: %v", err)
	}

	msg := nextMessage(t, second)
	var payload map[string]any
	decodePayload(t, msg, &payload)
	cursorWorld, _ := payload["cursorWorld"].(map[string]any)
	if payload["actorUserId"] != "user-1" ||
		payload["clientId"] != "client-1" ||
		payload["x"] != 0.0 ||
		payload["y"] != 0.0 ||
		cursorWorld["x"] != 0.0 ||
		cursorWorld["y"] != 0.0 {
		t.Fatalf("presence payload = %#v, want cursorWorld and legacy coordinates at 0,0", payload)
	}
}

func TestPresenceGenericMessagesBroadcast(t *testing.T) {
	service := testService()
	ctx := context.Background()

	first, err := service.OpenConnection(ctx, model.OpenRealtimeSessionRequest{
		SketchID: "sketch-id",
		UserID:   "user-1",
	})
	if err != nil {
		t.Fatalf("OpenConnection first returned error: %v", err)
	}
	err = first.HandleClientMessage(ctx, joinMessage("req-1", "client-1", 7))
	if err != nil {
		t.Fatalf("first join returned error: %v", err)
	}
	_ = nextMessage(t, first)

	second, err := service.OpenConnection(ctx, model.OpenRealtimeSessionRequest{
		SketchID: "sketch-id",
		UserID:   "user-2",
	})
	if err != nil {
		t.Fatalf("OpenConnection second returned error: %v", err)
	}
	err = second.HandleClientMessage(ctx, joinMessage("req-2", "client-2", 7))
	if err != nil {
		t.Fatalf("second join returned error: %v", err)
	}
	_ = nextMessage(t, second)
	_ = nextMessage(t, first)

	err = second.HandleClientMessage(ctx, model.ClientRealtimeMessage{
		Type:      msgPresenceSelection,
		RequestID: "req-selection",
		SketchID:  "sketch-id",
		Payload:   json.RawMessage(`{"entityIds":["line-1"],"constraintIds":["constraint-1"]}`),
	})
	if err != nil {
		t.Fatalf("presence.selection returned error: %v", err)
	}

	msg := nextMessage(t, first)
	if msg.Type != msgPresenceSelection || msg.RequestID != "req-selection" {
		t.Fatalf("message = %#v, want presence.selection req-selection", msg)
	}
	var payload map[string]any
	decodePayload(t, msg, &payload)
	if payload["userId"] != "user-2" || payload["clientId"] != "client-2" {
		t.Fatalf("presence payload = %#v, want user-2/client-2", payload)
	}
}

func TestPresenceRejectsInvalidCursor(t *testing.T) {
	service := testService()
	conn, err := service.OpenConnection(context.Background(), model.OpenRealtimeSessionRequest{
		SketchID: "sketch-id",
		UserID:   "user-1",
	})
	if err != nil {
		t.Fatalf("OpenConnection returned error: %v", err)
	}
	err = conn.HandleClientMessage(context.Background(), joinMessage("req-join", "client-1", 7))
	if err != nil {
		t.Fatalf("join returned error: %v", err)
	}
	_ = nextMessage(t, conn)

	err = conn.HandleClientMessage(context.Background(), model.ClientRealtimeMessage{
		Type:      msgPresenceCursor,
		RequestID: "req-cursor",
		SketchID:  "sketch-id",
		Payload:   json.RawMessage(`{"x":12}`),
	})
	if err == nil {
		t.Fatal("presence.cursor returned nil error")
	}
	msg := nextMessage(t, conn)
	if msg.Type != msgError || msg.RequestID != "req-cursor" {
		t.Fatalf("message = %#v, want error req-cursor", msg)
	}
}

func TestLockAcquireRepliesWithAcquired(t *testing.T) {
	service := testService()
	conn, err := service.OpenConnection(context.Background(), model.OpenRealtimeSessionRequest{
		SketchID: "sketch-id",
		UserID:   "user-1",
	})
	if err != nil {
		t.Fatalf("OpenConnection returned error: %v", err)
	}

	err = conn.HandleClientMessage(context.Background(), model.ClientRealtimeMessage{
		Type:      msgLockAcquire,
		RequestID: "req-lock",
		SketchID:  "sketch-id",
		Payload:   json.RawMessage(`{"scope":{"type":"entity","entityId":"line-1"},"mode":"edit","ttlMs":5000}`),
	})
	if err != nil {
		t.Fatalf("lock.acquire returned error: %v", err)
	}

	msg := nextMessage(t, conn)
	if msg.Type != msgLockAcquired || msg.RequestID != "req-lock" {
		t.Fatalf("message = %#v, want lock.acquired req-lock", msg)
	}
	var payload model.LockAcquiredPayload
	decodePayload(t, msg, &payload)
	if payload.LockID != "lock-id" || payload.Scope.EntityID != "line-1" || payload.ExpiresAt == "" {
		t.Fatalf("payload = %#v, want acquired lock for line-1", payload)
	}
}

func TestLockAcquireRejectsReader(t *testing.T) {
	service := testService()
	conn, err := service.OpenConnection(context.Background(), model.OpenRealtimeSessionRequest{
		SketchID: "sketch-id",
		UserID:   "user-2",
	})
	if err != nil {
		t.Fatalf("OpenConnection returned error: %v", err)
	}

	err = conn.HandleClientMessage(context.Background(), model.ClientRealtimeMessage{
		Type:      msgLockAcquire,
		RequestID: "req-lock",
		SketchID:  "sketch-id",
		Payload:   json.RawMessage(`{"scope":{"type":"entity","entityId":"line-1"},"mode":"edit","ttlMs":5000}`),
	})
	if err != nil {
		t.Fatalf("lock.acquire returned error: %v", err)
	}

	msg := nextMessage(t, conn)
	if msg.Type != msgLockRejected {
		t.Fatalf("message type = %q, want %q", msg.Type, msgLockRejected)
	}
	var payload model.LockRejectedPayload
	decodePayload(t, msg, &payload)
	if payload.Reason != "permission_denied" {
		t.Fatalf("reason = %q, want permission_denied", payload.Reason)
	}
}

func TestLockRefreshAndReleaseReply(t *testing.T) {
	service := testService()
	conn, err := service.OpenConnection(context.Background(), model.OpenRealtimeSessionRequest{
		SketchID: "sketch-id",
		UserID:   "user-1",
	})
	if err != nil {
		t.Fatalf("OpenConnection returned error: %v", err)
	}

	err = conn.HandleClientMessage(context.Background(), model.ClientRealtimeMessage{
		Type:      msgLockRefresh,
		RequestID: "req-refresh",
		SketchID:  "sketch-id",
		Payload:   json.RawMessage(`{"lockId":"lock-id","ttlMs":5000}`),
	})
	if err != nil {
		t.Fatalf("lock.refresh returned error: %v", err)
	}
	refreshed := nextMessage(t, conn)
	if refreshed.Type != msgLockRefreshed {
		t.Fatalf("message type = %q, want %q", refreshed.Type, msgLockRefreshed)
	}
	var refreshPayload model.LockRefreshedPayload
	decodePayload(t, refreshed, &refreshPayload)
	if refreshPayload.LockID != "lock-id" || refreshPayload.ExpiresAt == "" {
		t.Fatalf("refresh payload = %#v, want lock-id with expiresAt", refreshPayload)
	}

	err = conn.HandleClientMessage(context.Background(), model.ClientRealtimeMessage{
		Type:      msgLockRelease,
		RequestID: "req-release",
		SketchID:  "sketch-id",
		Payload:   json.RawMessage(`{"lockId":"lock-id"}`),
	})
	if err != nil {
		t.Fatalf("lock.release returned error: %v", err)
	}
	released := nextMessage(t, conn)
	if released.Type != msgLockReleased {
		t.Fatalf("message type = %q, want %q", released.Type, msgLockReleased)
	}
	var releasePayload model.LockReleasedPayload
	decodePayload(t, released, &releasePayload)
	if releasePayload.LockID != "lock-id" || releasePayload.Reason != "released" || releasePayload.UserID != "user-1" {
		t.Fatalf("release payload = %#v, want released lock-id by user-1", releasePayload)
	}
}

func TestOpSubmitRepliesAndBroadcastsCommitted(t *testing.T) {
	ops := &operationsStub{}
	service := testServiceWithOperations(ops)
	ctx := context.Background()

	first, err := service.OpenConnection(ctx, model.OpenRealtimeSessionRequest{
		SketchID: "sketch-id",
		UserID:   "user-1",
	})
	if err != nil {
		t.Fatalf("OpenConnection first returned error: %v", err)
	}
	err = first.HandleClientMessage(ctx, joinMessage("req-1", "client-1", 7))
	if err != nil {
		t.Fatalf("first join returned error: %v", err)
	}
	_ = nextMessage(t, first)

	second, err := service.OpenConnection(ctx, model.OpenRealtimeSessionRequest{
		SketchID: "sketch-id",
		UserID:   "user-3",
	})
	if err != nil {
		t.Fatalf("OpenConnection second returned error: %v", err)
	}
	err = second.HandleClientMessage(ctx, joinMessage("req-2", "client-2", 7))
	if err != nil {
		t.Fatalf("second join returned error: %v", err)
	}
	_ = nextMessage(t, second)
	_ = nextMessage(t, first)

	err = first.HandleClientMessage(ctx, model.ClientRealtimeMessage{
		Type:      msgOpSubmit,
		RequestID: "req-op",
		SketchID:  "sketch-id",
		Payload:   json.RawMessage(`{"baseVersion":7,"clientOpId":"client-op-1","op":{"type":"move_point"}}`),
	})
	if err != nil {
		t.Fatalf("op.submit returned error: %v", err)
	}

	reply := nextMessage(t, first)
	if reply.Type != msgOpCommitted || reply.RequestID != "req-op" {
		t.Fatalf("reply = %#v, want op.committed req-op", reply)
	}
	var payload model.OpCommittedPayload
	decodePayload(t, reply, &payload)
	if payload.OpID != "op-id" || payload.Version != 8 || payload.ActorUserID != "user-1" {
		t.Fatalf("committed payload = %#v, want op-id version 8 actor user-1", payload)
	}
	if ops.submitUserID != "user-1" || ops.submitSketchID != "sketch-id" {
		t.Fatalf("submit context = %q/%q, want user-1/sketch-id", ops.submitUserID, ops.submitSketchID)
	}
	if ops.submitRequest == nil || ops.submitRequest.ClientOpID != "client-op-1" {
		t.Fatalf("submit request = %#v, want client-op-1", ops.submitRequest)
	}

	broadcast := nextMessage(t, second)
	if broadcast.Type != msgOpCommitted || broadcast.RequestID != "req-op" {
		t.Fatalf("broadcast = %#v, want op.committed req-op", broadcast)
	}
}

func TestIntentDraftUpdateBroadcastsTransientDraft(t *testing.T) {
	service := testService()
	ctx := context.Background()

	first, err := service.OpenConnection(ctx, model.OpenRealtimeSessionRequest{
		SketchID: "sketch-id",
		UserID:   "user-1",
	})
	if err != nil {
		t.Fatalf("OpenConnection first returned error: %v", err)
	}
	if err := first.HandleClientMessage(ctx, joinMessage("req-1", "client-1", 7)); err != nil {
		t.Fatalf("first join returned error: %v", err)
	}
	_ = nextMessage(t, first)

	second, err := service.OpenConnection(ctx, model.OpenRealtimeSessionRequest{
		SketchID: "sketch-id",
		UserID:   "user-3",
	})
	if err != nil {
		t.Fatalf("OpenConnection second returned error: %v", err)
	}
	if err := second.HandleClientMessage(ctx, joinMessage("req-2", "client-2", 7)); err != nil {
		t.Fatalf("second join returned error: %v", err)
	}
	_ = nextMessage(t, second)
	_ = nextMessage(t, first)

	err = first.HandleClientMessage(ctx, model.ClientRealtimeMessage{
		Type:      msgIntentDraftUpdate,
		RequestID: "req-draft",
		SketchID:  "sketch-id",
		Payload:   json.RawMessage(`{"draftId":"draft-1","baseVersion":7,"tool":"line","phase":"select_second_point","sequence":2}`),
	})
	if err != nil {
		t.Fatalf("intent.draft.update returned error: %v", err)
	}

	broadcast := nextMessage(t, second)
	if broadcast.Type != msgIntentDraftUpdate || broadcast.RequestID != "req-draft" {
		t.Fatalf("broadcast = %#v, want intent.draft.update req-draft", broadcast)
	}
	var payload model.IntentDraftPayload
	decodePayload(t, broadcast, &payload)
	if payload.DraftID != "draft-1" || payload.ActorUserID != "user-1" || payload.ClientID != "client-1" {
		t.Fatalf("draft payload = %#v, want draft-1 from user-1/client-1", payload)
	}
}

func TestSyncResumeProcessesOfflineOperationsAndBroadcastsCommitted(t *testing.T) {
	opID1 := "op-8"
	version1 := int64(8)
	opID2 := "op-9"
	version2 := int64(9)
	ops := &operationsStub{
		listResult: &model.SketchOperationPage{
			SketchID:             "sketch-id",
			FromVersionExclusive: 6,
			ToVersion:            7,
			Ops: []model.CommittedOperation{
				{
					ID:          "remote-op-7",
					SketchID:    "sketch-id",
					Version:     7,
					ActorUserID: "user-3",
					ClientOpID:  optionalTestString("remote-client-op"),
					Patch:       []byte(`{"entities":{"remote":{}}}`),
					SolveStatus: []byte(`{"status":"ok"}`),
				},
			},
		},
		submitResponses: []*model.SubmitOperationResponse{
			{
				Accepted:         true,
				OpID:             &opID1,
				Version:          &version1,
				CurrentVersion:   version1,
				Patch:            []byte(`{"entities":{"p1":{}}}`),
				SolveStatus:      []byte(`{"status":"ok"}`),
				ChangedEntityIDs: []string{"p1"},
			},
			{
				Accepted:         true,
				OpID:             &opID2,
				Version:          &version2,
				CurrentVersion:   version2,
				Patch:            []byte(`{"entities":{"p2":{}}}`),
				SolveStatus:      []byte(`{"status":"ok"}`),
				ChangedEntityIDs: []string{"p2"},
			},
		},
	}
	service := testServiceWithOperations(ops)
	ctx := context.Background()

	first, err := service.OpenConnection(ctx, model.OpenRealtimeSessionRequest{
		SketchID: "sketch-id",
		UserID:   "user-1",
	})
	if err != nil {
		t.Fatalf("OpenConnection first returned error: %v", err)
	}
	if err := first.HandleClientMessage(ctx, joinMessage("req-1", "client-1", 7)); err != nil {
		t.Fatalf("first join returned error: %v", err)
	}
	_ = nextMessage(t, first)

	second, err := service.OpenConnection(ctx, model.OpenRealtimeSessionRequest{
		SketchID: "sketch-id",
		UserID:   "user-3",
	})
	if err != nil {
		t.Fatalf("OpenConnection second returned error: %v", err)
	}
	if err := second.HandleClientMessage(ctx, joinMessage("req-2", "client-2", 7)); err != nil {
		t.Fatalf("second join returned error: %v", err)
	}
	_ = nextMessage(t, second)
	_ = nextMessage(t, first)

	err = first.HandleClientMessage(ctx, model.ClientRealtimeMessage{
		Type:      msgSyncResume,
		RequestID: "req-sync",
		SketchID:  "sketch-id",
		Payload: json.RawMessage(`{
			"clientId":"client-1",
				"lastSeenVersion":6,
			"pendingOps":[
				{"clientOpId":"client-op-2","clientSeq":2,"baseVersion":7,"op":{"type":"create_point","pointId":"p2"}},
				{"clientOpId":"client-op-1","clientSeq":1,"baseVersion":7,"op":{"type":"create_point","pointId":"p1"}}
			]
		}`),
	})
	if err != nil {
		t.Fatalf("sync.resume returned error: %v", err)
	}

	reply := nextMessage(t, first)
	if reply.Type != msgSyncResumeResult || reply.RequestID != "req-sync" {
		t.Fatalf("reply = %#v, want sync.resume.result req-sync", reply)
	}
	var payload model.SyncResumeResultPayload
	decodePayload(t, reply, &payload)
	if payload.Status != "ok" || payload.CurrentVersion != 9 || len(payload.OpResults) != 2 {
		t.Fatalf("resume payload = %#v, want ok currentVersion 9 with 2 results", payload)
	}
	if len(payload.MissedPatches) != 1 || payload.MissedPatches[0].OpID != "remote-op-7" {
		t.Fatalf("missed patches = %#v, want remote-op-7", payload.MissedPatches)
	}
	if payload.OpResults[0].ClientOpID != "client-op-1" || payload.OpResults[0].CommittedVersion != 8 {
		t.Fatalf("first op result = %#v, want client-op-1 committed at 8", payload.OpResults[0])
	}
	if ops.listUserID != "user-1" {
		t.Fatalf("list userID = %q, want user-1", ops.listUserID)
	}
	if len(ops.submitRequests) != 2 || ops.submitRequests[0].BaseVersion != 7 || ops.submitRequests[1].BaseVersion != 8 {
		t.Fatalf("submit requests = %#v, want rebased versions 7 then 8", ops.submitRequests)
	}

	firstBroadcast := nextMessage(t, second)
	secondBroadcast := nextMessage(t, second)
	if firstBroadcast.Type != msgOpCommitted || secondBroadcast.Type != msgOpCommitted {
		t.Fatalf("broadcasts = %#v/%#v, want op.committed", firstBroadcast, secondBroadcast)
	}
}

func TestOpSubmitRejectedByService(t *testing.T) {
	service := testServiceWithOperations(&operationsStub{
		submitResponse: &model.SubmitOperationResponse{
			Accepted:       false,
			CurrentVersion: 9,
			Rejection: &model.OperationRejection{
				Reason:  "stale_base_version",
				Message: "base version is stale",
			},
		},
	})
	conn, err := service.OpenConnection(context.Background(), model.OpenRealtimeSessionRequest{
		SketchID: "sketch-id",
		UserID:   "user-1",
	})
	if err != nil {
		t.Fatalf("OpenConnection returned error: %v", err)
	}

	err = conn.HandleClientMessage(context.Background(), model.ClientRealtimeMessage{
		Type:      msgOpSubmit,
		RequestID: "req-op",
		SketchID:  "sketch-id",
		Payload:   json.RawMessage(`{"baseVersion":7,"clientOpId":"client-op-1","op":{"type":"move_point"}}`),
	})
	if err != nil {
		t.Fatalf("op.submit returned error: %v", err)
	}

	msg := nextMessage(t, conn)
	if msg.Type != msgOpRejected {
		t.Fatalf("message type = %q, want %q", msg.Type, msgOpRejected)
	}
	var payload model.OpRejectedPayload
	decodePayload(t, msg, &payload)
	if payload.ClientOpID != "client-op-1" || payload.CurrentVersion != 9 || payload.Reason != "stale_base_version" {
		t.Fatalf("rejected payload = %#v, want stale client-op-1 at version 9", payload)
	}
}

func TestOpSubmitRejectsReader(t *testing.T) {
	service := testService()
	conn, err := service.OpenConnection(context.Background(), model.OpenRealtimeSessionRequest{
		SketchID: "sketch-id",
		UserID:   "user-2",
	})
	if err != nil {
		t.Fatalf("OpenConnection returned error: %v", err)
	}

	err = conn.HandleClientMessage(context.Background(), model.ClientRealtimeMessage{
		Type:      msgOpSubmit,
		RequestID: "req-op",
		SketchID:  "sketch-id",
		Payload:   json.RawMessage(`{"baseVersion":7,"clientOpId":"client-op-1","op":{"type":"move_point"}}`),
	})
	if err != nil {
		t.Fatalf("op.submit returned error: %v", err)
	}

	msg := nextMessage(t, conn)
	if msg.Type != msgOpRejected {
		t.Fatalf("message type = %q, want %q", msg.Type, msgOpRejected)
	}
	var payload model.OpRejectedPayload
	decodePayload(t, msg, &payload)
	if payload.ClientOpID != "client-op-1" || payload.Reason != "permission_denied" {
		t.Fatalf("rejected payload = %#v, want permission_denied for client-op-1", payload)
	}
}

func TestDragBeginAcquiresEntityLock(t *testing.T) {
	service := testService()
	conn, err := service.OpenConnection(context.Background(), model.OpenRealtimeSessionRequest{
		SketchID: "sketch-id",
		UserID:   "user-1",
	})
	if err != nil {
		t.Fatalf("OpenConnection returned error: %v", err)
	}

	err = conn.HandleClientMessage(context.Background(), model.ClientRealtimeMessage{
		Type:      msgDragBegin,
		RequestID: "req-drag",
		SketchID:  "sketch-id",
		Payload:   json.RawMessage(`{"entityId":"line-1","kind":"line","baseVersion":7}`),
	})
	if err != nil {
		t.Fatalf("drag.begin returned error: %v", err)
	}

	msg := nextMessage(t, conn)
	if msg.Type != msgDragBeginAccepted || msg.RequestID != "req-drag" {
		t.Fatalf("message = %#v, want drag.begin.accepted req-drag", msg)
	}
	var payload model.DragBeginAcceptedPayload
	decodePayload(t, msg, &payload)
	if payload.LockID != "lock-id" || len(payload.LockedEntityIDs) != 1 || payload.LockedEntityIDs[0] != "line-1" {
		t.Fatalf("payload = %#v, want lock for line-1", payload)
	}
}

func TestDragBeginRejectsLockConflict(t *testing.T) {
	service := newTestService(testPermissions(), &operationsStub{})
	service.locks = &locksStub{acquireResponse: &model.AcquireLockResponse{
		Granted: false,
		Conflict: &model.LockConflict{
			HolderUserID: "user-3",
			LockID:       "lock-other",
		},
	}}
	conn, err := service.OpenConnection(context.Background(), model.OpenRealtimeSessionRequest{
		SketchID: "sketch-id",
		UserID:   "user-1",
	})
	if err != nil {
		t.Fatalf("OpenConnection returned error: %v", err)
	}

	err = conn.HandleClientMessage(context.Background(), model.ClientRealtimeMessage{
		Type:      msgDragBegin,
		RequestID: "req-drag",
		SketchID:  "sketch-id",
		Payload:   json.RawMessage(`{"entityId":"line-1","kind":"line","baseVersion":7}`),
	})
	if err != nil {
		t.Fatalf("drag.begin returned error: %v", err)
	}

	msg := nextMessage(t, conn)
	if msg.Type != msgDragBeginRejected {
		t.Fatalf("message type = %q, want %q", msg.Type, msgDragBeginRejected)
	}
	var payload model.DragBeginRejectedPayload
	decodePayload(t, msg, &payload)
	if payload.Reason != "lock_conflict" || payload.LockedByUserID != "user-3" || payload.LockID != "lock-other" {
		t.Fatalf("payload = %#v, want lock conflict from user-3", payload)
	}
}

func TestDragPreviewBroadcastsToOtherJoinedClients(t *testing.T) {
	service := testService()
	ctx := context.Background()

	first, err := service.OpenConnection(ctx, model.OpenRealtimeSessionRequest{
		SketchID: "sketch-id",
		UserID:   "user-1",
	})
	if err != nil {
		t.Fatalf("OpenConnection first returned error: %v", err)
	}
	err = first.HandleClientMessage(ctx, joinMessage("req-1", "client-1", 7))
	if err != nil {
		t.Fatalf("first join returned error: %v", err)
	}
	_ = nextMessage(t, first)

	second, err := service.OpenConnection(ctx, model.OpenRealtimeSessionRequest{
		SketchID: "sketch-id",
		UserID:   "user-3",
	})
	if err != nil {
		t.Fatalf("OpenConnection second returned error: %v", err)
	}
	err = second.HandleClientMessage(ctx, joinMessage("req-2", "client-2", 7))
	if err != nil {
		t.Fatalf("second join returned error: %v", err)
	}
	_ = nextMessage(t, second)
	_ = nextMessage(t, first)

	err = first.HandleClientMessage(ctx, model.ClientRealtimeMessage{
		Type:      msgDragPreview,
		RequestID: "req-preview",
		SketchID:  "sketch-id",
		Payload:   json.RawMessage(`{"lockId":"lock-id","entityId":"line-1","target":{"x":10,"y":20}}`),
	})
	if err != nil {
		t.Fatalf("drag.preview returned error: %v", err)
	}

	msg := nextMessage(t, second)
	if msg.Type != msgDragPreview || msg.RequestID != "req-preview" {
		t.Fatalf("message = %#v, want drag.preview req-preview", msg)
	}
	var payload map[string]any
	decodePayload(t, msg, &payload)
	if payload["userId"] != "user-1" || payload["clientId"] != "client-1" || payload["entityId"] != "line-1" {
		t.Fatalf("payload = %#v, want user-1/client-1 line-1", payload)
	}
	assertNoMessage(t, first)
}

func TestDragCommitSubmitsOperationWithLockID(t *testing.T) {
	ops := &operationsStub{}
	service := testServiceWithOperations(ops)
	conn, err := service.OpenConnection(context.Background(), model.OpenRealtimeSessionRequest{
		SketchID: "sketch-id",
		UserID:   "user-1",
	})
	if err != nil {
		t.Fatalf("OpenConnection returned error: %v", err)
	}

	err = conn.HandleClientMessage(context.Background(), model.ClientRealtimeMessage{
		Type:      msgDragCommit,
		RequestID: "req-commit",
		SketchID:  "sketch-id",
		Payload: json.RawMessage(
			`{"lockId":"lock-id","baseVersion":7,"clientOpId":"drag-op-1","op":{"type":"move_point"}}`,
		),
	})
	if err != nil {
		t.Fatalf("drag.commit returned error: %v", err)
	}

	msg := nextMessage(t, conn)
	if msg.Type != msgOpCommitted || msg.RequestID != "req-commit" {
		t.Fatalf("message = %#v, want op.committed req-commit", msg)
	}
	if ops.submitRequest == nil || ops.submitRequest.LockID == nil || *ops.submitRequest.LockID != "lock-id" {
		t.Fatalf("submit request = %#v, want lock-id", ops.submitRequest)
	}
}

func TestDragCancelReleasesLockAndBroadcastsCancelled(t *testing.T) {
	locks := &locksStub{}
	service := newTestService(testPermissions(), &operationsStub{})
	service.locks = locks
	ctx := context.Background()

	first, err := service.OpenConnection(ctx, model.OpenRealtimeSessionRequest{
		SketchID: "sketch-id",
		UserID:   "user-1",
	})
	if err != nil {
		t.Fatalf("OpenConnection first returned error: %v", err)
	}
	err = first.HandleClientMessage(ctx, joinMessage("req-1", "client-1", 7))
	if err != nil {
		t.Fatalf("first join returned error: %v", err)
	}
	_ = nextMessage(t, first)

	second, err := service.OpenConnection(ctx, model.OpenRealtimeSessionRequest{
		SketchID: "sketch-id",
		UserID:   "user-3",
	})
	if err != nil {
		t.Fatalf("OpenConnection second returned error: %v", err)
	}
	err = second.HandleClientMessage(ctx, joinMessage("req-2", "client-2", 7))
	if err != nil {
		t.Fatalf("second join returned error: %v", err)
	}
	_ = nextMessage(t, second)
	_ = nextMessage(t, first)

	err = first.HandleClientMessage(ctx, model.ClientRealtimeMessage{
		Type:      msgDragCancel,
		RequestID: "req-cancel",
		SketchID:  "sketch-id",
		Payload:   json.RawMessage(`{"lockId":"lock-id","entityId":"line-1"}`),
	})
	if err != nil {
		t.Fatalf("drag.cancel returned error: %v", err)
	}
	if locks.releasedLockID != "lock-id" {
		t.Fatalf("released lock = %q, want lock-id", locks.releasedLockID)
	}

	reply := nextMessage(t, first)
	if reply.Type != msgDragCancelled || reply.RequestID != "req-cancel" {
		t.Fatalf("reply = %#v, want drag.cancelled req-cancel", reply)
	}
	broadcast := nextMessage(t, second)
	if broadcast.Type != msgDragCancelled || broadcast.RequestID != "req-cancel" {
		t.Fatalf("broadcast = %#v, want drag.cancelled req-cancel", broadcast)
	}
}

func TestPermissionUpdatedByAdminPersistsAndBroadcasts(t *testing.T) {
	permissions := testPermissions()
	service := newTestService(permissions, &operationsStub{})
	ctx := context.Background()

	admin, err := service.OpenConnection(ctx, model.OpenRealtimeSessionRequest{
		SketchID: "sketch-id",
		UserID:   "user-admin",
	})
	if err != nil {
		t.Fatalf("OpenConnection admin returned error: %v", err)
	}
	err = admin.HandleClientMessage(ctx, joinMessage("req-admin", "client-admin", 7))
	if err != nil {
		t.Fatalf("admin join returned error: %v", err)
	}
	_ = nextMessage(t, admin)

	editor, err := service.OpenConnection(ctx, model.OpenRealtimeSessionRequest{
		SketchID: "sketch-id",
		UserID:   "user-1",
	})
	if err != nil {
		t.Fatalf("OpenConnection editor returned error: %v", err)
	}
	err = editor.HandleClientMessage(ctx, joinMessage("req-editor", "client-editor", 7))
	if err != nil {
		t.Fatalf("editor join returned error: %v", err)
	}
	_ = nextMessage(t, editor)
	_ = nextMessage(t, admin)

	err = admin.HandleClientMessage(ctx, model.ClientRealtimeMessage{
		Type:      msgPermissionUpdated,
		RequestID: "req-perm",
		SketchID:  "sketch-id",
		Payload:   json.RawMessage(`{"targetUserId":"user-2","role":"editor"}`),
	})
	if err != nil {
		t.Fatalf("permission.updated returned error: %v", err)
	}

	if permissions.putRequest == nil || permissions.putRequest.UserID != "user-2" ||
		permissions.putRequest.Role != roleEditor {
		t.Fatalf("put permission = %#v, want user-2 editor", permissions.putRequest)
	}
	if permissions.putRequest.GrantedByUserID == nil || *permissions.putRequest.GrantedByUserID != "user-admin" {
		t.Fatalf("grantedBy = %#v, want user-admin", permissions.putRequest.GrantedByUserID)
	}

	adminMsg := nextMessage(t, admin)
	if adminMsg.Type != msgPermissionUpdated {
		t.Fatalf("admin message type = %q, want %q", adminMsg.Type, msgPermissionUpdated)
	}
	var payload model.PermissionUpdatedPayload
	decodePayload(t, adminMsg, &payload)
	if payload.TargetUserID != "user-2" || payload.Role != roleEditor || payload.ChangedByUserID != "user-admin" {
		t.Fatalf("permission payload = %#v, want user-2/editor/user-admin", payload)
	}

	editorMsg := nextMessage(t, editor)
	if editorMsg.Type != msgPermissionUpdated {
		t.Fatalf("editor message type = %q, want %q", editorMsg.Type, msgPermissionUpdated)
	}
}

func TestPermissionRevokedClosesTargetConnections(t *testing.T) {
	permissions := testPermissions()
	service := newTestService(permissions, &operationsStub{})
	ctx := context.Background()

	admin, err := service.OpenConnection(ctx, model.OpenRealtimeSessionRequest{
		SketchID: "sketch-id",
		UserID:   "user-admin",
	})
	if err != nil {
		t.Fatalf("OpenConnection admin returned error: %v", err)
	}
	err = admin.HandleClientMessage(ctx, joinMessage("req-admin", "client-admin", 7))
	if err != nil {
		t.Fatalf("admin join returned error: %v", err)
	}
	_ = nextMessage(t, admin)

	target, err := service.OpenConnection(ctx, model.OpenRealtimeSessionRequest{
		SketchID: "sketch-id",
		UserID:   "user-1",
	})
	if err != nil {
		t.Fatalf("OpenConnection target returned error: %v", err)
	}
	err = target.HandleClientMessage(ctx, joinMessage("req-target", "client-target", 7))
	if err != nil {
		t.Fatalf("target join returned error: %v", err)
	}
	_ = nextMessage(t, target)
	_ = nextMessage(t, admin)

	err = admin.HandleClientMessage(ctx, model.ClientRealtimeMessage{
		Type:      msgPermissionRevoked,
		RequestID: "req-revoke",
		SketchID:  "sketch-id",
		Payload:   json.RawMessage(`{"targetUserId":"user-1"}`),
	})
	if err != nil {
		t.Fatalf("permission.revoked returned error: %v", err)
	}
	if permissions.deleteUser != "user-1" || permissions.deleteSketch != "sketch-id" {
		t.Fatalf("delete permission = %q/%q, want user-1/sketch-id", permissions.deleteUser, permissions.deleteSketch)
	}

	adminMsg := nextMessage(t, admin)
	if adminMsg.Type != msgPermissionRevoked {
		t.Fatalf("admin message type = %q, want %q", adminMsg.Type, msgPermissionRevoked)
	}
	accessRevoked := nextMessage(t, target)
	if accessRevoked.Type != msgPermissionRevoked {
		t.Fatalf("target first message type = %q, want %q", accessRevoked.Type, msgPermissionRevoked)
	}
	accessRevoked = nextMessage(t, target)
	if accessRevoked.Type != msgSessionAccessRevoked {
		t.Fatalf("target second message type = %q, want %q", accessRevoked.Type, msgSessionAccessRevoked)
	}
	left := nextMessage(t, admin)
	if left.Type != msgSessionUserLeft {
		t.Fatalf("admin follow-up type = %q, want %q", left.Type, msgSessionUserLeft)
	}
}

func TestPermissionUpdatedRejectsNonAdmin(t *testing.T) {
	service := testService()
	conn, err := service.OpenConnection(context.Background(), model.OpenRealtimeSessionRequest{
		SketchID: "sketch-id",
		UserID:   "user-1",
	})
	if err != nil {
		t.Fatalf("OpenConnection returned error: %v", err)
	}

	err = conn.HandleClientMessage(context.Background(), model.ClientRealtimeMessage{
		Type:      msgPermissionUpdated,
		RequestID: "req-perm",
		SketchID:  "sketch-id",
		Payload:   json.RawMessage(`{"targetUserId":"user-2","role":"editor"}`),
	})
	if err == nil {
		t.Fatal("permission.updated returned nil error for non-admin")
	}
	msg := nextMessage(t, conn)
	if msg.Type != msgError {
		t.Fatalf("message type = %q, want %q", msg.Type, msgError)
	}
}

func TestConflictCreatedBroadcastsToSketch(t *testing.T) {
	service := testService()
	ctx := context.Background()

	first, err := service.OpenConnection(ctx, model.OpenRealtimeSessionRequest{
		SketchID: "sketch-id",
		UserID:   "user-1",
	})
	if err != nil {
		t.Fatalf("OpenConnection first returned error: %v", err)
	}
	err = first.HandleClientMessage(ctx, joinMessage("req-1", "client-1", 7))
	if err != nil {
		t.Fatalf("first join returned error: %v", err)
	}
	_ = nextMessage(t, first)

	second, err := service.OpenConnection(ctx, model.OpenRealtimeSessionRequest{
		SketchID: "sketch-id",
		UserID:   "user-3",
	})
	if err != nil {
		t.Fatalf("OpenConnection second returned error: %v", err)
	}
	err = second.HandleClientMessage(ctx, joinMessage("req-2", "client-2", 7))
	if err != nil {
		t.Fatalf("second join returned error: %v", err)
	}
	_ = nextMessage(t, second)
	_ = nextMessage(t, first)

	err = first.HandleClientMessage(ctx, model.ClientRealtimeMessage{
		Type:      msgConflictCreated,
		RequestID: "req-conflict",
		SketchID:  "sketch-id",
		Payload: json.RawMessage(
			`{"conflictId":"conflict-1","conflictType":"over_constrained","status":"open","affectedEntityIds":["line-1"],"causedByOps":["op-1"],"message":"over constrained"}`,
		),
	})
	if err != nil {
		t.Fatalf("conflict.created returned error: %v", err)
	}

	for _, conn := range []*Connection{first, second} {
		msg := nextMessage(t, conn)
		if msg.Type != msgConflictCreated || msg.RequestID != "req-conflict" {
			t.Fatalf("message = %#v, want conflict.created req-conflict", msg)
		}
		var payload model.ConflictCreatedPayload
		decodePayload(t, msg, &payload)
		if payload.ConflictID != "conflict-1" || payload.Status != "open" {
			t.Fatalf("payload = %#v, want conflict-1 open", payload)
		}
	}
}

func TestConflictResolvedDefaultsResolverAndBroadcasts(t *testing.T) {
	service := testService()
	ctx := context.Background()

	first, err := service.OpenConnection(ctx, model.OpenRealtimeSessionRequest{
		SketchID: "sketch-id",
		UserID:   "user-1",
	})
	if err != nil {
		t.Fatalf("OpenConnection returned error: %v", err)
	}
	err = first.HandleClientMessage(ctx, joinMessage("req-1", "client-1", 7))
	if err != nil {
		t.Fatalf("join returned error: %v", err)
	}
	_ = nextMessage(t, first)

	err = first.HandleClientMessage(ctx, model.ClientRealtimeMessage{
		Type:      msgConflictResolved,
		RequestID: "req-resolve",
		SketchID:  "sketch-id",
		Payload:   json.RawMessage(`{"conflictId":"conflict-1","resolutionOpId":"op-2"}`),
	})
	if err != nil {
		t.Fatalf("conflict.resolved returned error: %v", err)
	}

	msg := nextMessage(t, first)
	if msg.Type != msgConflictResolved {
		t.Fatalf("message type = %q, want %q", msg.Type, msgConflictResolved)
	}
	var payload model.ConflictResolvedPayload
	decodePayload(t, msg, &payload)
	if payload.ConflictID != "conflict-1" || payload.ResolvedByUserID != "user-1" || payload.ResolutionOpID != "op-2" {
		t.Fatalf("payload = %#v, want conflict-1 resolved by user-1 with op-2", payload)
	}
}

func TestConflictMessagesRejectReader(t *testing.T) {
	service := testService()
	conn, err := service.OpenConnection(context.Background(), model.OpenRealtimeSessionRequest{
		SketchID: "sketch-id",
		UserID:   "user-2",
	})
	if err != nil {
		t.Fatalf("OpenConnection returned error: %v", err)
	}

	err = conn.HandleClientMessage(context.Background(), model.ClientRealtimeMessage{
		Type:      msgConflictResolved,
		RequestID: "req-conflict",
		SketchID:  "sketch-id",
		Payload:   json.RawMessage(`{"conflictId":"conflict-1"}`),
	})
	if err == nil {
		t.Fatal("conflict.resolved returned nil error for reader")
	}
	msg := nextMessage(t, conn)
	if msg.Type != msgError {
		t.Fatalf("message type = %q, want %q", msg.Type, msgError)
	}
}

func TestStateSnapshotBroadcastsByAdmin(t *testing.T) {
	service := testService()
	ctx := context.Background()

	admin, err := service.OpenConnection(ctx, model.OpenRealtimeSessionRequest{
		SketchID: "sketch-id",
		UserID:   "user-admin",
	})
	if err != nil {
		t.Fatalf("OpenConnection admin returned error: %v", err)
	}
	err = admin.HandleClientMessage(ctx, joinMessage("req-admin", "client-admin", 7))
	if err != nil {
		t.Fatalf("admin join returned error: %v", err)
	}
	_ = nextMessage(t, admin)

	editor, err := service.OpenConnection(ctx, model.OpenRealtimeSessionRequest{
		SketchID: "sketch-id",
		UserID:   "user-1",
	})
	if err != nil {
		t.Fatalf("OpenConnection editor returned error: %v", err)
	}
	err = editor.HandleClientMessage(ctx, joinMessage("req-editor", "client-editor", 7))
	if err != nil {
		t.Fatalf("editor join returned error: %v", err)
	}
	_ = nextMessage(t, editor)
	_ = nextMessage(t, admin)

	err = admin.HandleClientMessage(ctx, model.ClientRealtimeMessage{
		Type:      msgStateSnapshot,
		RequestID: "req-state",
		SketchID:  "sketch-id",
		Payload:   json.RawMessage(`{"version":9,"document":{"entities":{},"constraints":{},"dimensions":{}}}`),
	})
	if err != nil {
		t.Fatalf("state.snapshot returned error: %v", err)
	}

	for _, conn := range []*Connection{admin, editor} {
		msg := nextMessage(t, conn)
		if msg.Type != msgStateSnapshot || msg.RequestID != "req-state" {
			t.Fatalf("message = %#v, want state.snapshot req-state", msg)
		}
		var payload model.StateSnapshotPayload
		decodePayload(t, msg, &payload)
		if payload.Version != 9 || len(payload.Document) == 0 {
			t.Fatalf("payload = %#v, want version 9 with document", payload)
		}
	}
	if admin.currentVersion != 9 {
		t.Fatalf("currentVersion = %d, want 9", admin.currentVersion)
	}
}

func TestStatePatchAndResyncBroadcastByAdmin(t *testing.T) {
	service := testService()
	conn, err := service.OpenConnection(context.Background(), model.OpenRealtimeSessionRequest{
		SketchID: "sketch-id",
		UserID:   "user-admin",
	})
	if err != nil {
		t.Fatalf("OpenConnection returned error: %v", err)
	}
	err = conn.HandleClientMessage(context.Background(), joinMessage("req-join", "client-admin", 7))
	if err != nil {
		t.Fatalf("join returned error: %v", err)
	}
	_ = nextMessage(t, conn)

	err = conn.HandleClientMessage(context.Background(), model.ClientRealtimeMessage{
		Type:      msgStatePatch,
		RequestID: "req-patch",
		SketchID:  "sketch-id",
		Payload:   json.RawMessage(`{"version":8,"patch":{"entities":{"line-1":{}}}}`),
	})
	if err != nil {
		t.Fatalf("state.patch returned error: %v", err)
	}
	patch := nextMessage(t, conn)
	if patch.Type != msgStatePatch {
		t.Fatalf("message type = %q, want %q", patch.Type, msgStatePatch)
	}

	err = conn.HandleClientMessage(context.Background(), model.ClientRealtimeMessage{
		Type:      msgStateResyncRequired,
		RequestID: "req-resync",
		SketchID:  "sketch-id",
		Payload:   json.RawMessage(`{"currentVersion":10,"reason":"missed_events","recommendedAction":"fetch_ops"}`),
	})
	if err != nil {
		t.Fatalf("state.resync_required returned error: %v", err)
	}
	resync := nextMessage(t, conn)
	if resync.Type != msgStateResyncRequired {
		t.Fatalf("message type = %q, want %q", resync.Type, msgStateResyncRequired)
	}
	var payload model.StateResyncRequiredPayload
	decodePayload(t, resync, &payload)
	if payload.CurrentVersion != 10 || payload.RecommendedAction != "fetch_ops" {
		t.Fatalf("payload = %#v, want version 10 fetch_ops", payload)
	}
	if conn.currentVersion != 10 {
		t.Fatalf("currentVersion = %d, want 10", conn.currentVersion)
	}
}

func TestStateMessagesRejectNonAdmin(t *testing.T) {
	service := testService()
	conn, err := service.OpenConnection(context.Background(), model.OpenRealtimeSessionRequest{
		SketchID: "sketch-id",
		UserID:   "user-1",
	})
	if err != nil {
		t.Fatalf("OpenConnection returned error: %v", err)
	}

	err = conn.HandleClientMessage(context.Background(), model.ClientRealtimeMessage{
		Type:      msgStatePatch,
		RequestID: "req-state",
		SketchID:  "sketch-id",
		Payload:   json.RawMessage(`{"version":8,"patch":{"entities":{}}}`),
	})
	if err == nil {
		t.Fatal("state.patch returned nil error for non-admin")
	}
	msg := nextMessage(t, conn)
	if msg.Type != msgError {
		t.Fatalf("message type = %q, want %q", msg.Type, msgError)
	}
}

func TestCloseBroadcastsUserLeft(t *testing.T) {
	service := testService()
	ctx := context.Background()

	first, err := service.OpenConnection(ctx, model.OpenRealtimeSessionRequest{
		SketchID: "sketch-id",
		UserID:   "user-1",
	})
	if err != nil {
		t.Fatalf("OpenConnection first returned error: %v", err)
	}
	err = first.HandleClientMessage(ctx, joinMessage("req-1", "client-1", 7))
	if err != nil {
		t.Fatalf("first join returned error: %v", err)
	}
	_ = nextMessage(t, first)

	second, err := service.OpenConnection(ctx, model.OpenRealtimeSessionRequest{
		SketchID: "sketch-id",
		UserID:   "user-2",
	})
	if err != nil {
		t.Fatalf("OpenConnection second returned error: %v", err)
	}
	err = second.HandleClientMessage(ctx, joinMessage("req-2", "client-2", 7))
	if err != nil {
		t.Fatalf("second join returned error: %v", err)
	}
	_ = nextMessage(t, second)
	_ = nextMessage(t, first)

	err = second.Close(ctx, "transport_closed")
	if err != nil {
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
	return testServiceWithOperations(&operationsStub{})
}

func testServiceWithOperations(operations Operations) *Service {
	return newTestService(testPermissions(), operations)
}

func testPermissions() *permissionsStub {
	return &permissionsStub{permissions: []model.Permission{
		{SketchID: "sketch-id", UserID: "user-1", Role: "editor"},
		{SketchID: "sketch-id", UserID: "user-2", Role: "reader"},
		{SketchID: "sketch-id", UserID: "user-3", Role: "editor"},
		{SketchID: "sketch-id", UserID: "user-admin", Role: "admin"},
	}}
}

func newTestService(permissions *permissionsStub, operations Operations) *Service {
	return NewService(
		permissions,
		sketchesStub{document: &model.SketchDocument{ID: "sketch-id", Version: 7}},
		&locksStub{},
		operations,
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

func optionalTestString(value string) *string {
	return &value
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

func assertNoMessage(t *testing.T, conn *Connection) {
	t.Helper()
	select {
	case msg := <-conn.Outbound():
		t.Fatalf("unexpected outbound message: %#v", msg)
	default:
	}
}

func decodePayload(t *testing.T, msg model.ServerRealtimeMessage, out any) {
	t.Helper()
	if err := json.Unmarshal(msg.Payload, out); err != nil {
		t.Fatalf("decode payload: %v", err)
	}
}
