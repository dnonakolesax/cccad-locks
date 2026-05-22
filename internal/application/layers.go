package application

import (
	"context"

	locksDelivery "github.com/dnonakolesax/cccad-locks/internal/delivery/locks/v1"
	operationsDelivery "github.com/dnonakolesax/cccad-locks/internal/delivery/operations/v1"
	permissionsDelivery "github.com/dnonakolesax/cccad-locks/internal/delivery/permissions/v1"
	realtimeDelivery "github.com/dnonakolesax/cccad-locks/internal/delivery/realtime/v1"
	sketchesDelivery "github.com/dnonakolesax/cccad-locks/internal/delivery/sketches/v1"
	solverDelivery "github.com/dnonakolesax/cccad-locks/internal/delivery/solver/v1"
	workspacesDelivery "github.com/dnonakolesax/cccad-locks/internal/delivery/workspaces/v1"
	"github.com/dnonakolesax/cccad-locks/internal/model"
	realtimeService "github.com/dnonakolesax/cccad-locks/internal/service/realtime"
)

type Layers struct {
	PermissionsHTTP *permissionsDelivery.PermissionsHandler
	LocksHTTP       *locksDelivery.LocksHandler
	OperationsHTTP  *operationsDelivery.OperationsHandler
	SketchesHTTP    *sketchesDelivery.SketchesHandler
	RealtimeWS      *realtimeDelivery.Handler
	SolverHTTP      *solverDelivery.SolverHandler
	WorkspacesHTTP  *workspacesDelivery.WorkspacesHandler
}

func (a *App) SetupLayers() error {
	a.layers = &Layers{
		PermissionsHTTP: permissionsDelivery.NewPermissionsHandler(a.components.permissions),
		LocksHTTP:       locksDelivery.NewLocksHandler(a.components.locks),
		OperationsHTTP:  operationsDelivery.NewOperationsHandler(a.components.operations),
		SketchesHTTP:    sketchesDelivery.NewSketchesHandler(a.components.sketches),
		RealtimeWS: realtimeDelivery.NewHandler(
			realtimeAdapter{service: a.components.realtime},
			nil,
			realtimeDelivery.WithLogger(a.loggers.HTTP),
			realtimeDelivery.WithRoutePrefix("/"),
		),
		SolverHTTP:     solverDelivery.NewSolverHandler(a.components.solverSvc),
		WorkspacesHTTP: workspacesDelivery.NewWorkspacesHandler(a.components.workspaces),
	}

	return nil
}

type realtimeAdapter struct {
	service *realtimeService.Service
}

func (a realtimeAdapter) OpenConnection(
	ctx context.Context,
	req model.OpenRealtimeSessionRequest,
) (realtimeDelivery.Connection, error) {
	return a.service.OpenConnection(ctx, req)
}
