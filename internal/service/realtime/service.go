package realtime

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/dnonakolesax/cccad-locks/internal/auth"
	"github.com/dnonakolesax/cccad-locks/internal/model"
	"github.com/mailru/easyjson"
)

const protocolVersion = 1

const (
	roleReader = "reader"
	roleEditor = "editor"
	roleAdmin  = "admin"
)

const (
	msgSessionJoin          = "session.join"
	msgSessionJoined        = "session.joined"
	msgSessionUserJoined    = "session.user_joined"
	msgSessionUserLeft      = "session.user_left"
	msgSessionPing          = "session.ping"
	msgSessionPong          = "session.pong"
	msgSessionAccessRevoked = "session.access_revoked"

	msgPresenceCursor    = "presence.cursor"
	msgPresenceSelection = "presence.selection"
	msgPresenceHover     = "presence.hover"
	msgPresenceTool      = "presence.tool"

	msgIntentDraftBegin  = "intent.draft.begin"
	msgIntentDraftUpdate = "intent.draft.update"
	msgIntentDraftCancel = "intent.draft.cancel"
	msgIntentDraftEnded  = "intent.draft.ended"

	msgDragBegin         = "drag.begin"
	msgDragBeginAccepted = "drag.begin.accepted"
	msgDragBeginRejected = "drag.begin.rejected"
	msgDragPreview       = "drag.preview"
	msgDragCommit        = "drag.commit"
	msgDragCancel        = "drag.cancel"
	msgDragCancelled     = "drag.cancelled"

	msgOpSubmit    = "op.submit"
	msgOpCommitted = "op.committed"
	msgOpRejected  = "op.rejected"

	msgSyncResume       = "sync.resume"
	msgSyncResumeResult = "sync.resume.result"

	msgLockAcquire   = "lock.acquire"
	msgLockAcquired  = "lock.acquired"
	msgLockRejected  = "lock.rejected"
	msgLockRefresh   = "lock.refresh"
	msgLockRefreshed = "lock.refreshed"
	msgLockRelease   = "lock.release"
	msgLockReleased  = "lock.released"

	msgPermissionUpdated = "permission.updated"
	msgPermissionRevoked = "permission.revoked"

	msgConflictCreated  = "conflict.created"
	msgConflictResolved = "conflict.resolved"

	msgStateResyncRequired = "state.resync_required"
	msgStateSnapshot       = "state.snapshot"
	msgStatePatch          = "state.patch"

	msgError = "error"
)

const (
	outboundBufferSize = 16
	syncResumeOpsLimit = 1000

	uuidVersionMask    = 0x40
	uuidVariantMask    = 0x80
	uuidVersionBitmask = 0x0f
	uuidVariantBitmask = 0x3f
)

const (
	closeReasonDisconnect          = "disconnect"
	closeReasonAccessRevoked       = "access_revoked"
	closeReasonServerShutdown      = "server_shutdown"
	closeReasonDuplicateConnection = "duplicate_connection"
	closeReasonProtocolError       = "protocol_error"
)

const reasonPermissionDenied = "permission_denied"
const reasonStaleBaseVersion = "stale_base_version"

var errPermissionDenied = errors.New("permission denied")

type Permissions interface {
	List(ctx context.Context, sketchID string) ([]model.Permission, error)
	Put(ctx context.Context, permission *model.Permission) (*model.Permission, error)
	Delete(ctx context.Context, userID, sketchID string) error
}

type Sketches interface {
	Get(ctx context.Context, sketchID string) (*model.SketchDocument, error)
}

type Locks interface {
	Acquire(ctx context.Context, sketchID string, request *model.AcquireLockRequest) (*model.AcquireLockResponse, error)
	Refresh(ctx context.Context, sketchID, lockID string, request *model.RefreshLockRequest) (*model.SketchLock, error)
	Release(ctx context.Context, sketchID, lockID string) error
}

type Operations interface {
	List(
		ctx context.Context,
		sketchID string,
		afterVersion int64,
		limit int,
	) (*model.SketchOperationPage, error)
	Submit(
		ctx context.Context,
		sketchID string,
		request *model.SubmitOperationRequest,
	) (*model.SubmitOperationResponse, error)
}

type Service struct {
	permissions Permissions
	sketches    Sketches
	locks       Locks
	operations  Operations

	mu          sync.Mutex
	connections map[string]map[string]*Connection
}

func NewService(permissions Permissions, sketches Sketches, locks Locks, operations Operations) *Service {
	return &Service{
		permissions: permissions,
		sketches:    sketches,
		locks:       locks,
		operations:  operations,
		connections: make(map[string]map[string]*Connection),
	}
}

func (s *Service) OpenConnection(
	ctx context.Context,
	req model.OpenRealtimeSessionRequest,
) (*Connection, error) {
	req.SketchID = strings.TrimSpace(req.SketchID)
	req.UserID = strings.TrimSpace(req.UserID)
	if req.SketchID == "" || req.UserID == "" {
		return nil, errors.New("sketchID and userID are required")
	}
	if s.permissions == nil || s.sketches == nil {
		return nil, errors.New("realtime service dependencies are required")
	}

	role, err := s.userRole(ctx, req.SketchID, req.UserID)
	if err != nil {
		return nil, err
	}
	document, err := s.sketches.Get(ctx, req.SketchID)
	if err != nil {
		return nil, fmt.Errorf("get sketch for realtime session: %w", err)
	}
	if document == nil {
		return nil, errors.New("sketches service returned nil document")
	}

	return newConnection(s, req, role, document.Version), nil
}

func (s *Service) userRole(ctx context.Context, sketchID, userID string) (string, error) {
	permissions, err := s.permissions.List(ctx, sketchID)
	if err != nil {
		return "", fmt.Errorf("list sketch permissions: %w", err)
	}
	for _, permission := range permissions {
		if permission.UserID == userID && canRead(permission.Role) {
			return permission.Role, nil
		}
	}

	return "", errPermissionDenied
}

func canRead(role string) bool {
	switch role {
	case roleReader, roleEditor, roleAdmin:
		return true
	default:
		return false
	}
}

type Connection struct {
	service        *Service
	id             string
	sketchID       string
	userID         string
	displayName    string
	clientID       string
	role           string
	currentVersion int64
	joined         bool
	outbound       chan model.ServerRealtimeMessage
	closeOnce      sync.Once
}

func newConnection(
	service *Service,
	req model.OpenRealtimeSessionRequest,
	role string,
	currentVersion int64,
) *Connection {
	return &Connection{
		service:        service,
		id:             newID(),
		sketchID:       req.SketchID,
		userID:         req.UserID,
		displayName:    req.UserName,
		clientID:       req.ClientID,
		role:           role,
		currentVersion: currentVersion,
		outbound:       make(chan model.ServerRealtimeMessage, outboundBufferSize),
	}
}

func (c *Connection) ID() string {
	return c.id
}

func (c *Connection) SketchID() string {
	return c.sketchID
}

func (c *Connection) UserID() string {
	return c.userID
}

