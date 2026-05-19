package application

import (
	"context"
	"log/slog"

	"github.com/dnonakolesax/cccad-locks/internal/consts"
	dbredis "github.com/dnonakolesax/cccad-locks/internal/db/redis"
	dbsql "github.com/dnonakolesax/cccad-locks/internal/db/sql"
	"github.com/dnonakolesax/cccad-locks/internal/s3"
)

type Components struct {
	redis *dbredis.Client
	pgsql *dbsql.PGXWorker
	s3    *s3.Worker
}

func (a *App) SetupComponents() error {
	/************************************************/
	/*               SQL DB CONNECTION              */
	/************************************************/
	a.initLogger.InfoContext(context.Background(), "Starting SQL DB connection")
	psqlConn, err := dbsql.NewPGXConn(*a.configs.PSQL, a.loggers.Infra)
	a.initLogger.InfoContext(context.Background(), "SQL DB connection established")

	if err != nil {
		a.initLogger.ErrorContext(context.Background(), "Error connecting to database",
			slog.String(consts.ErrorLoggerKey, err.Error()))
		return err
	}

	psqlWorker, err := dbsql.NewPGXWorker(psqlConn, a.health.Postgres, a.configs.UpdateChans.PSQLCredentials)

	if err != nil {
		a.initLogger.ErrorContext(context.Background(), "Error creating pgsql worker",
			slog.String(consts.ErrorLoggerKey, err.Error()))
		return err
	}

	/************************************************/
	/*              REDIS DB CONNECTION             */
	/************************************************/
	a.initLogger.InfoContext(context.Background(), "Starting REDIS DB connection")
	redisClient, err := dbredis.NewClient(
		a.configs.Redis,
		a.health.Redis,
		a.loggers.Infra,
		a.configs.UpdateChans.RedisPassword,
	)
	a.initLogger.InfoContext(context.Background(), "REDIS DB connection established")

	if err != nil {
		a.initLogger.ErrorContext(context.Background(), "Error connecting to redis",
			slog.String(consts.ErrorLoggerKey, err.Error()))
		return err
	}

	/************************************************/
	/*              S3 CLIENT SETUP               */
	/************************************************/

	s3Worker, err := s3.NewWorker(a.configs.S3)

	if err != nil {
		a.initLogger.ErrorContext(context.Background(), "Error creating S3 worker",
			slog.String(consts.ErrorLoggerKey, err.Error()))
		return err
	}

	a.initLogger.InfoContext(context.Background(), "Created S3 client")
	a.components = &Components{
		pgsql: psqlWorker,
		redis: redisClient,
		s3:    s3Worker,
	}
	return nil
}
