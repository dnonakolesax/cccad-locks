SELECT
    s.id::text,
    s.workspace_id::text,
    s.name,
    s.created_by_user_id,
    s.unit::text,
    s.version,
    sp.role::text,
    s.created_at,
    s.updated_at
FROM sketches s
JOIN sketch_permissions sp ON sp.sketch_id = s.id
WHERE sp.user_id = $1
    AND sp.role IN ('reader', 'editor', 'admin')
    AND s.deleted_at IS NULL
ORDER BY s.updated_at DESC, s.created_at DESC, s.id ASC
