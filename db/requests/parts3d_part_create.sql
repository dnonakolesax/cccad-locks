INSERT INTO parts (
    id,
    workspace_id,
    name,
    created_by_user_id
)
VALUES (
    gen_random_uuid(),
    $1::uuid,
    $2,
    $3
)
RETURNING
    id::text,
    workspace_id::text,
    name,
    created_by_user_id,
    created_at,
    updated_at
