package logger

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/exec"
	"time"

	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/dnonakolesax/cccad-locks/internal/configs"
)

type Loggers struct {
	HTTP    *slog.Logger
	HTTPc   *slog.Logger
	GRPC    *slog.Logger
	Service *slog.Logger
	Repo    *slog.Logger
	Infra   *slog.Logger
}

func NewLogger(cfg *configs.LoggerConfig, layer string) *slog.Logger {
	logFile := &lumberjack.Logger{
		Filename:   fmt.Sprintf("/var/log/noted-auth/%s.log", layer),
		MaxSize:    cfg.LogMaxFileSize,
		MaxBackups: cfg.LogMaxBackups,
		MaxAge:     cfg.LogMaxAge,
		Compress:   true,
	}
	multiWriter := io.MultiWriter(os.Stdout, logFile)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	commitHash, ok := os.LookupEnv("CI_COMMIT_HASH")
	if !ok {
		hash, err := exec.CommandContext(ctx, "git", "rev-parse", "--short", "HEAD").Output()
		if err != nil {
			commitHash = "unknown"
		} else {
			commitHash = string(hash)
		}
	}

	podName, ok := os.LookupEnv("POD_NAME")

	if !ok {
		podName = "000000"
	}

	var logLevel slog.Level

	switch cfg.LogLevel {
	case "debug":
		logLevel = slog.LevelDebug
	case "info":
		logLevel = slog.LevelInfo
	case "warn":
		logLevel = slog.LevelWarn
	case "error":
		logLevel = slog.LevelError
	default:
		logLevel = slog.LevelInfo
	}

	handler := slog.NewJSONHandler(multiWriter, &slog.HandlerOptions{
		AddSource: cfg.LogAddSource,
		Level:     logLevel,
	})
	logger := slog.New(handler).With(
		slog.Group("exe info",
			slog.Int("pid", os.Getpid()),
			slog.String("commit hash", commitHash),
			slog.String("pod name", podName),
		),
		slog.String("layer", layer),
	)

	return logger
}

func SetupLoggers(cfg *configs.LoggerConfig) *Loggers {
	return &Loggers{
		HTTP:    NewLogger(cfg, "http-server"),
		HTTPc:   NewLogger(cfg, "http-client"),
		GRPC:    NewLogger(cfg, "grpc"),
		Service: NewLogger(cfg, "service"),
		Repo:    NewLogger(cfg, "repo"),
		Infra:   NewLogger(cfg, "infra"),
	}
}
