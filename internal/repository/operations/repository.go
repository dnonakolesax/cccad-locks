package operations

import (
	"context"
	"fmt"
	"time"

	dbsql "github.com/dnonakolesax/cccad-locks/internal/db/sql"
	"github.com/dnonakolesax/cccad-locks/internal/model"
)

const listOperationsRequest = "operations_list"

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
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("list operations rows: %w", err)
	}

	return page, nil
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
