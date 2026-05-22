package realtime

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/dnonakolesax/cccad-locks/internal/model"
)

const protocolVersion = 1

const (
	msgSessionJoin          = "session.join"
	msgSessionJoined        = "session.joined"
	msgSessionUserJoined    = "session.user_joined"
	msgSessionUserLeft      = "session.user_left"
	msgSessionPing          = "session.ping"
	msgSessionPong          = "session.pong"
	msgSessionAccessRevoked = "session.access_revoked"
	msgError                = "error"
)

const (
	closeReasonDisconnect          = "disconnect"
	closeReasonAccessRevoked       = "access_revoked"
	closeReasonServerShutdown      = "server_shutdown"
	closeReasonDuplicateConnection = "duplicate_connection"
	closeReasonProtocolError       = "protocol_error"
)

type Permissions interface {
	List(ctx context.Context, sketchID string) ([]model.Permission, error)
}

type Sketches interface {
	Get(ctx context.Context, sketchID string) (*model.SketchDocument, error)
}

type Service struct {
	permissions Permissions
	sketches    Sketches

	mu          sync.Mutex
	connections map[string]map[string]*Connection
}

func NewService(permissions Permissions, sketches Sketches) *Service {
	return &Service{
		permissions: permissions,
		sketches:    sketches,
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

	return "", errors.New("permission denied")
}

func canRead(role string) bool {
	switch role {
	case "reader", "editor", "admin":
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

func newConnection(service *Service, req model.OpenRealtimeSessionRequest, role string, currentVersion int64) *Connection {
	return &Connection{
		service:        service,
		id:             newID(),
		sketchID:       req.SketchID,
		userID:         req.UserID,
		displayName:    req.UserName,
		clientID:       req.ClientID,
		role:           role,
		currentVersion: currentVersion,
		outbound:       make(chan model.ServerRealtimeMessage, 16),
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
	switch msg.Type {
	case msgSessionJoin:
		if err := c.handleSessionJoin(msg); err != nil {
			c.sendError(msg.RequestID, "INVALID_MESSAGE", err.Error())
			return err
		}
	case msgSessionPing:
		if err := c.handleSessionPing(msg); err != nil {
			c.sendError(msg.RequestID, "INVALID_MESSAGE", err.Error())
			return err
		}
	default:
		c.sendError(msg.RequestID, "CONSTRAINT_NOT_SUPPORTED", "realtime message type is not implemented")
	}

	select {
	case <-ctx.Done():
		return ctx.Err()
	default:
		return nil
	}
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
	body, err := json.Marshal(payload)
	if err != nil {
		c.sendError(requestID, "INTERNAL_ERROR", "failed to encode realtime payload")
		return
	}

	c.enqueue(model.ServerRealtimeMessage{
		Type:       messageType,
		EventID:    newID(),
		RequestID:  requestID,
		SketchID:   c.sketchID,
		ServerTime: time.Now().UTC().Format(time.RFC3339Nano),
		Payload:    body,
	})
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

	id[6] = (id[6] & 0x0f) | 0x40
	id[8] = (id[8] & 0x3f) | 0x80

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
