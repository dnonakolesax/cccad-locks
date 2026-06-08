package parts3d

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	dbsql "github.com/dnonakolesax/cccad-locks/internal/db/sql"
	"github.com/dnonakolesax/cccad-locks/internal/model"
	"github.com/jackc/pgx/v5"
)

const (
	createPartRequest         = "parts3d_part_create"
	listPartsWorkspaceRequest = "parts3d_parts_list_by_workspace"
	deletePartRequest         = "parts3d_part_delete"
	listFeaturesRequest       = "parts3d_features_list"
	listBodiesRequest         = "parts3d_bodies_list"
	getTopologyRequest        = "parts3d_topology_get"
	getFacePlaneRequest       = "parts3d_face_plane_get"
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
	request *model.CreatePart3DRequest,
	createdByUserID string,
) (*model.Part3D, error) {
	sqlRequest, err := r.db.Request(createPartRequest)
	if err != nil {
		return nil, fmt.Errorf("create 3d part request: %w", err)
	}

	rows, err := r.db.Query(ctx, sqlRequest, workspaceID, request.Name, createdByUserID)
	if err != nil {
		return nil, fmt.Errorf("create 3d part: %w", err)
	}

	if !rows.Next() {
		if closeErr := rows.Close(); closeErr != nil {
			return nil, fmt.Errorf("create 3d part rows: %w", closeErr)
		}
		return nil, errors.New("create 3d part returned no rows")
	}

	part, err := scanPart(rows)
	if err != nil {
		_ = rows.Close()
		return nil, err
	}
	if closeErr := rows.Close(); closeErr != nil {
		return nil, fmt.Errorf("create 3d part rows: %w", closeErr)
	}

	return part, nil
}

func (r *Repository) ListByWorkspace(ctx context.Context, workspaceID string) ([]model.Part3D, error) {
	sqlRequest, err := r.db.Request(listPartsWorkspaceRequest)
	if err != nil {
		return nil, fmt.Errorf("list workspace 3d parts request: %w", err)
	}

	rows, err := r.db.Query(ctx, sqlRequest, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("list workspace 3d parts: %w", err)
	}

	parts := make([]model.Part3D, 0)
	for rows.Next() {
		part, scanErr := scanPart(rows)
		if scanErr != nil {
			_ = rows.Close()
			return nil, scanErr
		}
		parts = append(parts, *part)
	}
	if closeErr := rows.Close(); closeErr != nil {
		return nil, fmt.Errorf("list workspace 3d parts rows: %w", closeErr)
	}

	return parts, nil
}

func (r *Repository) Delete(ctx context.Context, partID string) error {
	sqlRequest, err := r.db.Request(deletePartRequest)
	if err != nil {
		return fmt.Errorf("delete 3d part request: %w", err)
	}

	rows, err := r.db.Query(ctx, sqlRequest, partID)
	if err != nil {
		return fmt.Errorf("delete 3d part: %w", err)
	}

	if !rows.Next() {
		if closeErr := rows.Close(); closeErr != nil {
			return fmt.Errorf("delete 3d part rows: %w", closeErr)
		}
		return errors.New("delete 3d part returned no rows")
	}
	if closeErr := rows.Close(); closeErr != nil {
		return fmt.Errorf("delete 3d part rows: %w", closeErr)
	}

	return nil
}

func (r *Repository) ListFeatures(
	ctx context.Context,
	partID string,
	includeSuppressed bool,
) ([]model.Feature3D, error) {
	sqlRequest, err := r.db.Request(listFeaturesRequest)
	if err != nil {
		return nil, fmt.Errorf("list 3d features request: %w", err)
	}

	rows, err := r.db.Query(ctx, sqlRequest, partID, includeSuppressed)
	if err != nil {
		return nil, fmt.Errorf("list 3d features: %w", err)
	}

	features := make([]model.Feature3D, 0)
	for rows.Next() {
		feature, scanErr := scanFeature(rows)
		if scanErr != nil {
			_ = rows.Close()
			return nil, scanErr
		}
		features = append(features, *feature)
	}
	if closeErr := rows.Close(); closeErr != nil {
		return nil, fmt.Errorf("list 3d features rows: %w", closeErr)
	}

	return features, nil
}