func (c *Connection) HandleClientMessage(ctx context.Context, msg model.ClientRealtimeMessage) error {
	if err := c.handleClientMessage(ctx, msg); err != nil {
		return err
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
}

func (c *Connection) handleClientMessage(ctx context.Context, msg model.ClientRealtimeMessage) error {
	switch msg.Type {
	case msgSessionJoin:
		return c.handleInvalidMessage(msg.RequestID, c.handleSessionJoin(msg))
	case msgSessionPing:
		return c.handleInvalidMessage(msg.RequestID, c.handleSessionPing(msg))
	case msgPresenceCursor, msgPresenceSelection, msgPresenceHover, msgPresenceTool:
		return c.handleInvalidMessage(msg.RequestID, c.handlePresence(msg))
	case msgIntentDraftBegin, msgIntentDraftUpdate:
		return c.handleInvalidMessage(msg.RequestID, c.handleIntentDraft(msg))
	case msgIntentDraftCancel:
		return c.handleInvalidMessage(msg.RequestID, c.handleIntentDraftCancel(msg))
	case msgDragBegin:
		return c.handleInvalidMessage(msg.RequestID, c.handleDragBegin(ctx, msg))
	case msgDragPreview:
		return c.handleInvalidMessage(msg.RequestID, c.handleDragPreview(msg))
	case msgDragCommit:
		return c.handleInvalidOperation(msg.RequestID, c.handleDragCommit(ctx, msg))
	case msgDragCancel:
		return c.handleLockMessageError(msg.RequestID, c.handleDragCancel(ctx, msg))
	case msgLockAcquire:
		return c.handleLockMessageError(msg.RequestID, c.handleLockAcquire(ctx, msg))
	case msgLockRefresh:
		return c.handleLockMessageError(msg.RequestID, c.handleLockRefresh(ctx, msg))
	case msgLockRelease:
		return c.handleLockMessageError(msg.RequestID, c.handleLockRelease(ctx, msg))
	case msgOpSubmit:
		return c.handleInvalidOperation(msg.RequestID, c.handleOpSubmit(ctx, msg))
	case msgSyncResume:
		return c.handleInvalidOperation(msg.RequestID, c.handleSyncResume(ctx, msg))
	case msgPermissionUpdated:
		return c.handleInvalidOperation(msg.RequestID, c.handlePermissionUpdated(ctx, msg))
	case msgPermissionRevoked:
		return c.handleInvalidOperation(msg.RequestID, c.handlePermissionRevoked(ctx, msg))
	case msgConflictCreated:
		return c.handleInvalidOperation(msg.RequestID, c.handleConflictCreated(msg))
	case msgConflictResolved:
		return c.handleInvalidOperation(msg.RequestID, c.handleConflictResolved(msg))
	case msgStateResyncRequired:
		return c.handleInvalidOperation(msg.RequestID, c.handleStateResyncRequired(msg))
	case msgStateSnapshot:
		return c.handleInvalidOperation(msg.RequestID, c.handleStateSnapshot(msg))
	case msgStatePatch:
		return c.handleInvalidOperation(msg.RequestID, c.handleStatePatch(msg))
	default:
		c.sendError(msg.RequestID, "CONSTRAINT_NOT_SUPPORTED", "realtime message type is not implemented")
		return nil
	}
}

func (c *Connection) handleInvalidMessage(requestID string, err error) error {
	if err != nil {
		c.sendError(requestID, "INVALID_MESSAGE", err.Error())
		return err
	}
	return nil
}

func (c *Connection) handleInvalidOperation(requestID string, err error) error {
	if err != nil {
		c.sendError(requestID, "INVALID_OPERATION", err.Error())
		return err
	}
	return nil
}

func (c *Connection) handleLockMessageError(requestID string, err error) error {
	if err != nil {
		c.sendLockError(requestID, err)
		return err
	}
	return nil
}

func (c *Connection) handleSessionJoin(msg model.ClientRealtimeMessage) error {
	var payload model.SessionJoinPayload
	if len(msg.Payload) == 0 {
		return errors.New("session.join payload is required")
	}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return fmt.Errorf("decode session.join: %w", err)
	}
	if payload.LastSeenVersion < 0 {
		return errors.New("lastSeenVersion must be greater than or equal to 0")
	}
	payload.ClientID = strings.TrimSpace(payload.ClientID)
	if payload.ClientID == "" {
		return errors.New("clientId is required")
	}
	if payload.SupportedProtocolVersion < 1 {
		return errors.New("supportedProtocolVersion must be greater than or equal to 1")
	}
	if payload.SupportedProtocolVersion > protocolVersion {
		return fmt.Errorf("unsupported protocol version %d", payload.SupportedProtocolVersion)
	}

	c.clientID = payload.ClientID

	activeUsers, firstJoin := c.service.join(c)
	c.send(msgSessionJoined, msg.RequestID, model.SessionJoinedPayload{
		ProtocolVersion:     protocolVersion,
		CurrentVersion:      c.currentVersion,
		User:                c.presence(),
		ActiveUsers:         activeUsers,
		MissingOpsAvailable: payload.LastSeenVersion < c.currentVersion,
	})
	if firstJoin {
		c.service.broadcastExcept(c.sketchID, c.id, model.ServerRealtimeMessage{
			Type:       msgSessionUserJoined,
			EventID:    newID(),
			SketchID:   c.sketchID,
			ServerTime: time.Now().UTC().Format(time.RFC3339Nano),
			Payload:    mustJSON(c.presence()),
		})
	}

	return nil
}

func (c *Connection) handleDragBegin(ctx context.Context, msg model.ClientRealtimeMessage) error {
	if !c.canEdit() {
		c.send(msgDragBeginRejected, msg.RequestID, model.DragBeginRejectedPayload{
			Reason: reasonPermissionDenied,
		})
		return nil
	}
	if c.service.locks == nil {
		return errors.New("locks service dependency is required")
	}

	var payload model.DragBeginPayload
	if len(msg.Payload) == 0 {
		return errors.New("drag.begin payload is required")
	}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return fmt.Errorf("decode drag.begin: %w", err)
	}
	payload.EntityID = strings.TrimSpace(payload.EntityID)
	payload.Kind = strings.TrimSpace(payload.Kind)
	if payload.EntityID == "" {
		c.send(msgDragBeginRejected, msg.RequestID, model.DragBeginRejectedPayload{Reason: "invalid_reference"})
		return nil
	}
	if payload.Kind == "" {
		return errors.New("kind is required")
	}
	if payload.BaseVersion < 0 {
		c.send(msgDragBeginRejected, msg.RequestID, model.DragBeginRejectedPayload{Reason: reasonStaleBaseVersion})
		return nil
	}

	scope := model.LockScopePayload{Type: "entity", EntityID: payload.EntityID}
	scopeBody, err := json.Marshal(scope)
	if err != nil {
		return fmt.Errorf("encode drag lock scope: %w", err)
	}

	response, err := c.service.locks.Acquire(
		auth.ContextWithUserID(ctx, c.userID),
		c.sketchID,
		&model.AcquireLockRequest{
			Scope: easyjson.RawMessage(scopeBody),
			Mode:  "edit",
		},
	)
	if err != nil {
		return err
	}
	if response == nil {
		return errors.New("locks service returned nil acquire response")
	}
	if !response.Granted {
		rejected := model.DragBeginRejectedPayload{Reason: "lock_conflict"}
		if response.Conflict != nil {
			rejected.LockedByUserID = response.Conflict.HolderUserID
			rejected.LockID = response.Conflict.LockID
		}
		c.send(msgDragBeginRejected, msg.RequestID, rejected)
		return nil
	}
	if response.Lock == nil {
		return errors.New("locks service returned nil acquired lock")
	}

	acquired, err := lockAcquiredPayload(response.Lock)
	if err != nil {
		return err
	}
	c.send(msgDragBeginAccepted, msg.RequestID, model.DragBeginAcceptedPayload{
		LockID:          acquired.LockID,
		LockedEntityIDs: acquired.EntityIDs,
		ComponentID:     acquired.Scope.ComponentID,
	})
	return nil
}

