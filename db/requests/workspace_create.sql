INSERT INTO workspaces (
    id,
    name,
    description,
    created_by_user_id
)
VALUES (
    gen_random_uuid(),
    $1,
    NULLIF($2, ''),
    $3
)
RETURNING
    id::text,
    name,
    COALESCE(description, ''),
    created_by_user_id,
    created_at,
    updated_at
