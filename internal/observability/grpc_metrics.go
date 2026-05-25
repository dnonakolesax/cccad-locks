package observability

import (
	"context"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"google.golang.org/grpc"
	"google.golang.org/grpc/status"
)

const (
	grpcLabelTarget  = "target"
	grpcLabelService = "service"
	grpcLabelMethod  = "method"
	grpcLabelCode    = "code"
	unknownLabel     = "unknown"
)

type GRPCClientMetrics struct {
	requests *prometheus.CounterVec
	duration *prometheus.HistogramVec
	inflight *prometheus.GaugeVec
}

func NewGRPCClientMetrics(registerer prometheus.Registerer) *GRPCClientMetrics {
	metrics := &GRPCClientMetrics{
		requests: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "cccad_grpc_client_requests_total",
				Help: "Total outbound gRPC client requests.",
			},
			[]string{grpcLabelTarget, grpcLabelService, grpcLabelMethod, grpcLabelCode},
		),
		duration: prometheus.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "cccad_grpc_client_request_duration_seconds",
				Help:    "Outbound gRPC client request duration in seconds.",
				Buckets: prometheus.DefBuckets,
			},
			[]string{grpcLabelTarget, grpcLabelService, grpcLabelMethod, grpcLabelCode},
		),
		inflight: prometheus.NewGaugeVec(
			prometheus.GaugeOpts{
				Name: "cccad_grpc_client_inflight_requests",
				Help: "Outbound gRPC client requests currently in flight.",
			},
			[]string{grpcLabelTarget, grpcLabelService, grpcLabelMethod},
		),
	}

	if registerer != nil {
		registerer.MustRegister(metrics.requests, metrics.duration, metrics.inflight)
	}

	return metrics
}

func (m *GRPCClientMetrics) UnaryClientInterceptor(target string) grpc.UnaryClientInterceptor {
	if m == nil {
		return nil
	}

	target = strings.TrimSpace(target)
	if target == "" {
		target = unknownLabel
	}

	return func(
		ctx context.Context,
		method string,
		req any,
		reply any,
		cc *grpc.ClientConn,
		invoker grpc.UnaryInvoker,
		opts ...grpc.CallOption,
	) error {
		service, rpcMethod := splitFullMethod(method)
		m.inflight.WithLabelValues(target, service, rpcMethod).Inc()
		start := time.Now()

		err := invoker(ctx, method, req, reply, cc, opts...)
		code := status.Code(err).String()

		m.inflight.WithLabelValues(target, service, rpcMethod).Dec()
		m.requests.WithLabelValues(target, service, rpcMethod, code).Inc()
		m.duration.WithLabelValues(target, service, rpcMethod, code).Observe(time.Since(start).Seconds())

		return err
	}
}

func splitFullMethod(fullMethod string) (string, string) {
	fullMethod = strings.Trim(fullMethod, "/")
	service, method, ok := strings.Cut(fullMethod, "/")
	if !ok || service == "" || method == "" {
		return unknownLabel, unknownLabel
	}

	return service, method
}
