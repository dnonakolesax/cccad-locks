package application

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"os/signal"
	"path"
	"strconv"
	"strings"
	"sync"
	"syscall"

	"github.com/dnonakolesax/cccad-locks/internal/auth"
	"github.com/dnonakolesax/cccad-locks/internal/configs"
	"github.com/dnonakolesax/cccad-locks/internal/consts"
	"github.com/dnonakolesax/cccad-locks/internal/logger"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type App struct {
	configs    *configs.Config
	health     *HealthChecks
	metrics    *Metrics
	initLogger *slog.Logger
	layers     *Layers
	loggers    *logger.Loggers
	components *Components
}

func NewApp(configsDir string) (*App, error) {
	lcfg := &configs.LoggerConfig{LogLevel: "info", LogAddSource: true}
	initLogger := logger.NewLogger(lcfg, "init")
	app := &App{}

	app.initLogger = initLogger

	app.SetupHealthChecks()

	configs, err := configs.SetupConfigs(initLogger, configsDir, app.health.Vault)

	if err != nil {
		return nil, err
	}

	app.configs = configs

	loggers := logger.SetupLoggers(app.configs.Logger)

	app.loggers = loggers

	app.SetupMetrics()

	err = app.SetupComponents()

	if err != nil {
		return nil, err
	}

	err = app.SetupLayers()

	if err != nil {
		return nil, err
	}

	return app, nil
}

func (a *App) Run() {
	router := http.NewServeMux()
	a.registerRoutes(router)

	mux := http.NewServeMux()
	mountPath := apiMountPath(a.configs.Service.BasePath)
	if mountPath == "/" {
		mux.Handle("/", router)
	} else {
		mux.Handle(mountPath, stripPrefixWithRoot(mountPath, router))
		mux.Handle(mountPath+"/", http.StripPrefix(mountPath, router))
	}

	handler := http.Handler(mux)
	if a.configs.HTTPServer.MaxReqBodySize > 0 {
		handler = http.MaxBytesHandler(handler, int64(a.configs.HTTPServer.MaxReqBodySize))
	}
	handler = auth.NewMiddleware(a.components.auth, a.loggers.HTTP).Handle(handler)
	handler = a.loggingMiddleware(handler)

	server := &http.Server{
		Addr:         ":" + strconv.Itoa(a.configs.Service.Port),
		Handler:      handler,
		ReadTimeout:  a.configs.HTTPServer.ReadTimeout,
		WriteTimeout: a.configs.HTTPServer.WriteTimeout,
		IdleTimeout:  a.configs.HTTPServer.IdleTimeout,
	}

	metricsMux := http.NewServeMux()
	metricsEndpoint := normalizeBasePath(a.configs.Service.MetricsEndpoint)
	metricsMux.Handle(
		metricsEndpoint,
		promhttp.HandlerFor(a.metrics.Reg, promhttp.HandlerOpts{Registry: a.metrics.Reg}),
	)
	metricsHandler := auth.NewMiddleware(a.components.auth, a.loggers.HTTP).Handle(metricsMux)

	metricsServer := &http.Server{
		Handler:           metricsHandler,
		Addr:              ":" + strconv.Itoa(a.configs.Service.MetricsPort),
		ReadHeaderTimeout: a.configs.HTTPServer.ReadTimeout,
	}

	var wg sync.WaitGroup
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, os.Interrupt, syscall.SIGINT, syscall.SIGTERM)
	defer signal.Stop(quit)

	wg.Go(func() {
		a.initLogger.Info("Starting HTTP server",
			slog.Int("port", a.configs.Service.Port),
			slog.String("base_path", mountPath))
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			a.initLogger.Error("HTTP server error", slog.String(consts.ErrorLoggerKey, err.Error()))
		}
	})

	wg.Go(func() {
		a.initLogger.Info("Starting metrics server",
			slog.Int("port", a.configs.Service.MetricsPort),
			slog.String("endpoint", metricsEndpoint))
		if err := metricsServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			a.initLogger.Error("Metrics server error", slog.String(consts.ErrorLoggerKey, err.Error()))
		}
	})

	sig := <-quit
	a.initLogger.InfoContext(context.Background(), "Received signal", slog.String("signal", sig.String()))

	ctx, cancel := context.WithTimeout(context.Background(), a.configs.HTTPServer.IdleTimeout)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		a.initLogger.ErrorContext(context.Background(), "Main HTTP server shutdown error",
			slog.String(consts.ErrorLoggerKey, err.Error()))
	}

	if err := metricsServer.Shutdown(ctx); err != nil {
		a.initLogger.ErrorContext(context.Background(), "Metrics server shutdown error",
			slog.String(consts.ErrorLoggerKey, err.Error()))
	}

	a.closeComponents()
	wg.Wait()
}

