WITH deleted_permission AS (
    DELETE FROM sketch_permissions
    WHERE sketch_id = $1::uuid AND user_id = $2
    RETURNING sketch_id, user_id, role, granted_by_user_id
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
        sketch_id,
        user_id,
        granted_by_user_id,
        role,
        NULL,
        'revoke'
    FROM deleted_permission
    RETURNING id
)
SELECT 1 FROM deleted_permission
