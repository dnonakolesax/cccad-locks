package application

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/dnonakolesax/cccad-locks/internal/auth"
	"github.com/dnonakolesax/cccad-locks/internal/consts"
	dbredis "github.com/dnonakolesax/cccad-locks/internal/db/redis"
	dbsql "github.com/dnonakolesax/cccad-locks/internal/db/sql"
	"github.com/dnonakolesax/cccad-locks/internal/geometry"
	operationsRepo "github.com/dnonakolesax/cccad-locks/internal/repository/operations"
	parts3dRepo "github.com/dnonakolesax/cccad-locks/internal/repository/parts3d"
	permissionsRepo "github.com/dnonakolesax/cccad-locks/internal/repository/permissions"
	locksRepo "github.com/dnonakolesax/cccad-locks/internal/repository/redis/locks"
	sketchesRepo "github.com/dnonakolesax/cccad-locks/internal/repository/sketches"
	workspacesRepo "github.com/dnonakolesax/cccad-locks/internal/repository/workspaces"
	"github.com/dnonakolesax/cccad-locks/internal/s3"
	locksService "github.com/dnonakolesax/cccad-locks/internal/service/locks"
	operationsService "github.com/dnonakolesax/cccad-locks/internal/service/operations"
	parts3dService "github.com/dnonakolesax/cccad-locks/internal/service/parts3d"
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
	geometry    *geometry.Client
	locks       *locksService.Service
	operations  *operationsService.Service
	parts3d     *parts3dService.Service
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
	solverClient, err := solver.NewClient(a.configs.Solver, a.loggers.GRPC, a.metrics.GRPCClient)
	if err != nil {
		a.initLogger.ErrorContext(context.Background(), "Error creating solver grpc client",
			slog.String(consts.ErrorLoggerKey, err.Error()))
		return err
	}
	if err := solverClient.Ping(context.Background()); err != nil {
		_ = solverClient.Close()
		err = fmt.Errorf("ping solver grpc service: %w", err)
		a.initLogger.ErrorContext(context.Background(), "Error pinging solver grpc service",
			slog.String("address", a.configs.Solver.Address),
			slog.String(consts.ErrorLoggerKey, err.Error()))
		return err
	}
	a.initLogger.InfoContext(context.Background(), "Solver grpc service is ready",
		slog.String("address", a.configs.Solver.Address))

	/************************************************/
	/*             GEOMETRY GRPC CLIENT             */
	/************************************************/
	geometryClient, err := geometry.NewClient(a.configs.Geometry, a.loggers.GRPC, a.metrics.GRPCClient)
	if err != nil {
		_ = solverClient.Close()
		a.initLogger.ErrorContext(context.Background(), "Error creating geometry grpc client",
			slog.String(consts.ErrorLoggerKey, err.Error()))
		return err
	}
	if err := geometryClient.Ping(context.Background()); err != nil {
		_ = geometryClient.Close()
		_ = solverClient.Close()
		err = fmt.Errorf("ping geometry grpc service: %w", err)
		a.initLogger.ErrorContext(context.Background(), "Error pinging geometry grpc service",
			slog.String("address", a.configs.Geometry.Address),
			slog.String(consts.ErrorLoggerKey, err.Error()))
		return err
	}
	a.initLogger.InfoContext(context.Background(), "Geometry grpc service is ready",
		slog.String("address", a.configs.Geometry.Address))

	/************************************************/
	/*               AUTH GRPC CLIENT               */
	/************************************************/
	authClient, err := auth.NewClient(a.configs.Auth, a.loggers.GRPC, a.metrics.GRPCClient)
	if err != nil {
		_ = geometryClient.Close()
		_ = solverClient.Close()
		a.initLogger.ErrorContext(context.Background(), "Error creating auth grpc client",
			slog.String(consts.ErrorLoggerKey, err.Error()))
		return err
	}
	if err := authClient.Ping(context.Background()); err != nil {
		_ = authClient.Close()
		_ = geometryClient.Close()
		_ = solverClient.Close()
		err = fmt.Errorf("ping auth grpc service: %w", err)
		a.initLogger.ErrorContext(context.Background(), "Error pinging auth grpc service",
			slog.String("address", a.configs.Auth.Address),
			slog.String(consts.ErrorLoggerKey, err.Error()))
		return err
	}
	a.initLogger.InfoContext(context.Background(), "Auth grpc service is ready",
		slog.String("address", a.configs.Auth.Address))

	a.components = &Components{
		pgsql:       psqlWorker,
		redis:       redisClient,
		s3:          s3Worker,
		solver:      solverClient,
		geometry:    geometryClient,
		locks:       locksService.NewService(locksRepo.NewRepository(redisClient)),
		operations:  operationsService.NewServiceWithSolver(operationsRepo.NewRepository(psqlWorker), solverClient),
		parts3d:     parts3dService.NewService(parts3dRepo.NewRepository(psqlWorker)),
		permissions: permissionsService.NewService(permissionsRepo.NewRepository(psqlWorker)),
		realtime: realtimeService.NewService(
			permissionsService.NewService(permissionsRepo.NewRepository(psqlWorker)),
			sketchesService.NewService(sketchesRepo.NewRepository(psqlWorker)),
			locksService.NewService(locksRepo.NewRepository(redisClient)),
			operationsService.NewServiceWithSolver(operationsRepo.NewRepository(psqlWorker), solverClient),
		),
		sketches:   sketchesService.NewService(sketchesRepo.NewRepository(psqlWorker)),
		solverSvc:  solverService.NewService(sketchesRepo.NewRepository(psqlWorker), solverClient),
		workspaces: workspacesService.NewService(workspacesRepo.NewRepository(psqlWorker)),
		auth:       authClient,
	}
	return nil
}
