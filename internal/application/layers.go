package application

import (
	"context"

	commentsDelivery "github.com/dnonakolesax/cccad-locks/internal/delivery/comments/v1"
	locksDelivery "github.com/dnonakolesax/cccad-locks/internal/delivery/locks/v1"
	operationsDelivery "github.com/dnonakolesax/cccad-locks/internal/delivery/operations/v1"
	parts3dDelivery "github.com/dnonakolesax/cccad-locks/internal/delivery/parts3d/v1"
	permissionsDelivery "github.com/dnonakolesax/cccad-locks/internal/delivery/permissions/v1"
	realtimeDelivery "github.com/dnonakolesax/cccad-locks/internal/delivery/realtime/v1"
	sketchesDelivery "github.com/dnonakolesax/cccad-locks/internal/delivery/sketches/v1"
	solverDelivery "github.com/dnonakolesax/cccad-locks/internal/delivery/solver/v1"
	workspacesDelivery "github.com/dnonakolesax/cccad-locks/internal/delivery/workspaces/v1"
	"github.com/dnonakolesax/cccad-locks/internal/model"
	parts3dRepo "github.com/dnonakolesax/cccad-locks/internal/repository/parts3d"
	realtimeService "github.com/dnonakolesax/cccad-locks/internal/service/realtime"
)

type Layers struct {
	CommentsHTTP    *commentsDelivery.CommentsHandler
	PermissionsHTTP *permissionsDelivery.PermissionsHandler
	LocksHTTP       *locksDelivery.LocksHandler
	OperationsHTTP  *operationsDelivery.OperationsHandler
	Parts3DHTTP     *parts3dDelivery.Parts3DHandler
	Parts3DWS       *parts3dDelivery.Parts3DWSHandler
	SketchesHTTP    *sketchesDelivery.SketchesHandler
	RealtimeWS      *realtimeDelivery.Handler
	SolverHTTP      *solverDelivery.SolverHandler
	WorkspacesHTTP  *workspacesDelivery.WorkspacesHandler
}

func (a *App) SetupLayers() error {
	a.layers = &Layers{
		CommentsHTTP:    commentsDelivery.NewCommentsHandler(a.components.comments),
		PermissionsHTTP: permissionsDelivery.NewPermissionsHandler(a.components.permissions),
		LocksHTTP:       locksDelivery.NewLocksHandler(a.components.locks),
		OperationsHTTP:  operationsDelivery.NewOperationsHandler(a.components.operations),
		Parts3DHTTP:     parts3dDelivery.NewParts3DHandler(a.components.parts3d),
		Parts3DWS: parts3dDelivery.NewParts3DWSHandler(
			parts3dDelivery.WithParts3DWSLogger(a.loggers.HTTP),
			parts3dDelivery.WithParts3DFeatureProcessor(
				a.components.geometry,
				parts3dRepo.NewRepository(a.components.pgsql),
			),
		),
		SketchesHTTP: sketchesDelivery.NewSketchesHandler(a.components.sketches),
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
