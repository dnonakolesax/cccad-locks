UPDATE cad_comments c
SET deleted_at = COALESCE(c.deleted_at, now())
FROM workspaces w
WHERE c.id = $1::uuid
    AND w.id = c.workspace_id
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
RETURNING c.id::text
