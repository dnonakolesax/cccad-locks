-- +goose Up
ALTER TABLE parts
    ADD COLUMN IF NOT EXISTS workspace_id UUID REFERENCES workspaces(id) ON DELETE CASCADE,
    ADD COLUMN IF NOT EXISTS created_by_user_id TEXT;

CREATE INDEX IF NOT EXISTS idx_parts_workspace_not_deleted
    ON parts(workspace_id, updated_at DESC)
    WHERE workspace_id IS NOT NULL AND deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_parts_created_by_user_id
    ON parts(created_by_user_id)
    WHERE created_by_user_id IS NOT NULL AND deleted_at IS NULL;

-- +goose Down
DROP INDEX IF EXISTS idx_parts_created_by_user_id;
DROP INDEX IF EXISTS idx_parts_workspace_not_deleted;

ALTER TABLE parts
    DROP COLUMN IF EXISTS created_by_user_id,
    DROP COLUMN IF EXISTS workspace_id;
