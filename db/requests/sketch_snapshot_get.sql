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
    FROM authorized_current
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
