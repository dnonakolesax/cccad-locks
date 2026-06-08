SELECT
    id::text,
    workspace_id::text,
    name,
    created_by_user_id,
    created_at,
    updated_at
FROM parts
WHERE workspace_id = $1::uuid
  AND deleted_at IS NULL
ORDER BY updated_at DESC, created_at DESC, id