func (c *Connection) handleDragPreview(msg model.ClientRealtimeMessage) error {
	if !c.joined {
		return errors.New("session.join is required before drag.preview")
	}
	if !c.canEdit() {
		return errPermissionDenied
	}
	if len(msg.Payload) == 0 {
		return errors.New("drag.preview payload is required")
	}

	payload, err := dragPreviewPayload(msg.Payload)
	if err != nil {
		return err
	}
	payload["userId"] = c.userID
	payload["clientId"] = c.clientID

	c.service.broadcastExcept(c.sketchID, c.id, c.serverMessage(msgDragPreview, msg.RequestID, payload))
	return nil
}

func (c *Connection) handleDragCommit(ctx context.Context, msg model.ClientRealtimeMessage) error {
	var payload model.DragCommitPayload
	if len(msg.Payload) == 0 {
		return errors.New("drag.commit payload is required")
	}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return fmt.Errorf("decode drag.commit: %w", err)
	}

	opPayload := model.OpSubmitPayload{
		BaseVersion: payload.BaseVersion,
		ClientOpID:  payload.ClientOpID,
		Op:          payload.Op,
	}
	lockID := strings.TrimSpace(payload.LockID)
	return c.submitOperation(ctx, msg.RequestID, opPayload, lockID)
}

func (c *Connection) handleDragCancel(ctx context.Context, msg model.ClientRealtimeMessage) error {
	if !c.canEdit() {
		return errPermissionDenied
	}

	var payload model.DragCancelPayload
	if len(msg.Payload) == 0 {
		return errors.New("drag.cancel payload is required")
	}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return fmt.Errorf("decode drag.cancel: %w", err)
	}
	payload.EntityID = strings.TrimSpace(payload.EntityID)
	payload.LockID = strings.TrimSpace(payload.LockID)
	if payload.EntityID == "" {
		return errors.New("entityId is required")
	}

	if payload.LockID != "" {
		if c.service.locks == nil {
			return errors.New("locks service dependency is required")
		}
		if err := c.service.locks.Release(
			auth.ContextWithUserID(ctx, c.userID),
			c.sketchID,
			payload.LockID,
		); err != nil {
			return err
		}
	}

	c.service.broadcastExcept(c.sketchID, c.id, c.serverMessage(msgDragCancelled, msg.RequestID, map[string]any{
		"userId":   c.userID,
		"clientId": c.clientID,
		"entityId": payload.EntityID,
		"lockId":   payload.LockID,
	}))
	c.send(msgDragCancelled, msg.RequestID, map[string]any{
		"userId":   c.userID,
		"clientId": c.clientID,
		"entityId": payload.EntityID,
		"lockId":   payload.LockID,
	})
	return nil
}

func (c *Connection) handleSessionPing(msg model.ClientRealtimeMessage) error {
	var payload model.SessionPingPayload
	if len(msg.Payload) == 0 {
		return errors.New("session.ping payload is required")
	}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return fmt.Errorf("decode session.ping: %w", err)
	}
	if payload.ClientVersion < 0 {
		return errors.New("clientVersion must be greater than or equal to 0")
	}

	c.send(msgSessionPong, msg.RequestID, model.SessionPongPayload{CurrentVersion: c.currentVersion})
	return nil
}

func (c *Connection) handlePresence(msg model.ClientRealtimeMessage) error {
	if !c.joined {
		return errors.New("session.join is required before presence messages")
	}
	if len(msg.Payload) == 0 {
		return fmt.Errorf("%s payload is required", msg.Type)
	}

	payload, err := presencePayload(msg.Type, msg.Payload)
	if err != nil {
		return err
	}
	payload["userId"] = c.userID
	payload["clientId"] = c.clientID

	c.service.broadcastExcept(c.sketchID, c.id, c.serverMessage(msg.Type, msg.RequestID, payload))
	return nil
}

func (c *Connection) handleIntentDraft(msg model.ClientRealtimeMessage) error {
	if !c.joined {
		return fmt.Errorf("session.join is required before %s", msg.Type)
	}
	if !c.canEdit() {
		return errPermissionDenied
	}

	var payload model.IntentDraftPayload
	if len(msg.Payload) == 0 {
		return fmt.Errorf("%s payload is required", msg.Type)
	}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return fmt.Errorf("decode %s: %w", msg.Type, err)
	}
	payload.DraftID = strings.TrimSpace(payload.DraftID)
	payload.ActorUserID = strings.TrimSpace(payload.ActorUserID)
	payload.ClientID = strings.TrimSpace(payload.ClientID)
	payload.Tool = strings.TrimSpace(payload.Tool)
	payload.OperationType = strings.TrimSpace(payload.OperationType)
	payload.Phase = strings.TrimSpace(payload.Phase)
	payload.StyleHint = strings.TrimSpace(payload.StyleHint)
	if payload.DraftID == "" {
		return errors.New("draftId is required")
	}
	if payload.BaseVersion < 0 {
		return errors.New("baseVersion must be greater than or equal to 0")
	}
	if payload.Tool == "" {
		return errors.New("tool is required")
	}
	if payload.Phase == "" {
		return errors.New("phase is required")
	}
	if payload.ActorUserID == "" {
		payload.ActorUserID = c.userID
	}
	if payload.ActorUserID != c.userID {
		return errors.New("actorUserId must match authenticated user")
	}
	if payload.ClientID == "" {
		payload.ClientID = c.clientID
	}

	c.service.broadcastExcept(c.sketchID, c.id, c.serverMessage(msg.Type, msg.RequestID, payload))
	return nil
}

func (c *Connection) handleIntentDraftCancel(msg model.ClientRealtimeMessage) error {
	if !c.joined {
		return errors.New("session.join is required before intent.draft.cancel")
	}
	if !c.canEdit() {
		return errPermissionDenied
	}

	var payload model.IntentDraftCancelPayload
	if len(msg.Payload) == 0 {
		return errors.New("intent.draft.cancel payload is required")
	}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return fmt.Errorf("decode intent.draft.cancel: %w", err)
	}
	payload.DraftID = strings.TrimSpace(payload.DraftID)
	payload.ActorUserID = strings.TrimSpace(payload.ActorUserID)
	payload.ClientID = strings.TrimSpace(payload.ClientID)
	payload.Reason = strings.TrimSpace(payload.Reason)
	if payload.DraftID == "" {
		return errors.New("draftId is required")
	}
	if payload.ActorUserID == "" {
		payload.ActorUserID = c.userID
	}
	if payload.ActorUserID != c.userID {
		return errors.New("actorUserId must match authenticated user")
	}
	if payload.ClientID == "" {
		payload.ClientID = c.clientID
	}
	if payload.Reason == "" {
		payload.Reason = "user_cancelled"
	}

	c.service.broadcastExcept(c.sketchID, c.id, c.serverMessage(msgIntentDraftCancel, msg.RequestID, payload))
	c.service.broadcastExcept(c.sketchID, c.id, c.serverMessage(msgIntentDraftEnded, msg.RequestID, model.IntentDraftEndedPayload{
		DraftID:     payload.DraftID,
		ActorUserID: payload.ActorUserID,
		ClientID:    payload.ClientID,
		Reason:      "cancelled",
	}))
	return nil
}

