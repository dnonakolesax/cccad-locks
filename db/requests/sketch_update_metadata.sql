UPDATE sketches
SET
    name = COALESCE($2, name),
    unit = COALESCE($3::sketch_unit, unit)
WHERE id = $1::uuid
    AND deleted_at IS NULL
RETURNING
    id::text,
    workspace_id::text,
    name,
    created_by_user_id,
    unit::text,
    plane,
    version,
    created_at,
    updated_at