func (a *App) registerRoutes(mux *http.ServeMux) {
	if a.layers == nil {
		return
	}
	if a.layers.SketchesHTTP != nil {
		a.layers.SketchesHTTP.RegisterRoutes(mux)
	}
	if a.layers.RealtimeWS != nil {
		a.layers.RealtimeWS.RegisterRoutes(mux)
	}
	if a.layers.WorkspacesHTTP != nil {
		a.layers.WorkspacesHTTP.RegisterRoutes(mux)
	}
	if a.layers.OperationsHTTP != nil {
		a.layers.OperationsHTTP.RegisterRoutes(mux)
	}
	if a.layers.SolverHTTP != nil {
		a.layers.SolverHTTP.RegisterRoutes(mux)
	}
	if a.layers.LocksHTTP != nil {
		a.layers.LocksHTTP.RegisterRoutes(mux)
	}
	if a.layers.PermissionsHTTP != nil {
		a.layers.PermissionsHTTP.RegisterRoutes(mux)
	}
}

func (a *App) loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		next.ServeHTTP(w, r)
		if a.loggers != nil && a.loggers.HTTP != nil {
			a.loggers.HTTP.InfoContext(r.Context(), "HTTP request",
				slog.String("method", r.Method),
				slog.String("path", r.URL.Path),
				slog.String("remote_addr", r.RemoteAddr))
		}
	})
}

func (a *App) closeComponents() {
	if a.components == nil {
		return
	}
	if a.components.pgsql != nil && a.components.pgsql.Conn != nil {
		a.components.pgsql.Conn.Disconnect()
	}
	if a.components.redis != nil && a.components.redis.Client != nil {
		if err := a.components.redis.Client.Close(); err != nil {
			a.initLogger.Error("Redis close error", slog.String(consts.ErrorLoggerKey, err.Error()))
		}
	}
	if a.components.solver != nil {
		if err := a.components.solver.Close(); err != nil {
			a.initLogger.Error("Solver grpc close error", slog.String(consts.ErrorLoggerKey, err.Error()))
		}
	}
	if a.components.auth != nil {
		if err := a.components.auth.Close(); err != nil {
			a.initLogger.Error("Auth grpc close error", slog.String(consts.ErrorLoggerKey, err.Error()))
		}
	}
}

func normalizeBasePath(basePath string) string {
	basePath = strings.TrimSpace(basePath)
	if basePath == "" || basePath == "/" {
		return "/"
	}

	return path.Clean("/" + strings.Trim(basePath, "/"))
}

func apiMountPath(basePath string) string {
	const apiPrefix = "/api/v1"

	basePath = normalizeBasePath(basePath)
	if basePath == "/" {
		return apiPrefix
	}

	return path.Clean(apiPrefix + basePath)
}

func stripPrefixWithRoot(prefix string, h http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		r2 := new(http.Request)
		*r2 = *r
		r2.URL = cloneURL(r.URL)
		r2.URL.Path = strings.TrimPrefix(r.URL.Path, prefix)
		r2.URL.RawPath = strings.TrimPrefix(r.URL.RawPath, prefix)
		if r2.URL.Path == "" {
			r2.URL.Path = "/"
		}
		if r2.URL.RawPath == "" {
			r2.URL.RawPath = "/"
		}
		h.ServeHTTP(w, r2)
	})
}

func cloneURL(u *url.URL) *url.URL {
	u2 := new(url.URL)
	*u2 = *u
	return u2
}
