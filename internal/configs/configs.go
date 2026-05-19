package configs

import (
	"context"
	"log/slog"
	"sync/atomic"

	"github.com/dnonakolesax/viper"
	"github.com/joho/godotenv"

	"github.com/dnonakolesax/cccad-locks/internal/consts"
	"github.com/dnonakolesax/cccad-locks/internal/vault"
)

type Config struct {
	PSQL  *RDBConfig
	Redis *RedisConfig

	HTTPServer *HTTPServerConfig
	Service    *ServiceConfig

	S3 *S3Config

	Vault *VaultConfig

	Logger *LoggerConfig

	UpdateChans *UpdateChans
}

type UpdateChans struct {
	PSQLCredentials chan string
	RedisPassword   chan string
	KCClientSecret  chan string
}

func ListenUpdates(updateChan chan viper.KVEntry, hc *atomic.Bool) *UpdateChans {
	psqlChan := make(chan string)
	redisChan := make(chan string)
	kcChan := make(chan string)

	go func() {
		for value := range updateChan {
			switch value.Key {
			case postgresRolePath:
				psqlChan <- value.Value
			case RedisPasswordKey:
				redisChan <- value.Value
			}
		}
		hc.Store(false)
	}()

	return &UpdateChans{
		PSQLCredentials: psqlChan,
		RedisPassword:   redisChan,
		KCClientSecret:  kcChan,
	}
}

func SetupConfigs(initLogger *slog.Logger, configsDir string, hc *atomic.Bool) (*Config, error) {
	err := godotenv.Load()
	if err != nil {
		initLogger.WarnContext(context.Background(), "Error loading .env file")
	}

	v := viper.New()
	v.PanicOnNil = true

	psqlConfig := &RDBConfig{}
	redisConfig := &RedisConfig{}
	serverConfig := &HTTPServerConfig{}
	serviceConfig := &ServiceConfig{}
	loggerConfig := &LoggerConfig{}
	s3Config := &S3Config{}
	vaultConfig := NewVaultConfig()
	creds := &vault.Credentials{
		Login:    vaultConfig.Login,
		Password: vaultConfig.Password,
	}
	vaultClient, err := vault.SetupVault(vaultConfig.Address, creds, initLogger)

	if err != nil {
		initLogger.ErrorContext(context.Background(), "Error creating vault client",
			slog.String(consts.ErrorLoggerKey, err.Error()))
		return nil, err
	}
	hc.Store(true)

	err = Load(configsDir, v, initLogger, vaultClient.Client, vaultClient.UpdateChan, psqlConfig,
		redisConfig, s3Config, serverConfig, serviceConfig, loggerConfig)

	if err != nil {
		initLogger.ErrorContext(context.Background(), "Error loading config",
			slog.String(consts.ErrorLoggerKey, err.Error()))
		hc.Store(false)
		return nil, err
	}

	updates := ListenUpdates(vaultClient.UpdateChan, hc)

	return &Config{
		PSQL:        psqlConfig,
		Redis:       redisConfig,
		HTTPServer:  serverConfig,
		Service:     serviceConfig,
		Logger:      loggerConfig,
		S3:          s3Config,
		Vault:       vaultConfig,
		UpdateChans: updates,
	}, nil
}
