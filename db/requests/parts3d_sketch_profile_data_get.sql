SELECT
    COALESCE(st.profiles, '[]'::jsonb),
    COALESCE(st.graph_state->'entities', '{}'::jsonb) ||
        COALESCE(st.materialized_geometry->'entities', '{}'::jsonb)
FROM sketches s
JOIN sketch_current_states st ON st.sketch_id = s.id
WHERE s.id = $1::uuid
  AND s.deleted_at IS NULL
