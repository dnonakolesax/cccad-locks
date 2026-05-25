package workspaces

import (
	"context"
	"errors"
	"fmt"
	"time"

	dbsql "github.com/dnonakolesax/cccad-locks/internal/db/sql"
	"github.com/dnonakolesax/cccad-locks/internal/model"
)

const (
	createWorkspaceRequest         = "workspace_create"
	listAvailableWorkspacesRequest = "workspace_list_available"
	updateWorkspaceRequest         = "workspace_update"
	deleteWorkspaceRequest         = "workspace_delete"
)

type Repository struct {
	db *dbsql.PGXWorker
}

func NewRepository(db *dbsql.PGXWorker) *Repository {
	return &Repository{db: db}
}

func (r *Repository) Create(
	ctx context.Context,
	request *model.CreateWorkspaceRequest,
	createdByUserID string,
) (*model.Workspace, error) {
	sqlRequest, err := r.db.Request(createWorkspaceRequest)
	if err != nil {
		return nil, fmt.Errorf("create workspace request: %w", err)
	}

	rows, err := r.db.Query(ctx, sqlRequest, request.Name, request.Description, createdByUserID)
	if err != nil {
		return nil, fmt.Errorf("create workspace: %w", err)
	}

	if !rows.Next() {
		if closeErr := rows.Close(); closeErr != nil {
			return nil, fmt.Errorf("create workspace rows: %w", closeErr)
		}
		return nil, errors.New("create workspace returned no rows")
	}

	workspace, err := scanWorkspace(rows)
	if err != nil {
		return nil, err
	}
	if closeErr := rows.Close(); closeErr != nil {
		return nil, fmt.Errorf("create workspace rows: %w", closeErr)
	}

	return workspace, nil
}

func (r *Repository) ListAvailable(ctx context.Context, userID string) ([]model.Workspace, error) {
	sqlRequest, err := r.db.Request(listAvailableWorkspacesRequest)
	if err != nil {
		return nil, fmt.Errorf("list available workspaces request: %w", err)
	}

	rows, err := r.db.Query(ctx, sqlRequest, userID)
	if err != nil {
		return nil, fmt.Errorf("list available workspaces: %w", err)
	}

	workspaces := make([]model.Workspace, 0)
	for rows.Next() {
		workspace, scanErr := scanWorkspace(rows)
		if scanErr != nil {
			return nil, scanErr
		}
		workspaces = append(workspaces, *workspace)
	}
	if closeErr := rows.Close(); closeErr != nil {
		return nil, fmt.Errorf("list available workspaces rows: %w", closeErr)
	}

	return workspaces, nil
}

func (r *Repository) Update(
	ctx context.Context,
	workspaceID string,
	request *model.UpdateWorkspaceRequest,
	actorUserID string,
) (*model.Workspace, error) {
	sqlRequest, err := r.db.Request(updateWorkspaceRequest)
	if err != nil {
		return nil, fmt.Errorf("update workspace request: %w", err)
	}

	rows, err := r.db.Query(ctx, sqlRequest, workspaceID, request.Name, request.Description, actorUserID)
	if err != nil {
		return nil, fmt.Errorf("update workspace: %w", err)
	}

	if !rows.Next() {
		if closeErr := rows.Close(); closeErr != nil {
			return nil, fmt.Errorf("update workspace rows: %w", closeErr)
		}
		return nil, errors.New("update workspace returned no rows")
	}

	workspace, err := scanWorkspace(rows)
	if err != nil {
		return nil, err
	}
	if closeErr := rows.Close(); closeErr != nil {
		return nil, fmt.Errorf("update workspace rows: %w", closeErr)
	}

	return workspace, nil
}

func (r *Repository) Delete(ctx context.Context, workspaceID string, actorUserID string) error {
	sqlRequest, err := r.db.Request(deleteWorkspaceRequest)
	if err != nil {
		return fmt.Errorf("delete workspace request: %w", err)
	}

	rows, err := r.db.Query(ctx, sqlRequest, workspaceID, actorUserID)
	if err != nil {
		return fmt.Errorf("delete workspace: %w", err)
	}

	if !rows.Next() {
		if closeErr := rows.Close(); closeErr != nil {
			return fmt.Errorf("delete workspace rows: %w", closeErr)
		}
		return errors.New("delete workspace returned no rows")
	}

	if closeErr := rows.Close(); closeErr != nil {
		return fmt.Errorf("delete workspace rows: %w", closeErr)
	}

	return nil
}

func scanWorkspace(rows *dbsql.PGXResponse) (*model.Workspace, error) {
	var workspace model.Workspace
	var createdAt time.Time
	var updatedAt time.Time

	if err := rows.Scan(
		&workspace.ID,
		&workspace.Name,
		&workspace.Description,
		&workspace.CreatedByUserID,
		&createdAt,
		&updatedAt,
	); err != nil {
		return nil, fmt.Errorf("scan workspace: %w", err)
	}

	workspace.CreatedAt = createdAt.UTC().Format(time.RFC3339Nano)
	workspace.UpdatedAt = updatedAt.UTC().Format(time.RFC3339Nano)

	return &workspace, nil
}
