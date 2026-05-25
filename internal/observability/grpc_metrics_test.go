package observability

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"google.golang.org/grpc"
)

func TestGRPCClientMetricsInterceptorRecordsUnaryCall(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := NewGRPCClientMetrics(reg)
	interceptor := metrics.UnaryClientInterceptor("solver")

	err := interceptor(
		context.Background(),
		"/cccad.solver.v1.SketchSolver/Analyze",
		nil,
		nil,
		nil,
		func(
			context.Context,
			string,
			any,
			any,
			*grpc.ClientConn,
			...grpc.CallOption,
		) error {
			return nil
		},
	)
	if err != nil {
		t.Fatalf("interceptor returned error: %v", err)
	}

	metricsFamilies, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}

	if !hasMetric(metricsFamilies, "cccad_grpc_client_requests_total", "Analyze", "OK") {
		t.Fatal("request counter for Analyze OK was not recorded")
	}
}

func TestGRPCClientMetricsInterceptorRecordsErrorCode(t *testing.T) {
	reg := prometheus.NewRegistry()
	metrics := NewGRPCClientMetrics(reg)
	interceptor := metrics.UnaryClientInterceptor("auth")
	callErr := errors.New("transport failed")

	err := interceptor(
		context.Background(),
		"/cccad.auth.v1.AuthService/AuthUserIDCtx",
		nil,
		nil,
		nil,
		func(
			context.Context,
			string,
			any,
			any,
			*grpc.ClientConn,
			...grpc.CallOption,
		) error {
			return callErr
		},
	)
	if !errors.Is(err, callErr) {
		t.Fatalf("interceptor error = %v, want %v", err, callErr)
	}

	metricsFamilies, err := reg.Gather()
	if err != nil {
		t.Fatalf("gather metrics: %v", err)
	}

	if !hasMetric(metricsFamilies, "cccad_grpc_client_requests_total", "AuthUserIDCtx", "Unknown") {
		t.Fatal("request counter for AuthUserIDCtx Unknown was not recorded")
	}
}

func hasMetric(metricsFamilies []*dto.MetricFamily, name string, values ...string) bool {
	for _, family := range metricsFamilies {
		if family.GetName() != name {
			continue
		}
		for _, metric := range family.GetMetric() {
			labels := make([]string, 0, len(metric.GetLabel()))
			for _, label := range metric.GetLabel() {
				labels = append(labels, label.GetValue())
			}
			if containsAll(labels, values...) {
				return true
			}
		}
	}

	return false
}

func containsAll(labels []string, values ...string) bool {
	joined := "\x00" + strings.Join(labels, "\x00") + "\x00"
	for _, value := range values {
		if !strings.Contains(joined, "\x00"+value+"\x00") {
			return false
		}
	}

	return true
}
