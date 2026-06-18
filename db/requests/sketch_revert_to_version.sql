WITH authorized AS (
    SELECT
        s.id,
        s.workspace_id,
        s.name,
        s.created_by_user_id,
        s.unit,
        s.plane,
        s.version AS current_version
    FROM sketches s
    WHERE s.id = $1::uuid
        AND s.created_by_user_id = $3
        AND s.deleted_at IS NULL
    FOR UPDATE OF s
),
target_snapshot AS (
    SELECT
        ss.sketch_id,
        ss.version,
        ss.graph_state,
        ss.materialized_geometry,
        ss.solve_status
    FROM sketch_snapshots ss
    JOIN authorized a ON a.id = ss.sketch_id
    WHERE ss.version = $2
),
target_operation AS (
    SELECT
        so.sketch_id,
        so.version,
        so.graph_state,
        so.materialized_geometry,
        COALESCE(so.solve_status, '{}'::jsonb) AS solve_status
    FROM sketch_ops so
    JOIN authorized a ON a.id = so.sketch_id
    WHERE so.version = $2
        AND so.graph_state IS NOT NULL
        AND so.materialized_geometry IS NOT NULL
        AND NOT EXISTS (SELECT 1 FROM target_snapshot)
),
target_current AS (
    SELECT
        st.sketch_id,
        st.version,
        st.graph_state,
        st.materialized_geometry,
        st.solve_status
    FROM sketch_current_states st
    JOIN authorized a ON a.id = st.sketch_id
    WHERE st.version = $2
        AND NOT EXISTS (SELECT 1 FROM target_snapshot)
        AND NOT EXISTS (SELECT 1 FROM target_operation)
),
target_state AS (
    SELECT
        sketch_id,
        version,
        graph_state,
        materialized_geometry,
        solve_status
    FROM target_snapshot
    UNION ALL
    SELECT
        sketch_id,
        version,
        graph_state,
        materialized_geometry,
        solve_status
    FROM target_operation
    UNION ALL
    SELECT
        sketch_id,
        version,
        graph_state,
        materialized_geometry,
        solve_status
    FROM target_current
    LIMIT 1
),
target_snapshot_insert AS (
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
    FROM target_state
    ON CONFLICT (sketch_id, version) DO NOTHING
    RETURNING sketch_id
),
revert_op AS (
    INSERT INTO sketch_ops (
        sketch_id,
        version,
        actor_user_id,
        client_op_id,
        base_version,
        op_type,
        payload,
        materialized_patch,
        graph_state,
        materialized_geometry,
        solve_status
    )
    SELECT
        a.id,
        a.current_version + 1,
        $3,
        'server-revert-' || gen_random_uuid()::text,
        a.current_version,
        'revert_to_version',
        jsonb_build_object(
            'type', 'revert_to_version',
            'targetVersion', ts.version,
            'previousVersion', a.current_version
        ),
        jsonb_build_object(
            'type', 'revert_to_version',
            'targetVersion', ts.version
        ),
        ts.graph_state,
        ts.materialized_geometry,
        ts.solve_status
    FROM authorized a
    JOIN target_state ts ON ts.sketch_id = a.id
    WHERE a.current_version <> ts.version
    RETURNING
        sketch_id,
        version
),
state_update AS (
    UPDATE sketch_current_states st
    SET
        version = ro.version,
        graph_state = ts.graph_state,
        materialized_geometry = ts.materialized_geometry,
        solve_status = ts.solve_status,
        profiles = CASE
            WHEN jsonb_typeof(ts.materialized_geometry->'profiles') = 'array'
                THEN ts.materialized_geometry->'profiles'
            ELSE '[]'::jsonb
        END
    FROM revert_op ro
    JOIN target_state ts ON ts.sketch_id = ro.sketch_id
    WHERE st.sketch_id = ro.sketch_id
    RETURNING
        st.sketch_id,
        st.version,
        st.graph_state,
        st.solve_status,
        st.profiles
),
sketch_update AS (
    UPDATE sketches s
    SET version = ro.version
    FROM revert_op ro
    WHERE s.id = ro.sketch_id
    RETURNING s.id
),
final_state AS (
    SELECT
        st.sketch_id,
        st.version,
        st.graph_state,
        st.solve_status,
        st.profiles
    FROM state_update st
    UNION ALL
    SELECT
        st.sketch_id,
        st.version,
        st.graph_state,
        st.solve_status,
        st.profiles
    FROM sketch_current_states st
    JOIN authorized a ON a.id = st.sketch_id
    JOIN target_state ts ON ts.sketch_id = st.sketch_id
    WHERE a.current_version = ts.version
        AND NOT EXISTS (SELECT 1 FROM state_update)
    LIMIT 1
)
SELECT
    a.id::text,
    a.workspace_id::text,
    a.name,
    a.created_by_user_id,
    a.unit::text,
    a.plane,
    fs.version,
    COALESCE(fs.graph_state->'entities', '{}'::jsonb),
    COALESCE(fs.graph_state->'constraints', '{}'::jsonb),
    COALESCE(fs.graph_state->'dimensions', '{}'::jsonb),
    COALESCE(fs.graph_state->'groups', '{}'::jsonb),
    fs.solve_status,
    fs.profiles,
    COALESCE(
        jsonb_agg(c.payload ORDER BY c.created_at DESC) FILTER (WHERE c.id IS NOT NULL),
        '[]'::jsonb
    )
FROM authorized a
JOIN final_state fs ON fs.sketch_id = a.id
LEFT JOIN sketch_conflicts c
    ON c.sketch_id = a.id
    AND c.status = 'open'
GROUP BY
    a.id,
    a.workspace_id,
    a.name,
    a.created_by_user_id,
    a.unit,
    a.plane,
    fs.version,
    fs.graph_state,
    fs.profiles,
    fs.solve_status
