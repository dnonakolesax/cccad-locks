package operations

import (
	"context"
	"fmt"
	"time"

	dbsql "github.com/dnonakolesax/cccad-locks/internal/db/sql"
	"github.com/dnonakolesax/cccad-locks/internal/model"
	"github.com/mailru/easyjson"
)

const (
	listOperationsRequest      = "operations_list"
	getOperationStateRequest   = "operations_get_submit_state"
	submitOperationRequestName = "operations_submit"
)

type Repository struct {
	db *dbsql.PGXWorker
}

func NewRepository(db *dbsql.PGXWorker) *Repository {
	return &Repository{db: db}
}

func (r *Repository) List(
	ctx context.Context,
	userID string,
	sketchID string,
	afterVersion int64,
	limit int,
) (*model.SketchOperationPage, error) {
	sqlRequest, err := r.db.Request(listOperationsRequest)
	if err != nil {
		return nil, fmt.Errorf("list operations request: %w", err)
	}

	rows, err := r.db.Query(ctx, sqlRequest, sketchID, userID, afterVersion, limit)
	if err != nil {
		return nil, fmt.Errorf("list operations: %w", err)
	}

	page := &model.SketchOperationPage{
		SketchID:             sketchID,
		FromVersionExclusive: afterVersion,
		ToVersion:            afterVersion,
		Ops:                  make([]model.CommittedOperation, 0),
	}

	for rows.Next() {
		operation, scanErr := scanCommittedOperation(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		page.Ops = append(page.Ops, *operation)
		page.ToVersion = operation.Version
	}
	if closeErr := rows.Close(); closeErr != nil {
		return nil, fmt.Errorf("list operations rows: %w", closeErr)
	}

	return page, nil
}

func (r *Repository) GetSubmitState(
	ctx context.Context,
	userID string,
	sketchID string,
) (*model.SubmitState, error) {
	sqlRequest, err := r.db.Request(getOperationStateRequest)
	if err != nil {
		return nil, fmt.Errorf("get submit state request: %w", err)
	}

	rows, err := r.db.Query(ctx, sqlRequest, sketchID, userID)
	if err != nil {
		return nil, fmt.Errorf("get submit state: %w", err)
	}
	if !rows.Next() {
		if closeErr := rows.Close(); closeErr != nil {
			return nil, fmt.Errorf("get submit state rows: %w", closeErr)
		}
		return nil, fmt.Errorf("sketch %q is not editable or does not exist", sketchID)
	}

	state, err := scanSubmitState(rows)
	if err != nil {
		return nil, err
	}
	if closeErr := rows.Close(); closeErr != nil {
		return nil, fmt.Errorf("get submit state rows: %w", closeErr)
	}

	return state, nil
}

func (r *Repository) Submit(
	ctx context.Context,
	userID string,
	sketchID string,
	request model.SubmitCommitRequest,
) (*model.SubmitCommitResult, error) {
	sqlRequest, err := r.db.Request(submitOperationRequestName)
	if err != nil {
		return nil, fmt.Errorf("submit operation request: %w", err)
	}

	rows, err := r.db.Query(
		ctx,
		sqlRequest,
		sketchID,
		userID,
		request.ClientOpID,
		request.BaseVersion,
		request.OpType,
		request.Payload,
		request.Patch,
		request.GraphState,
		request.MaterializedGeometry,
		request.SolveStatus,
	)
	if err != nil {
		return nil, fmt.Errorf("submit operation: %w", err)
	}
	if !rows.Next() {
		if closeErr := rows.Close(); closeErr != nil {
			return nil, fmt.Errorf("submit operation rows: %w", closeErr)
		}
		return nil, fmt.Errorf("submit operation returned no rows")
	}

	result, err := scanSubmitResult(rows)
	if err != nil {
		return nil, err
	}
	if closeErr := rows.Close(); closeErr != nil {
		return nil, fmt.Errorf("submit operation rows: %w", closeErr)
	}

	return result, nil
}

func scanCommittedOperation(rows *dbsql.PGXResponse) (*model.CommittedOperation, error) {
	var operation model.CommittedOperation
	var clientOpID string
	var createdAt time.Time
	var payload []byte

	if err := rows.Scan(
		&operation.ID,
		&operation.SketchID,
		&operation.Version,
		&operation.ActorUserID,
		&clientOpID,
		&createdAt,
		&payload,
	); err != nil {
		return nil, fmt.Errorf("scan committed operation: %w", err)
	}

	operation.ClientOpID = &clientOpID
	operation.CreatedAt = createdAt.UTC().Format(time.RFC3339Nano)
	operation.Payload = append(operation.Payload, payload...)

	return &operation, nil
}

func scanSubmitState(rows *dbsql.PGXResponse) (*model.SubmitState, error) {
	var state model.SubmitState
	var graphState []byte
	var materializedGeometry []byte
	var solveStatus []byte

	if err := rows.Scan(
		&state.Version,
		&graphState,
		&materializedGeometry,
		&solveStatus,
	); err != nil {
		return nil, fmt.Errorf("scan submit state: %w", err)
	}

	state.GraphState = easyjson.RawMessage(append([]byte(nil), graphState...))
	state.MaterializedGeometry = easyjson.RawMessage(append([]byte(nil), materializedGeometry...))
	state.SolveStatus = easyjson.RawMessage(append([]byte(nil), solveStatus...))

	return &state, nil
}

func scanSubmitResult(rows *dbsql.PGXResponse) (*model.SubmitCommitResult, error) {
	var result model.SubmitCommitResult
	if err := rows.Scan(
		&result.Status,
		&result.OpID,
		&result.Version,
		&result.CurrentVersion,
		&result.Duplicate,
	); err != nil {
		return nil, fmt.Errorf("scan submit operation result: %w", err)
	}

	return &result, nil
}
