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

	Solver   *SolverConfig
	Geometry *GeometryConfig
	Auth     *AuthConfig

	Vault *VaultConfig

	Logger *LoggerConfig

	UpdateChans *UpdateChans
}

type UpdateChans struct {
	PSQLCredentials chan string
	RedisPassword   chan string
	KCClientSecret  chan string
}

func ListenUpdates(
	updateChan chan viper.KVEntry,
	hc *atomic.Bool,
	psqlRolePath string,
	redisPasswordPath string,
) *UpdateChans {
	psqlChan := make(chan string)
	redisChan := make(chan string)
	kcChan := make(chan string)

	go func() {
		for value := range updateChan {
			switch value.Key {
			case psqlRolePath:
				psqlChan <- value.Value
			case redisPasswordPath:
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
	solverConfig := &SolverConfig{}
	geometryConfig := &GeometryConfig{}
	authConfig := &AuthConfig{}
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
		redisConfig, s3Config, solverConfig, geometryConfig, authConfig, serverConfig, serviceConfig, loggerConfig)

	if err != nil {
		initLogger.ErrorContext(context.Background(), "Error loading config",
			slog.String(consts.ErrorLoggerKey, err.Error()))
		hc.Store(false)
		return nil, err
	}

	updates := ListenUpdates(vaultClient.UpdateChan, hc, psqlConfig.RolePath, redisConfig.PasswordPath)

	return &Config{
		PSQL:        psqlConfig,
		Redis:       redisConfig,
		HTTPServer:  serverConfig,
		Service:     serviceConfig,
		Logger:      loggerConfig,
		S3:          s3Config,
		Solver:      solverConfig,
		Geometry:    geometryConfig,
		Auth:        authConfig,
		Vault:       vaultConfig,
		UpdateChans: updates,
	}, nil
}
