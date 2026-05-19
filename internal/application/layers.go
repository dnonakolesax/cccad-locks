package application

import (
	locksDelivery "github.com/dnonakolesax/cccad-locks/internal/delivery/locks/v1"
	operationsDelivery "github.com/dnonakolesax/cccad-locks/internal/delivery/operations/v1"
	permissionsDelivery "github.com/dnonakolesax/cccad-locks/internal/delivery/permissions/v1"
	sketchesDelivery "github.com/dnonakolesax/cccad-locks/internal/delivery/sketches/v1"
	solverDelivery "github.com/dnonakolesax/cccad-locks/internal/delivery/solver/v1"
)

type Layers struct {
	PermissionsHTTP *permissionsDelivery.PermissionsHandler
	LocksHTTP       *locksDelivery.LocksHandler
	OperationsHTTP  *operationsDelivery.OperationsHandler
	SketchesHTTP    *sketchesDelivery.SketchesHandler
	SolverHTTP      *solverDelivery.SolverHandler
}

func (a *App) SetupLayers() error {
	a.layers = &Layers{
		PermissionsHTTP: permissionsDelivery.NewPermissionsHandler(a.components.permissions),
		// LocksHTTP:       locksDelivery.NewLocksHandler(a.components.locks),
		// OperationsHTTP:  operationsDelivery.NewOperationsHandler(a.components.operations),
		// SketchesHTTP:    sketchesDelivery.NewSketchesHandler(a.components.sketches),
		// SolverHTTP:      solverDelivery.NewSolverHandler(a.components.solver),
	}

	return nil
}
