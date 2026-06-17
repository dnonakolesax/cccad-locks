WITH existing_exact AS (
    SELECT
        ss.sketch_id,
        ss.version
    FROM sketch_snapshots ss
    JOIN sketches s ON s.id = ss.sketch_id
    JOIN sketch_permissions sp ON sp.sketch_id = ss.sketch_id
    WHERE ss.sketch_id = $1::uuid
        AND ss.version = $2
        AND sp.user_id = $3
        AND sp.role IN ('reader', 'editor', 'admin')
        AND s.deleted_at IS NULL
),
authorized_current_exact AS (
    SELECT
        st.sketch_id,
        st.version,
        st.graph_state,
        st.materialized_geometry,
        st.solve_status,
        1 AS source_rank
    FROM sketches s
    JOIN sketch_current_states st ON st.sketch_id = s.id
    JOIN sketch_permissions sp ON sp.sketch_id = s.id
    WHERE s.id = $1::uuid
        AND st.version = $2
        AND sp.user_id = $3
        AND sp.role IN ('reader', 'editor', 'admin')
        AND s.deleted_at IS NULL
        AND NOT EXISTS (SELECT 1 FROM existing_exact)
),
authorized_operation_exact AS (
    SELECT
        so.sketch_id,
        so.version,
        so.graph_state,
        so.materialized_geometry,
        COALESCE(so.solve_status, '{}'::jsonb) AS solve_status,
        2 AS source_rank
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
        AND NOT EXISTS (SELECT 1 FROM existing_exact)
),
authorized_current_fallback AS (
    SELECT
        st.sketch_id,
        st.version,
        st.graph_state,
        st.materialized_geometry,
        st.solve_status,
        3 AS source_rank
    FROM sketches s
    JOIN sketch_current_states st ON st.sketch_id = s.id
    JOIN sketch_permissions sp ON sp.sketch_id = s.id
    WHERE s.id = $1::uuid
        AND sp.user_id = $3
        AND sp.role IN ('reader', 'editor', 'admin')
        AND s.deleted_at IS NULL
        AND NOT EXISTS (SELECT 1 FROM existing_exact)
),
snapshot_source AS (
    SELECT
        sketch_id,
        version,
        graph_state,
        materialized_geometry,
        solve_status
    FROM (
        SELECT * FROM authorized_current_exact
        UNION ALL
        SELECT * FROM authorized_operation_exact
        UNION ALL
        SELECT * FROM authorized_current_fallback
    ) candidate
    ORDER BY source_rank ASC
    LIMIT 1
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
selected_snapshot AS (
    SELECT
        sketch_id,
        version
    FROM existing_exact
    UNION ALL
    SELECT
        sketch_id,
        version
    FROM snapshot_source
    WHERE NOT EXISTS (SELECT 1 FROM existing_exact)
    LIMIT 1
),
existing_selected AS (
    SELECT
        ss.sketch_id::text,
        ss.version,
        ss.graph_state,
        ss.materialized_geometry,
        ss.solve_status,
        ss.created_at
    FROM sketch_snapshots ss
    JOIN selected_snapshot selected
        ON selected.sketch_id = ss.sketch_id
        AND selected.version = ss.version
)
SELECT
    sketch_id,
    version,
    graph_state,
    materialized_geometry,
    solve_status,
    created_at
FROM existing_selected
UNION ALL
SELECT
    sketch_id,
    version,
    graph_state,
    materialized_geometry,
    solve_status,
    created_at
FROM inserted
WHERE NOT EXISTS (SELECT 1 FROM existing_selected)
LIMIT 1
