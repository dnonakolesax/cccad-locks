package sketches

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	dbsql "github.com/dnonakolesax/cccad-locks/internal/db/sql"
	"github.com/dnonakolesax/cccad-locks/internal/model"
)

const (
	createSketchRequest          = "sketch_create"
	listAvailableSketchesRequest = "sketch_list_available"
	getSketchRequest             = "sketch_get"
	getSketchSnapshotRequest     = "sketch_snapshot_get"
	revertSketchToVersionRequest = "sketch_revert_to_version"
	updateSketchMetadataRequest  = "sketch_update_metadata"
	deleteSketchRequest          = "sketch_delete"
	getDeletedEntityGeometry     = "sketch_deleted_entity_geometry_get"
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

	plane, err := json.Marshal(request.Plane)
	if err != nil {
		return nil, fmt.Errorf("marshal sketch plane: %w", err)
	}

	rows, err := r.db.Query(ctx, sqlRequest, workspaceID, request.Name, request.Unit, string(plane), createdByUserID)
	if err != nil {
		return nil, fmt.Errorf("create sketch: %w", err)
	}

	if !rows.Next() {
		if closeErr := rows.Close(); closeErr != nil {
			return nil, fmt.Errorf("create sketch rows: %w", closeErr)
		}
		return nil, errors.New("create sketch returned no rows")
	}

	metadata, err := scanMetadata(rows)
	if err != nil {
		return nil, err
	}
	if closeErr := rows.Close(); closeErr != nil {
		return nil, fmt.Errorf("create sketch rows: %w", closeErr)
	}

	return metadata, nil
}

func (r *Repository) ListAvailable(ctx context.Context, userID string) ([]model.AvailableSketch, error) {
	sqlRequest, err := r.db.Request(listAvailableSketchesRequest)
	if err != nil {
		return nil, fmt.Errorf("list available sketches request: %w", err)
	}

	rows, err := r.db.Query(ctx, sqlRequest, userID)
	if err != nil {
		return nil, fmt.Errorf("list available sketches: %w", err)
	}

	sketches := make([]model.AvailableSketch, 0)
	for rows.Next() {
		sketch, scanErr := scanAvailableSketch(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		sketches = append(sketches, *sketch)
	}
	if closeErr := rows.Close(); closeErr != nil {
		return nil, fmt.Errorf("list available sketches rows: %w", closeErr)
	}

	return sketches, nil
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
		if closeErr := rows.Close(); closeErr != nil {
			return nil, fmt.Errorf("get sketch rows: %w", closeErr)
		}
		return nil, errors.New("get sketch returned no rows")
	}

	document, err := scanDocument(rows)
	if err != nil {
		return nil, err
	}
	if closeErr := rows.Close(); closeErr != nil {
		return nil, fmt.Errorf("get sketch rows: %w", closeErr)
	}

	return document, nil
}

func (r *Repository) Snapshot(ctx context.Context, sketchID string, version int64, userID string) (*model.SketchSnapshot, error) {
	sqlRequest, err := r.db.Request(getSketchSnapshotRequest)
	if err != nil {
		return nil, fmt.Errorf("get sketch snapshot request: %w", err)
	}

	rows, err := r.db.Query(ctx, sqlRequest, sketchID, version, userID)
	if err != nil {
		return nil, fmt.Errorf("get sketch snapshot: %w", err)
	}

	if !rows.Next() {
		if closeErr := rows.Close(); closeErr != nil {
			return nil, fmt.Errorf("get sketch snapshot rows: %w", closeErr)
		}
		return nil, errors.New("get sketch snapshot returned no rows")
	}

	snapshot, err := scanSnapshot(rows)
	if err != nil {
		return nil, err
	}
	if closeErr := rows.Close(); closeErr != nil {
		return nil, fmt.Errorf("get sketch snapshot rows: %w", closeErr)
	}

	return snapshot, nil
}

