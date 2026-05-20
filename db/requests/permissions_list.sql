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
