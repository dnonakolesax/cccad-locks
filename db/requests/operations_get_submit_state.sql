SELECT
    st.version,
    st.graph_state,
    st.materialized_geometry,
    st.solve_status
FROM sketches s
JOIN sketch_current_states st ON st.sketch_id = s.id
JOIN sketch_permissions sp ON sp.sketch_id = s.id
WHERE s.id = $1::uuid
    AND sp.user_id = $2
    AND sp.role IN ('editor', 'admin')
    AND s.deleted_at IS NULL
