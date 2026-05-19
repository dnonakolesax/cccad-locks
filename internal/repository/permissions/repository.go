package permissions

import (
	"context"
	"fmt"
	"time"

	dbsql "github.com/dnonakolesax/cccad-locks/internal/db/sql"
	"github.com/dnonakolesax/cccad-locks/internal/model"
)

const explicitSource = "explicit"

type Repository struct {
	db *dbsql.PGXWorker
}

func NewRepository(db *dbsql.PGXWorker) *Repository {
	return &Repository{db: db}
}

func (r *Repository) List(ctx context.Context, sketchID string) ([]model.Permission, error) {
	rows, err := r.db.Query(ctx, `
SELECT
    sketch_id::text,
    user_id,
    role::text,
    granted_by_user_id,
    granted_at,
    updated_at
FROM sketch_permissions
WHERE sketch_id = $1::uuid
ORDER BY role DESC, user_id ASC
`, sketchID)
	if err != nil {
		return nil, fmt.Errorf("list permissions: %w", err)
	}

	permissions := make([]model.Permission, 0)
	for rows.Next() {
		permission, scanErr := scanPermission(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		permissions = append(permissions, *permission)
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("list permissions rows: %w", err)
	}

	return permissions, nil
}

func (r *Repository) Put(ctx context.Context, permission *model.Permission) (*model.Permission, error) {
	grantedBy := permission.UserID
	if permission.GrantedByUserID != nil && *permission.GrantedByUserID != "" {
		grantedBy = *permission.GrantedByUserID
	}

	rows, err := r.db.Query(ctx, `
WITH previous_permission AS (
    SELECT role
    FROM sketch_permissions
    WHERE sketch_id = $1::uuid AND user_id = $2
),
upserted_permission AS (
    INSERT INTO sketch_permissions (sketch_id, user_id, role, granted_by_user_id)
    VALUES ($1::uuid, $2, $3::sketch_role, $4)
    ON CONFLICT (sketch_id, user_id)
    DO UPDATE SET
        role = EXCLUDED.role,
        granted_by_user_id = EXCLUDED.granted_by_user_id
    RETURNING sketch_id, user_id, role, granted_by_user_id, granted_at, updated_at
),
audit AS (
    INSERT INTO sketch_permission_audit (
        sketch_id,
        target_user_id,
        actor_user_id,
        old_role,
        new_role,
        action
    )
    SELECT
        $1::uuid,
        $2,
        $4,
        (SELECT role FROM previous_permission),
        $3::sketch_role,
        CASE
            WHEN EXISTS (SELECT 1 FROM previous_permission) THEN 'update'
            ELSE 'grant'
        END
    RETURNING id
)
SELECT
    sketch_id::text,
    user_id,
    role::text,
    granted_by_user_id,
    granted_at,
    updated_at
FROM upserted_permission
`, permission.SketchID, permission.UserID, permission.Role, grantedBy)
	if err != nil {
		return nil, fmt.Errorf("put permission: %w", err)
	}

	if !rows.Next() {
		if err := rows.Close(); err != nil {
			return nil, fmt.Errorf("put permission rows: %w", err)
		}
		return nil, fmt.Errorf("put permission returned no rows")
	}

	result, err := scanPermission(rows)
	if err != nil {
		return nil, err
	}
	if err := rows.Close(); err != nil {
		return nil, fmt.Errorf("put permission rows: %w", err)
	}

	return result, nil
}

func (r *Repository) Delete(ctx context.Context, userID, sketchID string) error {
	rows, err := r.db.Query(ctx, `
WITH deleted_permission AS (
    DELETE FROM sketch_permissions
    WHERE sketch_id = $1::uuid AND user_id = $2
    RETURNING sketch_id, user_id, role, granted_by_user_id
),
audit AS (
    INSERT INTO sketch_permission_audit (
        sketch_id,
        target_user_id,
        actor_user_id,
        old_role,
        new_role,
        action
    )
    SELECT
        sketch_id,
        user_id,
        granted_by_user_id,
        role,
        NULL,
        'revoke'
    FROM deleted_permission
    RETURNING id
)
SELECT 1 FROM deleted_permission
`, sketchID, userID)
	if err != nil {
		return fmt.Errorf("delete permission: %w", err)
	}

	if err := rows.Close(); err != nil {
		return fmt.Errorf("delete permission rows: %w", err)
	}

	return nil
}

func scanPermission(rows *dbsql.PGXResponse) (*model.Permission, error) {
	var permission model.Permission
	var grantedBy string
	var grantedAt time.Time
	var updatedAt time.Time

	if err := rows.Scan(
		&permission.SketchID,
		&permission.UserID,
		&permission.Role,
		&grantedBy,
		&grantedAt,
		&updatedAt,
	); err != nil {
		return nil, fmt.Errorf("scan permission: %w", err)
	}

	permission.Source = explicitSource
	permission.GrantedByUserID = &grantedBy
	permission.CreatedAt = grantedAt.UTC().Format(time.RFC3339Nano)
	permission.UpdatedAt = updatedAt.UTC().Format(time.RFC3339Nano)

	return &permission, nil
}
