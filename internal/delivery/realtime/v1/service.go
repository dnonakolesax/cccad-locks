package v1

import (
	"context"

	"github.com/dnonakolesax/cccad-locks/internal/model"
)

// Service is the application-layer boundary used by the realtime delivery layer.
// It owns permissions, locks, presence storage, operation submission, solver calls,
// PostgreSQL persistence, and Redis pub/sub. This package only transports messages.
type Service interface {
	OpenConnection(ctx context.Context, req model.OpenRealtimeSessionRequest) (Connection, error)
}

// Connection represents one authenticated browser tab connected to one sketch.
// Implementation is expected to fan-in broadcasts from Redis/pubsub into Outbound().
type Connection interface {
	ID() string
	SketchID() string
	UserID() string

	HandleClientMessage(ctx context.Context, msg model.ClientRealtimeMessage) error
	Outbound() <-chan model.ServerRealtimeMessage
	Close(ctx context.Context, reason string) error
}

type UserIdentity struct {
	UserID      string
	DisplayName string
	AccessToken string
}

type UserResolver interface {
	Resolve(rctx context.Context, bearerToken string) (UserIdentity, error)
}
