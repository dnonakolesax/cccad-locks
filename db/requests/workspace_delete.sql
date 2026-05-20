UPDATE workspaces
SET deleted_at = now()
WHERE id = $1::uuid
    AND created_by_user_id = $2
    AND deleted_at IS NULL
RETURNING 1
