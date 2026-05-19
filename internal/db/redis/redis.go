package redis

import (
	"context"
	"errors"
	"log/slog"
	"runtime"
	"strconv"
	"sync/atomic"
	"time"

	"github.com/redis/go-redis/v9"

	"github.com/dnonakolesax/cccad-locks/internal/configs"
	"github.com/dnonakolesax/cccad-locks/internal/consts"
	"github.com/dnonakolesax/cccad-locks/internal/xerrors"
)

type Client struct {
	Client       *redis.Client
	Config       *configs.RedisConfig
	Timeout      time.Duration
	logger       *slog.Logger
	Alive        *atomic.Bool
	ConnUpdating *atomic.Bool
}

const addressLoggerKey = "address"

func newClient(cfg *configs.RedisConfig, logger *slog.Logger) (*redis.Client, error) {
	options := &redis.Options{
		Addr:     cfg.Address + ":" + strconv.Itoa(cfg.Port),
		Password: cfg.Password,
		DB:       0,
	}

	logger.Info("Starting new redis client", slog.String(addressLoggerKey, options.Addr))
	client := redis.NewClient(options)
	logger.Info("Redis client started", slog.String(addressLoggerKey, options.Addr))

	ctx, cancel := context.WithTimeout(context.Background(), cfg.RequestTimeout)
	defer cancel()
	logger.Info("Trying to ping redis client", slog.String(addressLoggerKey, options.Addr))
	err := client.Ping(ctx).Err()

	if err != nil {
		logger.Error("Error while pinging redis client", slog.String(addressLoggerKey, options.Addr),
			slog.String(consts.ErrorLoggerKey, err.Error()))
		return nil, err
	}
	logger.Info("Redis client ping successfull", slog.String(addressLoggerKey, options.Addr))
	return client, nil
}

func NewClient(
	cfg *configs.RedisConfig,
	alive *atomic.Bool,
	logger *slog.Logger,
	vaultChan chan string,
) (*Client, error) {
	client, err := newClient(cfg, logger)

	if err != nil {
		return nil, err
	}

	alive.Store(true)
	c := &Client{
		Client:       client,
		Config:       cfg,
		Timeout:      cfg.RequestTimeout,
		logger:       logger,
		Alive:        alive,
		ConnUpdating: &atomic.Bool{},
	}

	go c.MonitorVault(vaultChan)

	return c, nil
}

func (c *Client) MonitorVault(vaultChan chan string) {
	for passwd := range vaultChan {
		cfg := c.Config
		cfg.Password = passwd
		client, err := newClient(cfg, c.logger)

		if err != nil {
			c.logger.Error("Error creating new redis conn from vault credentials",
				slog.String(consts.ErrorLoggerKey, err.Error()))
			continue
		}

		for !c.ConnUpdating.CompareAndSwap(false, true) {
			runtime.Gosched()
		}
		c.Client = client
		c.ConnUpdating.Store(false)
	}
}

func (c *Client) Get(ctx context.Context, key string) (string, error) {
	rctx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()
	if c.ConnUpdating.Load() {
		for c.ConnUpdating.Load() {
		}
	}
	val, err := c.Client.Get(rctx, key).Result()

	if errors.Is(err, redis.Nil) {
		c.logger.WarnContext(ctx, "State not found", slog.String(consts.ErrorLoggerKey, err.Error()))
		return "", xerrors.ErrObjectNotFoundInRepoError
	} else if err != nil {
		c.Alive.Store(false)
		c.logger.ErrorContext(ctx, "Error getting value from redis", slog.String(consts.ErrorLoggerKey, err.Error()))
		return "", err
	}

	return val, nil
}

func (c *Client) Set(ctx context.Context, key string, value string, ttl time.Duration) error {
	rctx, cancel := context.WithTimeout(ctx, c.Timeout)
	defer cancel()
	if c.ConnUpdating.Load() {
		for c.ConnUpdating.Load() {
		}
	}
	err := c.Client.Set(rctx, key, value, ttl).Err()

	if err != nil {
		c.Alive.Store(false)
		c.logger.ErrorContext(ctx, "Failed to set state to redis", slog.String(consts.ErrorLoggerKey, err.Error()))
		return err
	}
	return nil
}
