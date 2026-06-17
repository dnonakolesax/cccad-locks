WITH allowed AS (
    SELECT c.id
    FROM cad_comments c
    JOIN workspaces w ON w.id = c.workspace_id
    WHERE c.id = $1::uuid
        AND c.deleted_at IS NULL
        AND c.message_type = 'user'
        AND w.deleted_at IS NULL
        AND (
            w.created_by_user_id = $2
            OR EXISTS (
                SELECT 1
                FROM sketch_permissions sp
                WHERE sp.sketch_id = c.sketch_id
                    AND sp.user_id = $2
                    AND sp.role IN ('editor', 'admin')
            )
        )
),
deleted AS (
    DELETE FROM comment_assignees ca
    USING allowed a
    WHERE ca.comment_id = a.id
    RETURNING 1
)
SELECT id::text FROM allowed