func (r *Repository) ListBodies(ctx context.Context, partID string) ([]model.Body3D, error) {
	sqlRequest, err := r.db.Request(listBodiesRequest)
	if err != nil {
		return nil, fmt.Errorf("list 3d bodies request: %w", err)
	}

	rows, err := r.db.Query(ctx, sqlRequest, partID)
	if err != nil {
		return nil, fmt.Errorf("list 3d bodies: %w", err)
	}

	bodies := make([]model.Body3D, 0)
	for rows.Next() {
		body, scanErr := scanBody(rows)
		if scanErr != nil {
			_ = rows.Close()
			return nil, scanErr
		}
		bodies = append(bodies, *body)
	}
	if closeErr := rows.Close(); closeErr != nil {
		return nil, fmt.Errorf("list 3d bodies rows: %w", closeErr)
	}

	return bodies, nil
}

func (r *Repository) GetTopology(
	ctx context.Context,
	partID string,
	bodyID *string,
) (*model.TopologySummary3D, error) {
	sqlRequest, err := r.db.Request(getTopologyRequest)
	if err != nil {
		return nil, fmt.Errorf("get 3d topology request: %w", err)
	}

	var bodyArg any
	if bodyID != nil {
		bodyArg = *bodyID
	}
	rows, err := r.db.Query(ctx, sqlRequest, partID, bodyArg)
	if err != nil {
		return nil, fmt.Errorf("get 3d topology: %w", err)
	}

	builder := newTopologyBuilder()
	for rows.Next() {
		var row topologyRow
		if scanErr := scanTopologyRow(rows, &row); scanErr != nil {
			_ = rows.Close()
			return nil, scanErr
		}
		if err := builder.add(row); err != nil {
			_ = rows.Close()
			return nil, err
		}
	}
	if closeErr := rows.Close(); closeErr != nil {
		return nil, fmt.Errorf("get 3d topology rows: %w", closeErr)
	}

	return builder.summary(), nil
}

func (r *Repository) GetFacePlane(
	ctx context.Context,
	partID string,
	bodyID string,
	faceID string,
) (*model.FacePlane3D, error) {
	sqlRequest, err := r.db.Request(getFacePlaneRequest)
	if err != nil {
		return nil, fmt.Errorf("get 3d face plane request: %w", err)
	}

	rows, err := r.db.Query(ctx, sqlRequest, partID, bodyID, faceID)
	if err != nil {
		return nil, fmt.Errorf("get 3d face plane: %w", err)
	}

	if !rows.Next() {
		if closeErr := rows.Close(); closeErr != nil {
			return nil, fmt.Errorf("get 3d face plane rows: %w", closeErr)
		}
		return nil, nil
	}

	plane, err := scanFacePlane(rows)
	if err != nil {
		_ = rows.Close()
		return nil, err
	}
	if closeErr := rows.Close(); closeErr != nil {
		return nil, fmt.Errorf("get 3d face plane rows: %w", closeErr)
	}

	return plane, nil
}