func (c *Connection) handleLockAcquire(ctx context.Context, msg model.ClientRealtimeMessage) error {
	if !c.canEdit() {
		c.send(msgLockRejected, msg.RequestID, model.LockRejectedPayload{Reason: reasonPermissionDenied})
		return nil
	}
	if c.service.locks == nil {
		return errors.New("locks service dependency is required")
	}

	var payload model.LockAcquirePayload
	if len(msg.Payload) == 0 {
		return errors.New("lock.acquire payload is required")
	}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return fmt.Errorf("decode lock.acquire: %w", err)
	}
	scope, ok := normalizeLockScope(payload.Scope)
	if !ok {
		c.send(msgLockRejected, msg.RequestID, model.LockRejectedPayload{Reason: "invalid_scope"})
		return nil
	}
	scopeBody, err := json.Marshal(scope)
	if err != nil {
		return fmt.Errorf("encode lock scope: %w", err)
	}

	response, err := c.service.locks.Acquire(
		auth.ContextWithUserID(ctx, c.userID),
		c.sketchID,
		&model.AcquireLockRequest{
			Scope: easyjson.RawMessage(scopeBody),
			Mode:  strings.TrimSpace(payload.Mode),
			TTLMS: int(payload.TTLMS),
		},
	)
	if err != nil {
		return err
	}
	if response == nil {
		return errors.New("locks service returned nil acquire response")
	}
	if !response.Granted {
		rejected := model.LockRejectedPayload{Reason: "already_locked"}
		if response.Conflict != nil {
			rejected.LockedByUserID = response.Conflict.HolderUserID
			rejected.LockID = response.Conflict.LockID
		}
		c.send(msgLockRejected, msg.RequestID, rejected)
		return nil
	}
	if response.Lock == nil {
		return errors.New("locks service returned nil acquired lock")
	}

	acquired, err := lockAcquiredPayload(response.Lock)
	if err != nil {
		return err
	}
	c.send(msgLockAcquired, msg.RequestID, acquired)
	return nil
}

func (c *Connection) handleLockRefresh(ctx context.Context, msg model.ClientRealtimeMessage) error {
	if !c.canEdit() {
		return errPermissionDenied
	}
	if c.service.locks == nil {
		return errors.New("locks service dependency is required")
	}

	var payload model.LockRefreshPayload
	if len(msg.Payload) == 0 {
		return errors.New("lock.refresh payload is required")
	}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return fmt.Errorf("decode lock.refresh: %w", err)
	}
	payload.LockID = strings.TrimSpace(payload.LockID)
	if payload.LockID == "" {
		return errors.New("lockId is required")
	}

	lock, err := c.service.locks.Refresh(
		auth.ContextWithUserID(ctx, c.userID),
		c.sketchID,
		payload.LockID,
		&model.RefreshLockRequest{TTLMS: int(payload.TTLMS)},
	)
	if err != nil {
		return err
	}
	if lock == nil {
		return errors.New("locks service returned nil refreshed lock")
	}

	c.send(msgLockRefreshed, msg.RequestID, model.LockRefreshedPayload{
		LockID:    lock.ID,
		ExpiresAt: lock.ExpiresAt,
	})
	return nil
}

func (c *Connection) handleLockRelease(ctx context.Context, msg model.ClientRealtimeMessage) error {
	if !c.canEdit() {
		return errPermissionDenied
	}
	if c.service.locks == nil {
		return errors.New("locks service dependency is required")
	}

	var payload model.LockReleasePayload
	if len(msg.Payload) == 0 {
		return errors.New("lock.release payload is required")
	}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return fmt.Errorf("decode lock.release: %w", err)
	}
	payload.LockID = strings.TrimSpace(payload.LockID)
	if payload.LockID == "" {
		return errors.New("lockId is required")
	}

	if err := c.service.locks.Release(auth.ContextWithUserID(ctx, c.userID), c.sketchID, payload.LockID); err != nil {
		return err
	}

	c.send(msgLockReleased, msg.RequestID, model.LockReleasedPayload{
		LockID: payload.LockID,
		Reason: "released",
		UserID: c.userID,
	})
	return nil
}

func (c *Connection) handleOpSubmit(ctx context.Context, msg model.ClientRealtimeMessage) error {
	var payload model.OpSubmitPayload
	if len(msg.Payload) == 0 {
		return errors.New("op.submit payload is required")
	}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return fmt.Errorf("decode op.submit: %w", err)
	}
	return c.submitOperation(ctx, msg.RequestID, payload, "")
}

func (c *Connection) handleSyncResume(ctx context.Context, msg model.ClientRealtimeMessage) error {
	if !c.joined {
		return errors.New("session.join is required before sync.resume")
	}
	if !c.canEdit() {
		c.send(msgSyncResumeResult, msg.RequestID, model.SyncResumeResultPayload{
			Status:         "rejected",
			ClientID:       c.clientID,
			CurrentVersion: c.currentVersion,
			OpResults:      []model.OfflineOperationResult{},
			Message:        errPermissionDenied.Error(),
		})
		return nil
	}
	if c.service.operations == nil {
		return errors.New("operations service dependency is required")
	}

	var payload model.SyncResumePayload
	if len(msg.Payload) == 0 {
		return errors.New("sync.resume payload is required")
	}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return fmt.Errorf("decode sync.resume: %w", err)
	}
	payload.ClientID = strings.TrimSpace(payload.ClientID)
	if payload.ClientID == "" {
		return errors.New("clientId is required")
	}
	if payload.LastSeenVersion < 0 {
		return errors.New("lastSeenVersion must be greater than or equal to 0")
	}
	if payload.LastAckedClientSeq < 0 {
		return errors.New("lastAckedClientSeq must be greater than or equal to 0")
	}
	if payload.ClientID != c.clientID {
		return errors.New("clientId must match joined session")
	}

	pendingOps := append([]model.PendingOfflineOperation(nil), payload.PendingOps...)
	sort.SliceStable(pendingOps, func(i, j int) bool {
		return pendingOps[i].ClientSeq < pendingOps[j].ClientSeq
	})

	result := model.SyncResumeResultPayload{
		Status:         "ok",
		ClientID:       payload.ClientID,
		FromVersion:    payload.LastSeenVersion,
		CurrentVersion: c.currentVersion,
		MissedPatches:  []model.CommittedPatchPayload{},
		OpResults:      make([]model.OfflineOperationResult, 0, len(pendingOps)),
	}

	missed, err := c.service.operations.List(
		auth.ContextWithUserID(ctx, c.userID),
		c.sketchID,
		payload.LastSeenVersion,
		syncResumeOpsLimit,
	)
	if err != nil {
		return err
	}
	if missed == nil {
		return errors.New("operations service returned nil operation page")
	}
	result.MissedPatches = committedPatchPayloads(missed.Ops)
	if len(missed.Ops) == syncResumeOpsLimit && missed.ToVersion < c.currentVersion {
		result.Status = "snapshot_required"
		result.Message = "too many missed operations; fetch a fresh snapshot"
		c.send(msgSyncResumeResult, msg.RequestID, result)
		return nil
	}

	for _, pending := range pendingOps {
		opResult, err := c.submitOfflineOperation(ctx, msg.RequestID, pending)
		if err != nil {
			return err
		}
		result.OpResults = append(result.OpResults, opResult)
		result.CurrentVersion = c.currentVersion
		if opResult.Status == "rejected" {
			result.Status = "rejected"
		}
	}

	c.send(msgSyncResumeResult, msg.RequestID, result)
	return nil
}

