SELECT
    so.id::text,
    so.sketch_id::text,
    so.version,
    so.actor_user_id,
    so.client_op_id,
    so.created_at,
    so.payload,
    COALESCE(so.materialized_patch, '{}'::jsonb),
    COALESCE(so.solve_status, '{}'::jsonb)
FROM sketch_ops so
JOIN sketches s ON s.id = so.sketch_id
JOIN sketch_permissions sp ON sp.sketch_id = so.sketch_id
WHERE so.sketch_id = $1::uuid
    AND sp.user_id = $2
    AND sp.role IN ('reader', 'editor', 'admin')
    AND s.deleted_at IS NULL
    AND so.version > $3
ORDER BY so.version ASC
LIMIT $4