func (r *Repository) CommitFeatureBuild(
	ctx context.Context,
	commit model.Feature3DCommit,
) (*model.Feature3DCommitResult, error) {
	result := &model.Feature3DCommitResult{
		FeatureID:       commit.FeatureID,
		DocumentVersion: commit.DocumentVersion,
	}

	err := r.db.WithTx(ctx, func(txCtx context.Context, tx pgx.Tx) error {
		payload := []byte(commit.Payload)
		if len(payload) == 0 {
			payload = []byte(`{}`)
		}
		diagnostics, err := json.Marshal(commit.Diagnostics)
		if err != nil {
			return fmt.Errorf("marshal 3d diagnostics: %w", err)
		}

		if err := tx.QueryRow(txCtx, `
INSERT INTO features_3d (
    id,
    part_id,
    sketch_id,
    type,
    payload,
    order_index,
    suppressed,
    created_by
)
VALUES (
    $1::uuid,
    $2::uuid,
    NULLIF($3, '')::uuid,
    $4::feature_3d_type,
    $5::jsonb,
    COALESCE((SELECT max(order_index) + 1 FROM features_3d WHERE part_id = $2::uuid AND deleted_at IS NULL), 0),
    $6,
    NULLIF($7, '')::uuid
)
RETURNING order_index`, commit.FeatureID, commit.PartID, commit.SketchID, commit.Type, payload, commit.Suppressed, commit.CreatedBy).Scan(&result.OrderIndex); err != nil {
			return fmt.Errorf("insert 3d feature: %w", err)
		}

		var rebuildID string
		status := "success"
		if !commit.Success {
			status = "failed"
		}
		if err := tx.QueryRow(txCtx, `
INSERT INTO part_rebuilds_3d (
    part_id,
    document_version,
    status,
    started_at,
    finished_at,
    failed_feature_id,
    diagnostics,
    created_by
)
VALUES (
    $1::uuid,
    $2,
    $3::feature_3d_build_status,
    now(),
    now(),
    CASE WHEN $3 = 'failed' THEN $4::uuid ELSE NULL END,
    $5::jsonb,
    NULLIF($6, '')::uuid
)
RETURNING id::text`, commit.PartID, commit.DocumentVersion, status, commit.FeatureID, diagnostics, commit.CreatedBy).Scan(&rebuildID); err != nil {
			return fmt.Errorf("insert 3d rebuild: %w", err)
		}

		for _, body := range commit.Bodies {
			active := body.Active
			if !active {
				active = true
			}
			if _, err := tx.Exec(txCtx, `
INSERT INTO part_bodies_3d (
    id,
    part_id,
    name,
    created_by_feature_id,
    active,
    stable_ref
)
VALUES ($1::uuid, $2::uuid, $3, $4::uuid, $5, NULLIF($6, ''))
ON CONFLICT (id) DO UPDATE SET
    name = EXCLUDED.name,
    created_by_feature_id = EXCLUDED.created_by_feature_id,
    active = EXCLUDED.active,
    stable_ref = EXCLUDED.stable_ref`, body.ID, commit.PartID, bodyName(body), body.CreatedByFeatureID, active, body.StableRef); err != nil {
				return fmt.Errorf("upsert 3d body %q: %w", body.ID, err)
			}
		}

		for _, rep := range commit.Representations {
			if rep.Kind == "" || rep.StorageKey == "" {
				continue
			}
			if _, err := tx.Exec(txCtx, `
INSERT INTO part_representations_3d (
    part_id,
    body_id,
    document_version,
    kind,
    storage_key,
    content_type,
    size_bytes,
    sha256
)
VALUES (
    $1::uuid,
    NULLIF($2, '')::uuid,
    $3,
    $4::body_3d_representation_kind,
    $5,
    NULLIF($6, ''),
    NULLIF($7, 0),
    NULLIF($8, '')
)
ON CONFLICT (storage_key) DO UPDATE SET
    content_type = EXCLUDED.content_type,
    size_bytes = EXCLUDED.size_bytes,
    sha256 = EXCLUDED.sha256`, commit.PartID, rep.BodyID, commit.DocumentVersion, rep.Kind, rep.StorageKey, rep.ContentType, rep.SizeBytes, rep.SHA256); err != nil {
				return fmt.Errorf("insert 3d representation %q: %w", rep.StorageKey, err)
			}
		}

		if _, err := tx.Exec(txCtx, `
DELETE FROM topology_refs_3d
WHERE part_id = $1::uuid
  AND document_version = $2`, commit.PartID, commit.DocumentVersion); err != nil {
			return fmt.Errorf("clear 3d topology: %w", err)
		}

		for _, ref := range commit.Topology {
			payload := []byte(ref.Payload)
			if len(payload) == 0 {
				payload = []byte(`{}`)
			}
			if _, err := tx.Exec(txCtx, `
INSERT INTO topology_refs_3d (
    part_id,
    body_id,
    document_version,
    ref_kind,
    ref_id,
    stable_ref,
    parent_ref_id,
    surface_or_curve_type,
    payload
)
VALUES (
    $1::uuid,
    $2::uuid,
    $3,
    $4,
    $5,
    NULLIF($6, ''),
    NULLIF($7, ''),
    NULLIF($8, ''),
    $9::jsonb
)`, commit.PartID, ref.BodyID, commit.DocumentVersion, ref.RefKind, ref.RefID, ref.StableRef, ref.ParentRefID, ref.SurfaceOrCurveType, payload); err != nil {
				return fmt.Errorf("insert 3d topology %s/%s: %w", ref.RefKind, ref.RefID, err)
			}
		}

		if _, err := tx.Exec(txCtx, `
INSERT INTO feature_build_results_3d (
    rebuild_id,
    feature_id,
    part_id,
    status,
    diagnostics,
    created_body_id
)
VALUES (
    $1::uuid,
    $2::uuid,
    $3::uuid,
    $4::feature_3d_build_status,
    $5::jsonb,
    NULLIF($6, '')::uuid
)`, rebuildID, commit.FeatureID, commit.PartID, status, diagnostics, firstBodyID(commit.Bodies)); err != nil {
			return fmt.Errorf("insert 3d build result: %w", err)
		}

		return nil
	})
	if err == nil {
		return result, nil
	}

	return nil, err
}

