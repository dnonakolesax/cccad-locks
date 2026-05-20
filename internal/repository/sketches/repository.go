package sketches

import (
	"context"
	"encoding/json"
	"fmt"
	"time"

	dbsql "github.com/dnonakolesax/cccad-locks/internal/db/sql"
	"github.com/dnonakolesax/cccad-locks/internal/model"
)

const (
	createSketchRequest         = "sketch_create"
	getSketchRequest            = "sketch_get"
	updateSketchMetadataRequest = "sketch_update_metadata"
	deleteSketchRequest         = "sketch_delete"
)

type Repository struct {
	db *dbsql.PGXWorker
}

func NewRepository(db *dbsql.PGXWorker) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(
	ctx context.Context,
	workspaceID string,
	request *model.CreateSketchRequest,
	createdByUserID string,
) (*model.SketchMetadata, error) {
	sqlRequest, err := r.db.Request(createSketchRequest)
	if err != nil {
		return nil, fmt.Errorf("create sketch request: %w", err)
	}

	rows, err := r.db.Query(ctx, sqlRequest, workspaceID, request.Name, request.Unit, createdByUserID)
	if err != nil {
		return nil, fmt.Errorf("create sketch: %w", err)
	}

	if !rows.Next() {
		if err := rows.Close(); err != nil {
			return nil, fmt.Errorf("create sketch rows: %w", err)
		}
		return nil, fmt.Errorf("create sketch returned no rows")
	}

	metadata, err := scanMetadata(rows)
	if err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("create sketch rows: %w", err)
	}

	return metadata, nil
}

func (r *Repository) Get(ctx context.Context, sketchID string) (*model.SketchDocument, error) {
	sqlRequest, err := r.db.Request(getSketchRequest)
	if err != nil {
		return nil, fmt.Errorf("get sketch request: %w", err)
	}

	rows, err := r.db.Query(ctx, sqlRequest, sketchID)
	if err != nil {
		return nil, fmt.Errorf("get sketch: %w", err)
	}

	if !rows.Next() {
		if err := rows.Close(); err != nil {
			return nil, fmt.Errorf("get sketch rows: %w", err)
		}
		return nil, fmt.Errorf("get sketch returned no rows")
	}

	document, err := scanDocument(rows)
	if err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("get sketch rows: %w", err)
	}

	return document, nil
}

func (r *Repository) UpdateMetadata(
	ctx context.Context,
	sketchID string,
	request *model.UpdateSketchMetadataRequest,
) (*model.SketchMetadata, error) {
	sqlRequest, err := r.db.Request(updateSketchMetadataRequest)
	if err != nil {
		return nil, fmt.Errorf("update sketch metadata request: %w", err)
	}

	rows, err := r.db.Query(ctx, sqlRequest, sketchID, request.Name, request.Unit)
	if err != nil {
		return nil, fmt.Errorf("update sketch metadata: %w", err)
	}

	if !rows.Next() {
		if err := rows.Close(); err != nil {
			return nil, fmt.Errorf("update sketch metadata rows: %w", err)
		}
		return nil, fmt.Errorf("update sketch metadata returned no rows")
	}

	metadata, err := scanMetadata(rows)
	if err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("update sketch metadata rows: %w", err)
	}

	return metadata, nil
}

func (r *Repository) Delete(ctx context.Context, sketchID string) error {
	sqlRequest, err := r.db.Request(deleteSketchRequest)
	if err != nil {
		return fmt.Errorf("delete sketch request: %w", err)
	}

	rows, err := r.db.Query(ctx, sqlRequest, sketchID)
	if err != nil {
		return fmt.Errorf("delete sketch: %w", err)
	}

	if !rows.Next() {
		if err := rows.Close(); err != nil {
			return fmt.Errorf("delete sketch rows: %w", err)
		}
		return fmt.Errorf("delete sketch returned no rows")
	}

	if err := rows.Close(); err != nil {
		return fmt.Errorf("delete sketch rows: %w", err)
	}

	return nil
}

func scanDocument(rows *dbsql.PGXResponse) (*model.SketchDocument, error) {
	var document model.SketchDocument
	var entities []byte
	var constraints []byte
	var dimensions []byte
	var groups []byte
	var solveStatus []byte
	var conflicts []byte

	if err := rows.Scan(
		&document.ID,
		&document.WorkspaceID,
		&document.Name,
		&document.CreatedByUserID,
		&document.Unit,
		&document.Version,
		&entities,
		&constraints,
		&dimensions,
		&groups,
		&solveStatus,
		&conflicts,
	); err != nil {
		return nil, fmt.Errorf("scan sketch document: %w", err)
	}

	if err := json.Unmarshal(entities, &document.Entities); err != nil {
		return nil, fmt.Errorf("scan sketch entities: %w", err)
	}
	if err := json.Unmarshal(constraints, &document.Constraints); err != nil {
		return nil, fmt.Errorf("scan sketch constraints: %w", err)
	}
	if err := json.Unmarshal(dimensions, &document.Dimensions); err != nil {
		return nil, fmt.Errorf("scan sketch dimensions: %w", err)
	}
	if err := json.Unmarshal(groups, &document.Groups); err != nil {
		return nil, fmt.Errorf("scan sketch groups: %w", err)
	}
	document.SolveStatus = append(document.SolveStatus, solveStatus...)
	if err := json.Unmarshal(conflicts, &document.Conflicts); err != nil {
		return nil, fmt.Errorf("scan sketch conflicts: %w", err)
	}

	return &document, nil
}

func scanMetadata(rows *dbsql.PGXResponse) (*model.SketchMetadata, error) {
	var metadata model.SketchMetadata
	var createdAt time.Time
	var updatedAt time.Time

	if err := rows.Scan(
		&metadata.ID,
		&metadata.WorkspaceID,
		&metadata.Name,
		&metadata.CreatedByUserID,
		&metadata.Unit,
		&metadata.Version,
		&createdAt,
		&updatedAt,
	); err != nil {
		return nil, fmt.Errorf("scan sketch metadata: %w", err)
	}

	metadata.CreatedAt = createdAt.UTC().Format(time.RFC3339Nano)
	metadata.UpdatedAt = updatedAt.UTC().Format(time.RFC3339Nano)

	return &metadata, nil
}
