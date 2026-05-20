WITH previous_permission AS (
    SELECT role
    FROM sketch_permissions
    WHERE sketch_id = $1::uuid AND user_id = $2
),
upserted_permission AS (
    INSERT INTO sketch_permissions (sketch_id, user_id, role, granted_by_user_id)
    VALUES ($1::uuid, $2, $3::sketch_role, $4)
    ON CONFLICT (sketch_id, user_id)
    DO UPDATE SET
        role = EXCLUDED.role,
        granted_by_user_id = EXCLUDED.granted_by_user_id
    RETURNING sketch_id, user_id, role, granted_by_user_id, granted_at, updated_at
),
audit AS (
    INSERT INTO sketch_permission_audit (
        sketch_id,
        target_user_id,
        actor_user_id,
        old_role,
        new_role,
        action
    )
    SELECT
        $1::uuid,
        $2,
        $4,
        (SELECT role FROM previous_permission),
        $3::sketch_role,
        CASE
            WHEN EXISTS (SELECT 1 FROM previous_permission) THEN 'update'
            ELSE 'grant'
        END
    RETURNING id
)
SELECT
    sketch_id::text,
    user_id,
    role::text,
    granted_by_user_id,
    granted_at,
    updated_at
FROM upserted_permission