func bodyName(body model.Body3DCommit) string {
	if body.Name != "" {
		return body.Name
	}
	return "Body " + body.ID
}

func firstBodyID(bodies []model.Body3DCommit) string {
	if len(bodies) == 0 {
		return ""
	}
	return bodies[0].ID
}

func scanPart(rows *dbsql.PGXResponse) (*model.Part3D, error) {
	var part model.Part3D
	var createdAt time.Time
	var updatedAt time.Time

	if err := rows.Scan(
		&part.ID,
		&part.WorkspaceID,
		&part.Name,
		&part.CreatedByUserID,
		&createdAt,
		&updatedAt,
	); err != nil {
		return nil, fmt.Errorf("scan 3d part: %w", err)
	}

	part.CreatedAt = createdAt.UTC().Format(time.RFC3339Nano)
	part.UpdatedAt = updatedAt.UTC().Format(time.RFC3339Nano)

	return &part, nil
}

func scanFeature(rows *dbsql.PGXResponse) (*model.Feature3D, error) {
	var feature model.Feature3D
	var payload []byte
	var createdAt time.Time
	var updatedAt time.Time

	if err := rows.Scan(
		&feature.ID,
		&feature.PartID,
		&feature.SketchID,
		&feature.Type,
		&payload,
		&feature.OrderIndex,
		&feature.Suppressed,
		&feature.CreatedBy,
		&createdAt,
		&updatedAt,
	); err != nil {
		return nil, fmt.Errorf("scan 3d feature: %w", err)
	}

	feature.Payload = append(feature.Payload[:0], payload...)
	feature.CreatedAt = createdAt.UTC().Format(time.RFC3339Nano)
	feature.UpdatedAt = updatedAt.UTC().Format(time.RFC3339Nano)

	return &feature, nil
}