func (c *Connection) submitOperation(
	ctx context.Context,
	requestID string,
	payload model.OpSubmitPayload,
	lockID string,
) error {
	if !c.canEdit() {
		c.send(msgOpRejected, requestID, model.OpRejectedPayload{
			ClientOpID:     strings.TrimSpace(payload.ClientOpID),
			CurrentVersion: c.currentVersion,
			Reason:         reasonPermissionDenied,
			Message:        errPermissionDenied.Error(),
		})
		return nil
	}
	if c.service.operations == nil {
		return errors.New("operations service dependency is required")
	}

	payload.ClientOpID = strings.TrimSpace(payload.ClientOpID)
	if payload.BaseVersion < 0 {
		return errors.New("baseVersion must be greater than or equal to 0")
	}
	if payload.ClientOpID == "" {
		return errors.New("clientOpId is required")
	}
	if len(payload.Op) == 0 {
		return errors.New("op is required")
	}
	lockID = strings.TrimSpace(lockID)

	response, err := c.service.operations.Submit(
		auth.ContextWithUserID(ctx, c.userID),
		c.sketchID,
		&model.SubmitOperationRequest{
			BaseVersion: payload.BaseVersion,
			ClientOpID:  payload.ClientOpID,
			LockID:      optionalString(lockID),
			Op:          easyjson.RawMessage(payload.Op),
		},
	)
	if err != nil {
		return err
	}
	if response == nil {
		return errors.New("operations service returned nil submit response")
	}
	if !response.Accepted {
		c.send(msgOpRejected, requestID, opRejectedPayload(payload.ClientOpID, response))
		return nil
	}

	committed, err := opCommittedPayload(c.userID, payload, response)
	if err != nil {
		return err
	}
	c.currentVersion = maxInt64(c.currentVersion, maxInt64(committed.Version, response.CurrentVersion))

	msgOut := c.serverMessage(msgOpCommitted, requestID, committed)
	c.enqueue(msgOut)
	c.service.broadcastExcept(c.sketchID, c.id, msgOut)
	return nil
}

func (c *Connection) submitOfflineOperation(
	ctx context.Context,
	requestID string,
	pending model.PendingOfflineOperation,
) (model.OfflineOperationResult, error) {
	pending.ClientOpID = strings.TrimSpace(pending.ClientOpID)
	if pending.ClientOpID == "" {
		return model.OfflineOperationResult{}, errors.New("pendingOps.clientOpId is required")
	}
	if pending.ClientSeq < 0 {
		return model.OfflineOperationResult{}, errors.New("pendingOps.clientSeq must be greater than or equal to 0")
	}
	if pending.BaseVersion < 0 {
		return model.OfflineOperationResult{}, errors.New("pendingOps.baseVersion must be greater than or equal to 0")
	}
	if len(pending.Op) == 0 {
		return model.OfflineOperationResult{}, errors.New("pendingOps.op is required")
	}

	result := model.OfflineOperationResult{
		ClientOpID:     pending.ClientOpID,
		ClientSeq:      pending.ClientSeq,
		CurrentVersion: c.currentVersion,
		Authoritative:  true,
	}

	response, err := c.service.operations.Submit(
		auth.ContextWithUserID(ctx, c.userID),
		c.sketchID,
		&model.SubmitOperationRequest{
			BaseVersion: c.currentVersion,
			ClientOpID:  pending.ClientOpID,
			Op:          easyjson.RawMessage(pending.Op),
		},
	)
	if err != nil {
		return model.OfflineOperationResult{}, err
	}
	if response == nil {
		return model.OfflineOperationResult{}, errors.New("operations service returned nil submit response")
	}
	if !response.Accepted {
		rejected := opRejectedPayload(pending.ClientOpID, response)
		result.Status = "rejected"
		result.CurrentVersion = rejected.CurrentVersion
		result.Reason = rejected.Reason
		result.Message = rejected.Message
		result.Diagnostics = rejected.Diagnostics
		c.currentVersion = maxInt64(c.currentVersion, response.CurrentVersion)
		return result, nil
	}

	committed, err := opCommittedPayload(c.userID, model.OpSubmitPayload{
		BaseVersion: c.currentVersion,
		ClientOpID:  pending.ClientOpID,
		Op:          pending.Op,
	}, response)
	if err != nil {
		return model.OfflineOperationResult{}, err
	}
	if response.Duplicate {
		result.Status = "duplicate_committed"
	} else {
		result.Status = "committed"
	}
	result.CommittedVersion = committed.Version
	result.CurrentVersion = maxInt64(committed.Version, response.CurrentVersion)
	result.OpID = committed.OpID
	result.Patch = committed.Patch
	result.SolveStatus = committed.SolveStatus
	result.AffectedEntityIDs = committed.AffectedEntityIDs
	result.AffectedConstraintIDs = committed.AffectedConstraintIDs
	result.AffectedDimensionIDs = committed.AffectedDimensionIDs
	result.AffectedComponentIDs = committed.AffectedComponentIDs

	c.currentVersion = maxInt64(c.currentVersion, result.CurrentVersion)
	c.service.broadcastExcept(c.sketchID, c.id, c.serverMessage(msgOpCommitted, requestID, committed))
	return result, nil
}

func (c *Connection) handlePermissionUpdated(ctx context.Context, msg model.ClientRealtimeMessage) error {
	if !c.canManagePermissions() {
		return errPermissionDenied
	}

	var payload model.PermissionUpdatedPayload
	if len(msg.Payload) == 0 {
		return errors.New("permission.updated payload is required")
	}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return fmt.Errorf("decode permission.updated: %w", err)
	}
	payload.TargetUserID = strings.TrimSpace(payload.TargetUserID)
	payload.Role = strings.TrimSpace(payload.Role)
	if payload.TargetUserID == "" {
		return errors.New("targetUserId is required")
	}
	if !isValidRole(payload.Role) {
		return errors.New("role must be reader, editor, or admin")
	}

	permission, err := c.service.permissions.Put(ctx, &model.Permission{
		SketchID:        c.sketchID,
		UserID:          payload.TargetUserID,
		Role:            payload.Role,
		GrantedByUserID: &c.userID,
	})
	if err != nil {
		return err
	}
	if permission == nil {
		return errors.New("permissions service returned nil permission")
	}

	c.broadcastToSketch(c.serverMessage(msgPermissionUpdated, msg.RequestID, model.PermissionUpdatedPayload{
		TargetUserID:    permission.UserID,
		Role:            permission.Role,
		ChangedByUserID: c.userID,
	}))
	return nil
}

