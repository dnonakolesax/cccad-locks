package permissions

import (
	"context"
	"errors"
	"fmt"
	"time"

	dbsql "github.com/dnonakolesax/cccad-locks/internal/db/sql"
	"github.com/dnonakolesax/cccad-locks/internal/model"
)

const (
	explicitSource          = "explicit"
	listPermissionsRequest  = "permissions_list"
	putPermissionRequest    = "permissions_put"
	deletePermissionRequest = "permissions_delete"
)

type Repository struct {
	db *dbsql.PGXWorker
}

func NewRepository(db *dbsql.PGXWorker) *Repository {
	return &Repository{db: db}
}

func (r *Repository) List(ctx context.Context, sketchID string) ([]model.Permission, error) {
	sqlRequest, err := r.db.Request(listPermissionsRequest)
	if err != nil {
		return nil, fmt.Errorf("list permissions request: %w", err)
	}

	rows, err := r.db.Query(ctx, sqlRequest, sketchID)
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
	if closeErr := rows.Close(); closeErr != nil {
		return nil, fmt.Errorf("list permissions rows: %w", closeErr)
	}

	return permissions, nil
}

func (r *Repository) Put(ctx context.Context, permission *model.Permission) (*model.Permission, error) {
	grantedBy := permission.UserID
	if permission.GrantedByUserID != nil && *permission.GrantedByUserID != "" {
		grantedBy = *permission.GrantedByUserID
	}

	sqlRequest, err := r.db.Request(putPermissionRequest)
	if err != nil {
		return nil, fmt.Errorf("put permission request: %w", err)
	}

	rows, err := r.db.Query(ctx, sqlRequest, permission.SketchID, permission.UserID, permission.Role, grantedBy)
	if err != nil {
		return nil, fmt.Errorf("put permission: %w", err)
	}

	if !rows.Next() {
		if closeErr := rows.Close(); closeErr != nil {
			return nil, fmt.Errorf("put permission rows: %w", closeErr)
		}
		return nil, errors.New("put permission returned no rows")
	}

	result, err := scanPermission(rows)
	if err != nil {
		return nil, err
	}
	if closeErr := rows.Close(); closeErr != nil {
		return nil, fmt.Errorf("put permission rows: %w", closeErr)
	}

	return result, nil
}

func (r *Repository) Delete(ctx context.Context, userID, sketchID string) error {
	sqlRequest, err := r.db.Request(deletePermissionRequest)
	if err != nil {
		return fmt.Errorf("delete permission request: %w", err)
	}

	rows, err := r.db.Query(ctx, sqlRequest, sketchID, userID)
	if err != nil {
		return fmt.Errorf("delete permission: %w", err)
	}

	if closeErr := rows.Close(); closeErr != nil {
		return fmt.Errorf("delete permission rows: %w", closeErr)
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