func (r *Repository) DeletedEntityGeometry(
	ctx context.Context,
	sketchID string,
	entityID string,
	userID string,
) (*model.DeletedSketchEntityGeometry, error) {
	sqlRequest, err := r.db.Request(getDeletedEntityGeometry)
	if err != nil {
		return nil, fmt.Errorf("get deleted sketch entity geometry request: %w", err)
	}

	rows, err := r.db.Query(ctx, sqlRequest, sketchID, entityID, userID)
	if err != nil {
		return nil, fmt.Errorf("get deleted sketch entity geometry: %w", err)
	}

	if !rows.Next() {
		if closeErr := rows.Close(); closeErr != nil {
			return nil, fmt.Errorf("get deleted sketch entity geometry rows: %w", closeErr)
		}
		return nil, errors.New("get deleted sketch entity geometry returned no rows")
	}

	geometry, err := scanDeletedEntityGeometry(rows)
	if err != nil {
		return nil, err
	}
	if closeErr := rows.Close(); closeErr != nil {
		return nil, fmt.Errorf("get deleted sketch entity geometry rows: %w", closeErr)
	}

	return geometry, nil
}

func (r *Repository) RevertToVersion(ctx context.Context, sketchID string, version int64, userID string) (*model.SketchDocument, error) {
	sqlRequest, err := r.db.Request(revertSketchToVersionRequest)
	if err != nil {
		return nil, fmt.Errorf("revert sketch to version request: %w", err)
	}

	rows, err := r.db.Query(ctx, sqlRequest, sketchID, version, userID)
	if err != nil {
		return nil, fmt.Errorf("revert sketch to version: %w", err)
	}

	if !rows.Next() {
		if closeErr := rows.Close(); closeErr != nil {
			return nil, fmt.Errorf("revert sketch to version rows: %w", closeErr)
		}
		return nil, errors.New("revert sketch to version returned no rows")
	}

	document, err := scanDocument(rows)
	if err != nil {
		return nil, err
	}
	if closeErr := rows.Close(); closeErr != nil {
		return nil, fmt.Errorf("revert sketch to version rows: %w", closeErr)
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
		if closeErr := rows.Close(); closeErr != nil {
			return nil, fmt.Errorf("update sketch metadata rows: %w", closeErr)
		}
		return nil, errors.New("update sketch metadata returned no rows")
	}

	metadata, err := scanMetadata(rows)
	if err != nil {
		return nil, err
	}
	if closeErr := rows.Close(); closeErr != nil {
		return nil, fmt.Errorf("update sketch metadata rows: %w", closeErr)
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
		if closeErr := rows.Close(); closeErr != nil {
			return fmt.Errorf("delete sketch rows: %w", closeErr)
		}
		return errors.New("delete sketch returned no rows")
	}

	if closeErr := rows.Close(); closeErr != nil {
		return fmt.Errorf("delete sketch rows: %w", closeErr)
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
	var profiles []byte
	var conflicts []byte
	var plane []byte

	if err := rows.Scan(
		&document.ID,
		&document.WorkspaceID,
		&document.Name,
		&document.CreatedByUserID,
		&document.Unit,
		&plane,
		&document.Version,
		&entities,
		&constraints,
		&dimensions,
		&groups,
		&solveStatus,
		&profiles,
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
	if err := json.Unmarshal(plane, &document.Plane); err != nil {
		return nil, fmt.Errorf("scan sketch plane: %w", err)
	}
	document.SolveStatus = append(document.SolveStatus, solveStatus...)
	if err := json.Unmarshal(profiles, &document.Profiles); err != nil {
		return nil, fmt.Errorf("scan sketch profiles: %w", err)
	}
	if err := json.Unmarshal(conflicts, &document.Conflicts); err != nil {
		return nil, fmt.Errorf("scan sketch conflicts: %w", err)
	}

	return &document, nil
}

func scanAvailableSketch(rows *dbsql.PGXResponse) (*model.AvailableSketch, error) {
	var sketch model.AvailableSketch
	var createdAt time.Time
	var updatedAt time.Time
	var plane []byte

	if err := rows.Scan(
		&sketch.ID,
		&sketch.WorkspaceID,
		&sketch.Name,
		&sketch.CreatedByUserID,
		&sketch.Unit,
		&plane,
		&sketch.Version,
		&sketch.Role,
		&createdAt,
		&updatedAt,
	); err != nil {
		return nil, fmt.Errorf("scan available sketch: %w", err)
	}

	sketch.CreatedAt = createdAt.UTC().Format(time.RFC3339Nano)
	sketch.UpdatedAt = updatedAt.UTC().Format(time.RFC3339Nano)
	if err := json.Unmarshal(plane, &sketch.Plane); err != nil {
		return nil, fmt.Errorf("scan available sketch plane: %w", err)
	}

	return &sketch, nil
}

func scanSnapshot(rows *dbsql.PGXResponse) (*model.SketchSnapshot, error) {
	var snapshot model.SketchSnapshot
	var graphState []byte
	var materializedGeometry []byte
	var solveStatus []byte
	var createdAt time.Time

	if err := rows.Scan(
		&snapshot.SketchID,
		&snapshot.Version,
		&graphState,
		&materializedGeometry,
		&solveStatus,
		&createdAt,
	); err != nil {
		return nil, fmt.Errorf("scan sketch snapshot: %w", err)
	}

	snapshot.GraphState = append(snapshot.GraphState, graphState...)
	snapshot.MaterializedGeometry = append(snapshot.MaterializedGeometry, materializedGeometry...)
	snapshot.SolveStatus = append(snapshot.SolveStatus, solveStatus...)
	snapshot.CreatedAt = createdAt.UTC().Format(time.RFC3339Nano)

	return &snapshot, nil
}

func scanDeletedEntityGeometry(rows *dbsql.PGXResponse) (*model.DeletedSketchEntityGeometry, error) {
	var geometry model.DeletedSketchEntityGeometry
	var entity []byte
	var materializedGeometry []byte
	var historicalEntities []byte
	var historicalMaterializedGeometry []byte

	if err := rows.Scan(
		&geometry.SketchID,
		&geometry.EntityID,
		&geometry.Version,
		&entity,
		&materializedGeometry,
		&historicalEntities,
		&historicalMaterializedGeometry,
	); err != nil {
		return nil, fmt.Errorf("scan deleted sketch entity geometry: %w", err)
	}

	geometry.Entity = append(geometry.Entity, entity...)
	geometry.MaterializedGeometry = append(geometry.MaterializedGeometry, materializedGeometry...)
	if err := json.Unmarshal(historicalEntities, &geometry.HistoricalEntities); err != nil {
		return nil, fmt.Errorf("scan deleted sketch entity historical entities: %w", err)
	}
	if err := json.Unmarshal(historicalMaterializedGeometry, &geometry.HistoricalMaterializedGeometry); err != nil {
		return nil, fmt.Errorf("scan deleted sketch entity historical materialized geometry: %w", err)
	}

	return &geometry, nil
}

func scanMetadata(rows *dbsql.PGXResponse) (*model.SketchMetadata, error) {
	var metadata model.SketchMetadata
	var createdAt time.Time
	var updatedAt time.Time
	var plane []byte

	if err := rows.Scan(
		&metadata.ID,
		&metadata.WorkspaceID,
		&metadata.Name,
		&metadata.CreatedByUserID,
		&metadata.Unit,
		&plane,
		&metadata.Version,
		&createdAt,
		&updatedAt,
	); err != nil {
		return nil, fmt.Errorf("scan sketch metadata: %w", err)
	}

	metadata.CreatedAt = createdAt.UTC().Format(time.RFC3339Nano)
	metadata.UpdatedAt = updatedAt.UTC().Format(time.RFC3339Nano)
	if err := json.Unmarshal(plane, &metadata.Plane); err != nil {
		return nil, fmt.Errorf("scan sketch metadata plane: %w", err)
	}

	return &metadata, nil
}
