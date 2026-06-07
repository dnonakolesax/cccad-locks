package geometry

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/dnonakolesax/cccad-locks/internal/configs"
	"github.com/dnonakolesax/cccad-locks/internal/grpcutil"
	"github.com/dnonakolesax/cccad-locks/internal/observability"
	geometryv1 "github.com/dnonakolesax/cccad-locks/internal/proto/geometry/v1"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

type Client struct {
	conn           *grpc.ClientConn
	client         geometryv1.GeometrySolverServiceClient
	logger         *slog.Logger
	requestTimeout time.Duration
}

func NewClient(
	cfg *configs.GeometryConfig,
	logger *slog.Logger,
	metrics *observability.GRPCClientMetrics,
) (*Client, error) {
	if cfg == nil {
		return nil, errors.New("geometry config is nil")
	}
	if cfg.Address == "" {
		return nil, errors.New("geometry address is empty")
	}

	opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
	if metrics != nil {
		opts = append(opts, grpc.WithUnaryInterceptor(metrics.UnaryClientInterceptor("geometry")))
	}

	conn, err := grpc.NewClient(cfg.Address, opts...)
	if err != nil {
		return nil, err
	}

	if logger != nil {
		logger.Info("Created geometry grpc client", slog.String("address", cfg.Address))
	}

	return &Client{
		conn:           conn,
		client:         geometryv1.NewGeometrySolverServiceClient(conn),
		logger:         logger,
		requestTimeout: cfg.RequestTimeout,
	}, nil
}

func (c *Client) Ping(ctx context.Context) error {
	ctx, cancel := c.contextWithTimeout(ctx)
	defer cancel()

	return grpcutil.Ping(ctx, c.conn)
}

func (c *Client) Health(ctx context.Context) (*geometryv1.HealthResponse, error) {
	ctx, cancel := c.contextWithTimeout(ctx)
	defer cancel()

	return c.client.Health(ctx, &geometryv1.HealthRequest{})
}

func (c *Client) BuildExtrude(
	ctx context.Context,
	req *geometryv1.BuildExtrudeRequest,
) (*geometryv1.BuildFeatureResponse, error) {
	ctx, cancel := c.contextWithTimeout(ctx)
	defer cancel()

	return c.client.BuildExtrude(ctx, req)
}

func (c *Client) BuildHole(
	ctx context.Context,
	req *geometryv1.BuildHoleRequest,
) (*geometryv1.BuildFeatureResponse, error) {
	ctx, cancel := c.contextWithTimeout(ctx)
	defer cancel()

	return c.client.BuildHole(ctx, req)
}

func (c *Client) BuildFillet(
	ctx context.Context,
	req *geometryv1.BuildFilletRequest,
) (*geometryv1.BuildFeatureResponse, error) {
	ctx, cancel := c.contextWithTimeout(ctx)
	defer cancel()

	return c.client.BuildFillet(ctx, req)
}

func (c *Client) BuildChamfer(
	ctx context.Context,
	req *geometryv1.BuildChamferRequest,
) (*geometryv1.BuildFeatureResponse, error) {
	ctx, cancel := c.contextWithTimeout(ctx)
	defer cancel()

	return c.client.BuildChamfer(ctx, req)
}

func (c *Client) BuildPattern(
	ctx context.Context,
	req *geometryv1.BuildPatternRequest,
) (*geometryv1.BuildFeatureResponse, error) {
	ctx, cancel := c.contextWithTimeout(ctx)
	defer cancel()

	return c.client.BuildPattern(ctx, req)
}

func (c *Client) BuildBoolean(
	ctx context.Context,
	req *geometryv1.BuildBooleanRequest,
) (*geometryv1.BuildFeatureResponse, error) {
	ctx, cancel := c.contextWithTimeout(ctx)
	defer cancel()

	return c.client.BuildBoolean(ctx, req)
}

func (c *Client) RebuildPart(
	ctx context.Context,
	req *geometryv1.RebuildPartRequest,
) (*geometryv1.RebuildPartResponse, error) {
	ctx, cancel := c.contextWithTimeout(ctx)
	defer cancel()

	return c.client.RebuildPart(ctx, req)
}

func (c *Client) GetTopology(
	ctx context.Context,
	req *geometryv1.GetTopologyRequest,
) (*geometryv1.GetTopologyResponse, error) {
	ctx, cancel := c.contextWithTimeout(ctx)
	defer cancel()

	return c.client.GetTopology(ctx, req)
}

func (c *Client) GetFacePlane(
	ctx context.Context,
	req *geometryv1.GetFacePlaneRequest,
) (*geometryv1.GetFacePlaneResponse, error) {
	ctx, cancel := c.contextWithTimeout(ctx)
	defer cancel()

	return c.client.GetFacePlane(ctx, req)
}

func (c *Client) Close() error {
	if c == nil || c.conn == nil {
		return nil
	}
	if c.logger != nil {
		c.logger.Info("Closing geometry grpc client")
	}

	return c.conn.Close()
}

func (c *Client) contextWithTimeout(ctx context.Context) (context.Context, context.CancelFunc) {
	if c.requestTimeout <= 0 {
		return context.WithCancel(ctx)
	}

	return context.WithTimeout(ctx, c.requestTimeout)
}
