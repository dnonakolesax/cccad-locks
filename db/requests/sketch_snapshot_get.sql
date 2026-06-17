WITH authorized_current AS (
    SELECT
        st.sketch_id,
        st.version,
        st.graph_state,
        st.materialized_geometry,
        st.solve_status
    FROM sketches s
    JOIN sketch_current_states st ON st.sketch_id = s.id
    JOIN sketch_permissions sp ON sp.sketch_id = s.id
    WHERE s.id = $1::uuid
        AND st.version = $2
        AND sp.user_id = $3
        AND sp.role IN ('reader', 'editor', 'admin')
        AND s.deleted_at IS NULL
),
authorized_operation AS (
    SELECT
        so.sketch_id,
        so.version,
        so.graph_state,
        so.materialized_geometry,
        so.solve_status
    FROM sketch_ops so
    JOIN sketches s ON s.id = so.sketch_id
    JOIN sketch_permissions sp ON sp.sketch_id = so.sketch_id
    WHERE so.sketch_id = $1::uuid
        AND so.version = $2
        AND so.graph_state IS NOT NULL
        AND so.materialized_geometry IS NOT NULL
        AND sp.user_id = $3
        AND sp.role IN ('reader', 'editor', 'admin')
        AND s.deleted_at IS NULL
),
snapshot_source AS (
    SELECT
        sketch_id,
        version,
        graph_state,
        materialized_geometry,
        solve_status
    FROM authorized_current
    UNION ALL
    SELECT
        sketch_id,
        version,
        graph_state,
        materialized_geometry,
        COALESCE(solve_status, '{}'::jsonb)
    FROM authorized_operation
    WHERE NOT EXISTS (SELECT 1 FROM authorized_current)
),
inserted AS (
    INSERT INTO sketch_snapshots (
        sketch_id,
        version,
        graph_state,
        materialized_geometry,
        solve_status
    )
    SELECT
        sketch_id,
        version,
        graph_state,
        materialized_geometry,
        solve_status
    FROM snapshot_source
    ON CONFLICT (sketch_id, version) DO NOTHING
    RETURNING
        sketch_id::text,
        version,
        graph_state,
        materialized_geometry,
        solve_status,
        created_at
),
existing AS (
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
)
SELECT
    sketch_id,
    version,
    graph_state,
    materialized_geometry,
    solve_status,
    created_at
FROM inserted
UNION ALL
SELECT
    sketch_id,
    version,
    graph_state,
    materialized_geometry,
    solve_status,
    created_at
FROM existing
WHERE NOT EXISTS (SELECT 1 FROM inserted)
LIMIT 1