func scanBody(rows *dbsql.PGXResponse) (*model.Body3D, error) {
	var body model.Body3D
	var createdAt time.Time
	var updatedAt time.Time

	if err := rows.Scan(
		&body.ID,
		&body.PartID,
		&body.Name,
		&body.Active,
		&body.CreatedByFeatureID,
		&body.StableRef,
		&createdAt,
		&updatedAt,
	); err != nil {
		return nil, fmt.Errorf("scan 3d body: %w", err)
	}

	body.CreatedAt = createdAt.UTC().Format(time.RFC3339Nano)
	body.UpdatedAt = updatedAt.UTC().Format(time.RFC3339Nano)

	return &body, nil
}

type topologyRow struct {
	bodyID             string
	refKind            string
	refID              string
	stableRef          string
	parentRefID        string
	surfaceOrCurveType string
	payload            map[string]any
}

func scanTopologyRow(rows *dbsql.PGXResponse, row *topologyRow) error {
	var payload []byte
	if err := rows.Scan(
		&row.bodyID,
		&row.refKind,
		&row.refID,
		&row.stableRef,
		&row.parentRefID,
		&row.surfaceOrCurveType,
		&payload,
	); err != nil {
		return fmt.Errorf("scan 3d topology row: %w", err)
	}

	if len(payload) == 0 {
		row.payload = map[string]any{}
		return nil
	}
	if err := json.Unmarshal(payload, &row.payload); err != nil {
		return fmt.Errorf("decode 3d topology payload: %w", err)
	}

	return nil
}

func scanFacePlane(rows *dbsql.PGXResponse) (*model.FacePlane3D, error) {
	var surfaceType string
	var payload []byte
	if err := rows.Scan(&surfaceType, &payload); err != nil {
		return nil, fmt.Errorf("scan 3d face plane: %w", err)
	}

	data := map[string]any{}
	if len(payload) != 0 {
		if err := json.Unmarshal(payload, &data); err != nil {
			return nil, fmt.Errorf("decode 3d face plane payload: %w", err)
		}
	}

	return &model.FacePlane3D{
		SurfaceType: surfaceType,
		Plane:       planeFromPayload(data),
		Diagnostics: diagnosticsFromPayload(data),
	}, nil
}

type topologyBuilder struct {
	bodyOrder []string
	bodies    map[string]*topologyBodyState
	shells    map[string]*model.TopologyShell3D
	faces     map[string]*model.TopologyFace3D
	loops     map[string]*model.TopologyLoop3D
}

type topologyBodyState struct {
	body     model.TopologyBody3D
	shellIDs []string
}

func newTopologyBuilder() *topologyBuilder {
	return &topologyBuilder{
		bodies: map[string]*topologyBodyState{},
		shells: map[string]*model.TopologyShell3D{},
		faces:  map[string]*model.TopologyFace3D{},
		loops:  map[string]*model.TopologyLoop3D{},
	}
}

