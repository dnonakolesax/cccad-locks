UPDATE workspaces
SET
    name = COALESCE($2, name),
    description = COALESCE($3, description)
WHERE id = $1::uuid
    AND created_by_user_id = $4
    AND deleted_at IS NULL
RETURNING
    id::text,
    name,
    COALESCE(description, ''),
    created_by_user_id,
    created_at,
    updated_at