func (c *Connection) handlePermissionRevoked(ctx context.Context, msg model.ClientRealtimeMessage) error {
	if !c.canManagePermissions() {
		return errPermissionDenied
	}

	var payload model.PermissionRevokedPayload
	if len(msg.Payload) == 0 {
		return errors.New("permission.revoked payload is required")
	}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return fmt.Errorf("decode permission.revoked: %w", err)
	}
	payload.TargetUserID = strings.TrimSpace(payload.TargetUserID)
	if payload.TargetUserID == "" {
		return errors.New("targetUserId is required")
	}
	if payload.TargetUserID == c.userID {
		return errors.New("cannot revoke own realtime permission")
	}

	if err := c.service.permissions.Delete(ctx, payload.TargetUserID, c.sketchID); err != nil {
		return err
	}

	c.broadcastToSketch(c.serverMessage(msgPermissionRevoked, msg.RequestID, model.PermissionRevokedPayload{
		TargetUserID:    payload.TargetUserID,
		ChangedByUserID: c.userID,
	}))
	c.service.closeUserConnections(
		ctx,
		c.sketchID,
		payload.TargetUserID,
		"Your access to this sketch was revoked.",
	)
	return nil
}

func (c *Connection) handleConflictCreated(msg model.ClientRealtimeMessage) error {
	if !c.canEdit() {
		return errPermissionDenied
	}

	var payload model.ConflictCreatedPayload
	if len(msg.Payload) == 0 {
		return errors.New("conflict.created payload is required")
	}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return fmt.Errorf("decode conflict.created: %w", err)
	}
	payload.ConflictID = strings.TrimSpace(payload.ConflictID)
	payload.ConflictType = strings.TrimSpace(payload.ConflictType)
	payload.Status = strings.TrimSpace(payload.Status)
	payload.Message = strings.TrimSpace(payload.Message)
	if payload.ConflictID == "" {
		return errors.New("conflictId is required")
	}
	if payload.ConflictType == "" {
		return errors.New("conflictType is required")
	}
	if !isValidConflictStatus(payload.Status) {
		return errors.New("status must be open, resolved, or ignored")
	}
	if len(payload.AffectedEntityIDs) == 0 {
		return errors.New("affectedEntityIds is required")
	}
	if len(payload.CausedByOps) == 0 {
		return errors.New("causedByOps is required")
	}
	if payload.Message == "" {
		return errors.New("message is required")
	}

	c.broadcastToSketch(c.serverMessage(msgConflictCreated, msg.RequestID, payload))
	return nil
}

func (c *Connection) handleConflictResolved(msg model.ClientRealtimeMessage) error {
	if !c.canEdit() {
		return errPermissionDenied
	}

	var payload model.ConflictResolvedPayload
	if len(msg.Payload) == 0 {
		return errors.New("conflict.resolved payload is required")
	}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return fmt.Errorf("decode conflict.resolved: %w", err)
	}
	payload.ConflictID = strings.TrimSpace(payload.ConflictID)
	payload.ResolvedByUserID = strings.TrimSpace(payload.ResolvedByUserID)
	payload.ResolutionOpID = strings.TrimSpace(payload.ResolutionOpID)
	if payload.ConflictID == "" {
		return errors.New("conflictId is required")
	}
	if payload.ResolvedByUserID == "" {
		payload.ResolvedByUserID = c.userID
	}

	c.broadcastToSketch(c.serverMessage(msgConflictResolved, msg.RequestID, payload))
	return nil
}

func (c *Connection) handleStateResyncRequired(msg model.ClientRealtimeMessage) error {
	if !c.canManagePermissions() {
		return errPermissionDenied
	}

	var payload model.StateResyncRequiredPayload
	if len(msg.Payload) == 0 {
		return errors.New("state.resync_required payload is required")
	}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return fmt.Errorf("decode state.resync_required: %w", err)
	}
	payload.Reason = strings.TrimSpace(payload.Reason)
	payload.RecommendedAction = strings.TrimSpace(payload.RecommendedAction)
	if payload.CurrentVersion < 0 {
		return errors.New("currentVersion must be greater than or equal to 0")
	}
	if !isValidStateResyncReason(payload.Reason) {
		return errors.New("reason must be client_too_far_behind, missed_events, server_restart, or protocol_error")
	}
	if !isValidStateResyncAction(payload.RecommendedAction) {
		return errors.New("recommendedAction must be fetch_snapshot, fetch_ops, or reconnect")
	}

	c.currentVersion = maxInt64(c.currentVersion, payload.CurrentVersion)
	c.broadcastToSketch(c.serverMessage(msgStateResyncRequired, msg.RequestID, payload))
	return nil
}

func (c *Connection) handleStateSnapshot(msg model.ClientRealtimeMessage) error {
	if !c.canManagePermissions() {
		return errPermissionDenied
	}

	var payload model.StateSnapshotPayload
	if len(msg.Payload) == 0 {
		return errors.New("state.snapshot payload is required")
	}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return fmt.Errorf("decode state.snapshot: %w", err)
	}
	if payload.Version < 0 {
		return errors.New("version must be greater than or equal to 0")
	}
	if !rawObject(payload.Document) {
		return errors.New("document is required")
	}

	c.currentVersion = maxInt64(c.currentVersion, payload.Version)
	c.broadcastToSketch(c.serverMessage(msgStateSnapshot, msg.RequestID, payload))
	return nil
}

func (c *Connection) handleStatePatch(msg model.ClientRealtimeMessage) error {
	if !c.canManagePermissions() {
		return errPermissionDenied
	}

	var payload model.StatePatchPayload
	if len(msg.Payload) == 0 {
		return errors.New("state.patch payload is required")
	}
	if err := json.Unmarshal(msg.Payload, &payload); err != nil {
		return fmt.Errorf("decode state.patch: %w", err)
	}
	if payload.Version < 0 {
		return errors.New("version must be greater than or equal to 0")
	}
	if !rawObject(payload.Patch) {
		return errors.New("patch is required")
	}

	c.currentVersion = maxInt64(c.currentVersion, payload.Version)
	c.broadcastToSketch(c.serverMessage(msgStatePatch, msg.RequestID, payload))
	return nil
}

func (c *Connection) Outbound() <-chan model.ServerRealtimeMessage {
	return c.outbound
}

func (c *Connection) Close(_ context.Context, reason string) error {
	c.closeOnce.Do(func() {
		c.service.leave(c, sessionCloseReason(reason))
		close(c.outbound)
	})
	return nil
}

func (c *Connection) SendAccessRevoked(message string) {
	if strings.TrimSpace(message) == "" {
		message = "Your access to this sketch was revoked."
	}
	c.send(msgSessionAccessRevoked, "", model.SessionAccessRevokedPayload{Message: message})
}

func (c *Connection) presence() model.UserPresenceSummary {
	return model.UserPresenceSummary{
		UserID:      c.userID,
		DisplayName: c.displayName,
		Role:        c.role,
		ClientID:    c.clientID,
	}
}

func (c *Connection) send(messageType, requestID string, payload any) {
	c.enqueue(c.serverMessage(messageType, requestID, payload))
}

