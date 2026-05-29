package solver

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/dnonakolesax/cccad-locks/internal/configs"
	"github.com/dnonakolesax/cccad-locks/internal/grpcutil"
	"github.com/dnonakolesax/cccad-locks/internal/observability"
	solverv1 "github.com/dnonakolesax/cccad-locks/internal/proto/solver/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/protobuf/encoding/protojson"
	"google.golang.org/protobuf/proto"
)

type Client struct {
	conn           *grpc.ClientConn
	client         solverv1.SketchSolverClient
	logger         *slog.Logger
	requestTimeout time.Duration
}

func NewClient(
	cfg *configs.SolverConfig,
	logger *slog.Logger,
	metrics *observability.GRPCClientMetrics,
) (*Client, error) {
	if cfg == nil {
		return nil, errors.New("solver config is nil")
	}
	if cfg.Address == "" {
		return nil, errors.New("solver address is empty")
	}

	opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
	if metrics != nil {
		opts = append(opts, grpc.WithUnaryInterceptor(metrics.UnaryClientInterceptor("solver")))
	}

	conn, err := grpc.NewClient(cfg.Address, opts...)
	if err != nil {
		return nil, err
	}

	if logger != nil {
		logger.Info("Created solver grpc client", slog.String("address", cfg.Address))
	}

	return &Client{
		conn:           conn,
		client:         solverv1.NewSketchSolverClient(conn),
		logger:         logger,
		requestTimeout: cfg.RequestTimeout,
	}, nil
}

func (c *Client) Ping(ctx context.Context) error {
	ctx, cancel := c.contextWithTimeout(ctx)
	defer cancel()

	return grpcutil.Ping(ctx, c.conn)
}

func (c *Client) Solve(ctx context.Context, req *solverv1.SolveRequest) (*solverv1.SolveResponse, error) {
	ctx, cancel := c.contextWithTimeout(ctx)
	defer cancel()

	resp, err := c.client.Solve(ctx, req)
	c.logger.DebugContext(ctx, "about to debug solve")
	if err == nil {
		c.debugProtoResponse(ctx, "Solve", resp)
	}
	return resp, err
}

func (c *Client) Check(ctx context.Context, req *solverv1.CheckRequest) (*solverv1.CheckResponse, error) {
	ctx, cancel := c.contextWithTimeout(ctx)
	defer cancel()

	return c.client.Check(ctx, req)
}

func (c *Client) ApplyIntent(
	ctx context.Context,
	req *solverv1.ApplyIntentRequest,
) (*solverv1.ApplyIntentResponse, error) {
	ctx, cancel := c.contextWithTimeout(ctx)
	defer cancel()

	resp, err := c.client.ApplyIntent(ctx, req)
	c.logger.DebugContext(ctx, "about to debug apply intent")
	if err == nil {
		c.debugProtoResponse(ctx, "ApplyIntent", resp)
	}
	return resp, err
}

func (c *Client) Analyze(ctx context.Context, req *solverv1.AnalyzeRequest) (*solverv1.AnalyzeResponse, error) {
	ctx, cancel := c.contextWithTimeout(ctx)
	defer cancel()

	return c.client.Analyze(ctx, req)
}

func (c *Client) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	if c.logger != nil {
		c.logger.Info("Closing solver grpc client")
	}

	return c.conn.Close()
}

func (c *Client) contextWithTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if c.requestTimeout <= 0 {
		return context.WithCancel(ctx)
	}

	return context.WithTimeout(ctx, c.requestTimeout)
}

func (c *Client) debugProtoResponse(ctx context.Context, method string, resp proto.Message) {
	if c.logger == nil || !c.logger.Enabled(ctx, slog.LevelDebug) {
		return
	}

	body, err := protojson.MarshalOptions{
		UseProtoNames:   true,
		EmitUnpopulated: false,
	}.Marshal(resp)
	if err != nil {
		c.logger.DebugContext(ctx, "Solver protobuf response marshal failed",
			slog.String("method", method),
			slog.String("error", err.Error()))
		return
	}

	c.logger.DebugContext(ctx, "Solver protobuf response",
		slog.String("method", method),
		slog.String("response", string(body)))
}
