package application

import (
	"context"
	"log/slog"

	"github.com/dnonakolesax/cccad-locks/internal/auth"
	"github.com/dnonakolesax/cccad-locks/internal/consts"
	dbredis "github.com/dnonakolesax/cccad-locks/internal/db/redis"
	dbsql "github.com/dnonakolesax/cccad-locks/internal/db/sql"
	operationsRepo "github.com/dnonakolesax/cccad-locks/internal/repository/operations"
	permissionsRepo "github.com/dnonakolesax/cccad-locks/internal/repository/permissions"
	locksRepo "github.com/dnonakolesax/cccad-locks/internal/repository/redis/locks"
	sketchesRepo "github.com/dnonakolesax/cccad-locks/internal/repository/sketches"
	workspacesRepo "github.com/dnonakolesax/cccad-locks/internal/repository/workspaces"
	"github.com/dnonakolesax/cccad-locks/internal/s3"
	locksService "github.com/dnonakolesax/cccad-locks/internal/service/locks"
	operationsService "github.com/dnonakolesax/cccad-locks/internal/service/operations"
	permissionsService "github.com/dnonakolesax/cccad-locks/internal/service/permissions"
	realtimeService "github.com/dnonakolesax/cccad-locks/internal/service/realtime"
	sketchesService "github.com/dnonakolesax/cccad-locks/internal/service/sketches"
	solverService "github.com/dnonakolesax/cccad-locks/internal/service/solver"
	workspacesService "github.com/dnonakolesax/cccad-locks/internal/service/workspaces"
	"github.com/dnonakolesax/cccad-locks/internal/solver"
)

type Components struct {
	redis       *dbredis.Client
	pgsql       *dbsql.PGXWorker
	s3          *s3.Worker
	solver      *solver.Client
	locks       *locksService.Service
	operations  *operationsService.Service
	permissions *permissionsService.Service
	realtime    *realtimeService.Service
	sketches    *sketchesService.Service
	solverSvc   *solverService.Service
	workspaces  *workspacesService.Service
	auth        *auth.Client
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

	/************************************************/
	/*              SOLVER GRPC CLIENT              */
	/************************************************/
	solverClient, err := solver.NewClient(a.configs.Solver, a.loggers.GRPC)
	if err != nil {
		a.initLogger.ErrorContext(context.Background(), "Error creating solver grpc client",
			slog.String(consts.ErrorLoggerKey, err.Error()))
		return err
	}

	/************************************************/
	/*               AUTH GRPC CLIENT               */
	/************************************************/
	authClient, err := auth.NewClient(a.configs.Auth, a.loggers.GRPC)
	if err != nil {
		a.initLogger.ErrorContext(context.Background(), "Error creating auth grpc client",
			slog.String(consts.ErrorLoggerKey, err.Error()))
		return err
	}

	a.components = &Components{
		pgsql:       psqlWorker,
		redis:       redisClient,
		s3:          s3Worker,
		solver:      solverClient,
		locks:       locksService.NewService(locksRepo.NewRepository(redisClient)),
		operations:  operationsService.NewService(operationsRepo.NewRepository(psqlWorker)),
		permissions: permissionsService.NewService(permissionsRepo.NewRepository(psqlWorker)),
		realtime: realtimeService.NewService(
			permissionsService.NewService(permissionsRepo.NewRepository(psqlWorker)),
			sketchesService.NewService(sketchesRepo.NewRepository(psqlWorker)),
		),
		sketches:   sketchesService.NewService(sketchesRepo.NewRepository(psqlWorker)),
		solverSvc:  solverService.NewService(sketchesRepo.NewRepository(psqlWorker), solverClient),
		workspaces: workspacesService.NewService(workspacesRepo.NewRepository(psqlWorker)),
		auth:       authClient,
	}
	return nil
}