func (c *Connection) serverMessage(messageType, requestID string, payload any) model.ServerRealtimeMessage {
	body, err := json.Marshal(payload)
	if err != nil {
		c.sendError(requestID, "INTERNAL_ERROR", "failed to encode realtime payload")
		return model.ServerRealtimeMessage{}
	}

	return model.ServerRealtimeMessage{
		Type:       messageType,
		EventID:    newID(),
		RequestID:  requestID,
		SketchID:   c.sketchID,
		ServerTime: time.Now().UTC().Format(time.RFC3339Nano),
		Payload:    body,
	}
}

func (c *Connection) broadcastToSketch(msg model.ServerRealtimeMessage) {
	c.service.broadcast(c.sketchID, msg)
}

func (c *Connection) sendError(requestID, code, message string) {
	body, _ := json.Marshal(model.RealtimeErrorPayload{
		Code:    code,
		Message: message,
	})
	c.enqueue(model.ServerRealtimeMessage{
		Type:       msgError,
		EventID:    newID(),
		RequestID:  requestID,
		SketchID:   c.sketchID,
		ServerTime: time.Now().UTC().Format(time.RFC3339Nano),
		Payload:    body,
	})
}

func (c *Connection) sendLockError(requestID string, err error) {
	switch {
	case errors.Is(err, model.ErrLockNotFound):
		c.sendError(requestID, "LOCK_NOT_FOUND", err.Error())
	case errors.Is(err, model.ErrLockNotOwned), errors.Is(err, errPermissionDenied):
		c.sendError(requestID, "PERMISSION_DENIED", err.Error())
	default:
		c.sendError(requestID, "INVALID_MESSAGE", err.Error())
	}
}

func (c *Connection) canEdit() bool {
	switch c.role {
	case roleEditor, roleAdmin:
		return true
	default:
		return false
	}
}

func (c *Connection) canManagePermissions() bool {
	return c.role == roleAdmin
}

func isValidRole(role string) bool {
	return canRead(role)
}

func isValidConflictStatus(status string) bool {
	switch status {
	case "open", "resolved", "ignored":
		return true
	default:
		return false
	}
}

func isValidStateResyncReason(reason string) bool {
	switch reason {
	case "client_too_far_behind", "missed_events", "server_restart", "protocol_error":
		return true
	default:
		return false
	}
}

func isValidStateResyncAction(action string) bool {
	switch action {
	case "fetch_snapshot", "fetch_ops", "reconnect":
		return true
	default:
		return false
	}
}

func presencePayload(messageType string, raw json.RawMessage) (map[string]any, error) {
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("decode %s: %w", messageType, err)
	}
	if payload == nil {
		return nil, fmt.Errorf("%s payload must be an object", messageType)
	}

	switch messageType {
	case msgPresenceCursor:
		if err := validateCursorPresence(payload); err != nil {
			return nil, err
		}
	case msgPresenceHover:
		kind := strings.TrimSpace(stringValue(payload["kind"]))
		id := strings.TrimSpace(stringValue(payload["id"]))
		if kind == "" || id == "" {
			return nil, errors.New("presence.hover requires kind and id")
		}
	case msgPresenceTool:
		if strings.TrimSpace(stringValue(payload["tool"])) == "" {
			return nil, errors.New("presence.tool requires tool")
		}
	case msgPresenceSelection:
		return payload, nil
	default:
		return nil, fmt.Errorf("unsupported presence message type %s", messageType)
	}

	return payload, nil
}

func validateCursorPresence(payload map[string]any) error {
	if _, ok := numericValue(payload["x"]); !ok {
		return errors.New("presence.cursor requires numeric x")
	}
	if _, ok := numericValue(payload["y"]); !ok {
		return errors.New("presence.cursor requires numeric y")
	}

	viewport, ok := payload["viewport"].(map[string]any)
	if !ok || viewport == nil {
		return nil
	}
	scale, ok := numericValue(viewport["scale"])
	if !ok {
		return errors.New("presence.cursor viewport requires numeric scale")
	}
	if scale <= 0 {
		return errors.New("presence.cursor viewport scale must be greater than 0")
	}
	if _, offsetXOK := numericValue(viewport["offsetX"]); !offsetXOK {
		return errors.New("presence.cursor viewport requires numeric offsetX")
	}
	if _, offsetYOK := numericValue(viewport["offsetY"]); !offsetYOK {
		return errors.New("presence.cursor viewport requires numeric offsetY")
	}

	return nil
}

func dragPreviewPayload(raw json.RawMessage) (map[string]any, error) {
	var payload map[string]any
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil, fmt.Errorf("decode drag.preview: %w", err)
	}
	if payload == nil {
		return nil, errors.New("drag.preview payload must be an object")
	}
	if strings.TrimSpace(stringValue(payload["entityId"])) == "" {
		return nil, errors.New("drag.preview requires entityId")
	}
	target, targetOK := payload["target"].(map[string]any)
	if !targetOK || target == nil {
		return nil, errors.New("drag.preview requires target")
	}
	if _, numericOK := numericValue(target["x"]); !numericOK {
		return nil, errors.New("drag.preview target requires numeric x")
	}
	if _, numericOK := numericValue(target["y"]); !numericOK {
		return nil, errors.New("drag.preview target requires numeric y")
	}
	return payload, nil
}

func numericValue(value any) (float64, bool) {
	number, ok := value.(float64)
	return number, ok
}

func stringValue(value any) string {
	text, _ := value.(string)
	return text
}

func rawObject(raw json.RawMessage) bool {
	var payload map[string]any
	if len(raw) == 0 {
		return false
	}
	return json.Unmarshal(raw, &payload) == nil && payload != nil
}

func maxInt64(left, right int64) int64 {
	if right > left {
		return right
	}
	return left
}

func normalizeLockScope(scope model.LockScopePayload) (model.LockScopePayload, bool) {
	scope.Type = strings.TrimSpace(scope.Type)
	scope.EntityID = strings.TrimSpace(scope.EntityID)
	scope.ConstraintID = strings.TrimSpace(scope.ConstraintID)
	scope.DimensionID = strings.TrimSpace(scope.DimensionID)
	scope.ComponentID = strings.TrimSpace(scope.ComponentID)

	switch scope.Type {
	case "entity":
		if scope.EntityID == "" {
			return model.LockScopePayload{}, false
		}
	case "constraint":
		if scope.ConstraintID == "" {
			return model.LockScopePayload{}, false
		}
	case "dimension":
		if scope.DimensionID == "" {
			return model.LockScopePayload{}, false
		}
	case "constraint_component":
		if scope.ComponentID == "" {
			return model.LockScopePayload{}, false
		}
	default:
		return model.LockScopePayload{}, false
	}
	return scope, true
}

func lockAcquiredPayload(lock *model.SketchLock) (model.LockAcquiredPayload, error) {
	var scope model.LockScopePayload
	if err := json.Unmarshal(lock.Scope, &scope); err != nil {
		return model.LockAcquiredPayload{}, fmt.Errorf("decode acquired lock scope: %w", err)
	}

	entityIDs := make([]string, 0, 1)
	if scope.EntityID != "" {
		entityIDs = append(entityIDs, scope.EntityID)
	}

	return model.LockAcquiredPayload{
		LockID:    lock.ID,
		UserID:    lock.OwnerUserID,
		Scope:     scope,
		EntityIDs: entityIDs,
		ExpiresAt: lock.ExpiresAt,
	}, nil
}

func optionalString(value string) *string {
	if value == "" {
		return nil
	}
	return &value
}

