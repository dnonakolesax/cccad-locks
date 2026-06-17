SELECT
    ss.sketch_id::text,
    ss.version,
    ss.graph_state,
    ss.materialized_geometry,
    ss.solve_status,
    ss.created_at
FROM sketch_snapshots ss
JOIN sketches s ON s.id = ss.sketch_id
JOIN sketch_permissions sp ON sp.sketch_id = ss.sketch_id
WHERE ss.sketch_id = $1::uuid
    AND ss.version = $2
    AND sp.user_id = $3
    AND sp.role IN ('reader', 'editor', 'admin')
    AND s.deleted_at IS NULL
