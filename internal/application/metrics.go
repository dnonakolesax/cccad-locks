package application

import (
	"github.com/dnonakolesax/cccad-locks/internal/observability"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/collectors"
)

type Metrics struct {
	Reg        *prometheus.Registry
	GRPCClient *observability.GRPCClientMetrics
}

func (a *App) SetupMetrics() {
	reg := prometheus.NewRegistry()

	reg.MustRegister(
		collectors.NewGoCollector(),
		collectors.NewProcessCollector(collectors.ProcessCollectorOpts{}),
	)

	a.metrics = &Metrics{Reg: reg}
	a.metrics.GRPCClient = observability.NewGRPCClientMetrics(reg)
}