func opRejectedPayload(clientOpID string, response *model.SubmitOperationResponse) model.OpRejectedPayload {
	payload := model.OpRejectedPayload{
		ClientOpID:     clientOpID,
		CurrentVersion: response.CurrentVersion,
		Reason:         "invalid_operation",
	}
	if response.Rejection != nil {
		payload.Reason = strings.TrimSpace(response.Rejection.Reason)
		payload.Message = response.Rejection.Message
		payload.Diagnostics = joinRawMessages(response.Rejection.Diagnostics)
	}
	if payload.Reason == "" {
		payload.Reason = "invalid_operation"
	}
	return payload
}

func opCommittedPayload(
	userID string,
	request model.OpSubmitPayload,
	response *model.SubmitOperationResponse,
) (model.OpCommittedPayload, error) {
	if response.OpID == nil || strings.TrimSpace(*response.OpID) == "" {
		return model.OpCommittedPayload{}, errors.New("operations service returned accepted response without opId")
	}
	if response.Version == nil {
		return model.OpCommittedPayload{}, errors.New("operations service returned accepted response without version")
	}

	return model.OpCommittedPayload{
		OpID:                  *response.OpID,
		Version:               *response.Version,
		ActorUserID:           userID,
		ClientOpID:            request.ClientOpID,
		Op:                    request.Op,
		Patch:                 json.RawMessage(response.Patch),
		SolveStatus:           json.RawMessage(response.SolveStatus),
		AffectedEntityIDs:     append([]string(nil), response.ChangedEntityIDs...),
		AffectedConstraintIDs: append([]string(nil), response.ChangedConstraintIDs...),
		AffectedDimensionIDs:  append([]string(nil), response.ChangedDimensionIDs...),
		AffectedComponentIDs:  append([]string(nil), response.ChangedComponentIDs...),
		Authoritative:         true,
	}, nil
}

func committedPatchPayloads(ops []model.CommittedOperation) []model.CommittedPatchPayload {
	patches := make([]model.CommittedPatchPayload, 0, len(ops))
	for _, op := range ops {
		clientOpID := ""
		if op.ClientOpID != nil {
			clientOpID = *op.ClientOpID
		}
		patches = append(patches, model.CommittedPatchPayload{
			Version:       op.Version,
			OpID:          op.ID,
			ActorUserID:   op.ActorUserID,
			ClientOpID:    clientOpID,
			Patch:         json.RawMessage(op.Patch),
			SolveStatus:   json.RawMessage(op.SolveStatus),
			Authoritative: true,
		})
	}
	return patches
}

func joinRawMessages(messages []easyjson.RawMessage) json.RawMessage {
	if len(messages) == 0 {
		return nil
	}

	parts := make([]json.RawMessage, 0, len(messages))
	for _, message := range messages {
		if len(message) > 0 {
			parts = append(parts, json.RawMessage(message))
		}
	}
	if len(parts) == 0 {
		return nil
	}

	body, err := json.Marshal(parts)
	if err != nil {
		return nil
	}
	return body
}

func (s *Service) join(conn *Connection) ([]model.UserPresenceSummary, bool) {
	s.mu.Lock()
	defer s.mu.Unlock()

	activeUsers := make([]model.UserPresenceSummary, 0)
	for _, existing := range s.connections[conn.sketchID] {
		if existing.id != conn.id && existing.joined {
			activeUsers = append(activeUsers, existing.presence())
		}
	}

	if s.connections[conn.sketchID] == nil {
		s.connections[conn.sketchID] = make(map[string]*Connection)
	}
	_, alreadyJoined := s.connections[conn.sketchID][conn.id]
	s.connections[conn.sketchID][conn.id] = conn
	conn.joined = true

	return activeUsers, !alreadyJoined
}

func (s *Service) leave(conn *Connection, reason string) {
	s.mu.Lock()
	if sketchConnections := s.connections[conn.sketchID]; sketchConnections != nil {
		delete(sketchConnections, conn.id)
		if len(sketchConnections) == 0 {
			delete(s.connections, conn.sketchID)
		}
	}
	wasJoined := conn.joined
	conn.joined = false
	s.mu.Unlock()

	if !wasJoined {
		return
	}

	s.broadcastExcept(conn.sketchID, conn.id, model.ServerRealtimeMessage{
		Type:       msgSessionUserLeft,
		EventID:    newID(),
		SketchID:   conn.sketchID,
		ServerTime: time.Now().UTC().Format(time.RFC3339Nano),
		Payload: mustJSON(model.SessionUserLeftPayload{
			UserID:   conn.userID,
			ClientID: conn.clientID,
			Reason:   reason,
		}),
	})
}

func (s *Service) broadcastExcept(sketchID, excludedConnectionID string, msg model.ServerRealtimeMessage) {
	s.mu.Lock()
	recipients := make([]*Connection, 0, len(s.connections[sketchID]))
	for id, conn := range s.connections[sketchID] {
		if id != excludedConnectionID && conn.joined {
			recipients = append(recipients, conn)
		}
	}
	s.mu.Unlock()

	for _, conn := range recipients {
		conn.enqueue(msg)
	}
}

func (s *Service) broadcast(sketchID string, msg model.ServerRealtimeMessage) {
	s.mu.Lock()
	recipients := make([]*Connection, 0, len(s.connections[sketchID]))
	for _, conn := range s.connections[sketchID] {
		if conn.joined {
			recipients = append(recipients, conn)
		}
	}
	s.mu.Unlock()

	for _, conn := range recipients {
		conn.enqueue(msg)
	}
}

func (s *Service) closeUserConnections(ctx context.Context, sketchID, userID, message string) {
	s.mu.Lock()
	targets := make([]*Connection, 0)
	for _, conn := range s.connections[sketchID] {
		if conn.userID == userID && conn.joined {
			targets = append(targets, conn)
		}
	}
	s.mu.Unlock()

	for _, conn := range targets {
		conn.SendAccessRevoked(message)
		_ = conn.Close(ctx, closeReasonAccessRevoked)
	}
}

func sessionCloseReason(reason string) string {
	switch reason {
	case closeReasonAccessRevoked, closeReasonServerShutdown, closeReasonDuplicateConnection, closeReasonProtocolError:
		return reason
	default:
		return closeReasonDisconnect
	}
}

func mustJSON(payload any) json.RawMessage {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil
	}
	return body
}

func (c *Connection) enqueue(msg model.ServerRealtimeMessage) {
	defer func() {
		_ = recover()
	}()
	select {
	case c.outbound <- msg:
	default:
	}
}

func newID() string {
	var id [16]byte
	if _, err := rand.Read(id[:]); err != nil {
		return time.Now().UTC().Format("20060102150405.000000000")
	}

	id[6] = (id[6] & uuidVersionBitmask) | uuidVersionMask
	id[8] = (id[8] & uuidVariantBitmask) | uuidVariantMask

	var encoded [36]byte
	hex.Encode(encoded[0:8], id[0:4])
	encoded[8] = '-'
	hex.Encode(encoded[9:13], id[4:6])
	encoded[13] = '-'
	hex.Encode(encoded[14:18], id[6:8])
	encoded[18] = '-'
	hex.Encode(encoded[19:23], id[8:10])
	encoded[23] = '-'
	hex.Encode(encoded[24:36], id[10:16])

	return string(encoded[:])
}