func (b *topologyBuilder) add(row topologyRow) error {
	state := b.ensureBody(row.bodyID)
	switch row.refKind {
	case "body":
		state.body.StableRef = row.stableRef
	case "shell":
		shell := &model.TopologyShell3D{
			ShellID:   row.refID,
			StableRef: row.stableRef,
			Faces:     []model.TopologyFace3D{},
		}
		b.shells[topologyKey(row.bodyID, row.refID)] = shell
		state.shellIDs = append(state.shellIDs, row.refID)
	case "face":
		face := &model.TopologyFace3D{
			FaceID:      row.refID,
			StableRef:   row.stableRef,
			SurfaceType: row.surfaceOrCurveType,
			Plane:       planeFromPayload(row.payload),
			Loops:       []model.TopologyLoop3D{},
		}
		shell := b.shells[topologyKey(row.bodyID, row.parentRefID)]
		if shell == nil {
			return fmt.Errorf("topology face %q references missing shell %q", row.refID, row.parentRefID)
		}
		shell.Faces = append(shell.Faces, *face)
		b.faces[topologyKey(row.bodyID, row.refID)] = &shell.Faces[len(shell.Faces)-1]
	case "loop":
		loop := &model.TopologyLoop3D{
			LoopID:    row.refID,
			StableRef: row.stableRef,
			Edges:     []model.TopologyEdge3D{},
		}
		face := b.faces[topologyKey(row.bodyID, row.parentRefID)]
		if face == nil {
			return fmt.Errorf("topology loop %q references missing face %q", row.refID, row.parentRefID)
		}
		face.Loops = append(face.Loops, *loop)
		b.loops[topologyKey(row.bodyID, row.refID)] = &face.Loops[len(face.Loops)-1]
	case "edge":
		edge := model.TopologyEdge3D{
			EdgeID:        row.refID,
			StableRef:     row.stableRef,
			CurveType:     row.surfaceOrCurveType,
			StartVertexID: stringFromPayload(row.payload, "startVertexId", "start_vertex_id"),
			EndVertexID:   stringFromPayload(row.payload, "endVertexId", "end_vertex_id"),
		}
		loop := b.loops[topologyKey(row.bodyID, row.parentRefID)]
		if loop == nil {
			return fmt.Errorf("topology edge %q references missing loop %q", row.refID, row.parentRefID)
		}
		loop.Edges = append(loop.Edges, edge)
	case "vertex":
		state.body.Vertices = append(state.body.Vertices, model.TopologyVertex3D{
			VertexID:  row.refID,
			StableRef: row.stableRef,
			Point:     vectorFromPayload(row.payload, "point"),
		})
	default:
		return fmt.Errorf("unsupported topology ref kind %q", row.refKind)
	}
	return nil
}

func (b *topologyBuilder) ensureBody(bodyID string) *topologyBodyState {
	if state := b.bodies[bodyID]; state != nil {
		return state
	}
	state := &topologyBodyState{
		body: model.TopologyBody3D{
			BodyID:   bodyID,
			Shells:   []model.TopologyShell3D{},
			Vertices: []model.TopologyVertex3D{},
		},
	}
	b.bodies[bodyID] = state
	b.bodyOrder = append(b.bodyOrder, bodyID)
	return state
}

func (b *topologyBuilder) summary() *model.TopologySummary3D {
	result := &model.TopologySummary3D{Bodies: []model.TopologyBody3D{}}
	for _, bodyID := range b.bodyOrder {
		state := b.bodies[bodyID]
		body := state.body
		body.Shells = make([]model.TopologyShell3D, 0, len(state.shellIDs))
		for _, shellID := range state.shellIDs {
			if shell := b.shells[topologyKey(bodyID, shellID)]; shell != nil {
				body.Shells = append(body.Shells, *shell)
			}
		}
		result.Bodies = append(result.Bodies, body)
	}

	return result
}

func topologyKey(bodyID, refID string) string {
	return bodyID + "\x00" + refID
}

func planeFromPayload(payload map[string]any) *model.SketchPlane3D {
	raw, ok := payload["plane"]
	if !ok {
		return nil
	}
	body, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	var plane model.SketchPlane3D
	if err := json.Unmarshal(body, &plane); err != nil {
		return nil
	}
	return &plane
}

func vectorFromPayload(payload map[string]any, key string) *model.Vector3 {
	raw, ok := payload[key]
	if !ok {
		return nil
	}
	body, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	var vector model.Vector3
	if err := json.Unmarshal(body, &vector); err != nil {
		return nil
	}
	return &vector
}

func stringFromPayload(payload map[string]any, keys ...string) string {
	for _, key := range keys {
		if value, ok := payload[key].(string); ok {
			return value
		}
	}
	return ""
}

func diagnosticsFromPayload(payload map[string]any) []model.Diagnostic3D {
	raw, ok := payload["diagnostics"]
	if !ok {
		return nil
	}
	body, err := json.Marshal(raw)
	if err != nil {
		return nil
	}
	var diagnostics []model.Diagnostic3D
	if err := json.Unmarshal(body, &diagnostics); err != nil {
		return nil
	}
	return diagnostics
}
