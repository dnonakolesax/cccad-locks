package auth

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/dnonakolesax/cccad-locks/internal/configs"
	authv1 "github.com/dnonakolesax/cccad-locks/internal/proto/auth/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
)

type TokenData struct {
	UserID string
	AT     *string
	RT     *string
	IT     *string
}

type Client struct {
	conn           *grpc.ClientConn
	client         authv1.AuthServiceClient
	logger         *slog.Logger
	requestTimeout time.Duration
}

func NewClient(cfg *configs.AuthConfig, logger *slog.Logger) (*Client, error) {
	if cfg == nil {
		return nil, errors.New("auth config is nil")
	}
	if cfg.Address == "" {
		return nil, errors.New("auth address is empty")
	}

	conn, err := grpc.NewClient(
		cfg.Address,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
	)
	if err != nil {
		return nil, err
	}

	if logger != nil {
		logger.Info("Created auth grpc client", slog.String("address", cfg.Address))
	}

	return &Client{
		conn:           conn,
		client:         authv1.NewAuthServiceClient(conn),
		logger:         logger,
		requestTimeout: cfg.RequestTimeout,
	}, nil
}

func (c *Client) Authenticate(ctx context.Context, accessToken, refreshToken string) (*TokenData, error) {
	ctx, cancel := c.contextWithTimeout(ctx)
	defer cancel()
	if traceID, ok := TraceIDFromContext(ctx); ok {
		ctx = metadata.AppendToOutgoingContext(ctx, TraceIDHeader, traceID)
	}

	tokenData, err := c.client.AuthUserIDCtx(ctx, &authv1.UserTokens{
		Auth:    accessToken,
		Refresh: refreshToken,
	})
	if err != nil {
		return nil, err
	}
	if tokenData.GetID() == "" {
		return nil, errors.New("auth service returned empty user id")
	}

	return &TokenData{
		UserID: tokenData.GetID(),
		AT:     tokenData.At,
		RT:     tokenData.Rt,
		IT:     tokenData.It,
	}, nil
}

func (c *Client) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	if c.logger != nil {
		c.logger.Info("Closing auth grpc client")
	}

	return c.conn.Close()
}

func (c *Client) contextWithTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if c.requestTimeout <= 0 {
		return context.WithCancel(ctx)
	}

	return context.WithTimeout(ctx, c.requestTimeout)
}
