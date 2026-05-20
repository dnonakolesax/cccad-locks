SELECT DISTINCT
    w.id::text AS id,
    w.name AS name,
    COALESCE(w.description, '') AS description,
    w.created_by_user_id AS created_by_user_id,
    w.created_at AS created_at,
    w.updated_at AS updated_at
FROM workspaces w
LEFT JOIN sketches s
    ON s.workspace_id = w.id
    AND s.deleted_at IS NULL
LEFT JOIN sketch_permissions sp
    ON sp.sketch_id = s.id
    AND sp.user_id = $1
    AND sp.role IN ('reader', 'editor', 'admin')
WHERE w.deleted_at IS NULL
    AND (
        w.created_by_user_id = $1
        OR sp.user_id IS NOT NULL
    )
ORDER BY updated_at DESC, created_at DESC, id ASC
