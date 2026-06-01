SELECT
    s.id::text,
    s.workspace_id::text,
    s.name,
    s.created_by_user_id,
    s.unit::text,
    s.plane,
    s.version,
    COALESCE(st.graph_state->'entities', '{}'::jsonb),
    COALESCE(st.graph_state->'constraints', '{}'::jsonb),
    COALESCE(st.graph_state->'dimensions', '{}'::jsonb),
    COALESCE(st.graph_state->'groups', '{}'::jsonb),
    st.solve_status,
    COALESCE(
        jsonb_agg(c.payload ORDER BY c.created_at DESC) FILTER (WHERE c.id IS NOT NULL),
        '[]'::jsonb
    )
FROM sketches s
JOIN sketch_current_states st ON st.sketch_id = s.id
LEFT JOIN sketch_conflicts c
    ON c.sketch_id = s.id
    AND c.status = 'open'
WHERE s.id = $1::uuid
    AND s.deleted_at IS NULL
GROUP BY
    s.id,
    s.workspace_id,
    s.name,
    s.created_by_user_id,
    s.unit,
    s.plane,
    s.version,
    st.graph_state,
    st.solve_status
