WITH authorized AS (
    SELECT s.id
    FROM sketches s
    JOIN sketch_permissions sp ON sp.sketch_id = s.id
    WHERE s.id = $1::uuid
        AND sp.user_id = $3
        AND sp.role IN ('reader', 'editor', 'admin')
        AND s.deleted_at IS NULL
),
current_entity AS (
    SELECT 1
    FROM sketch_current_states scs
    JOIN authorized a ON a.id = scs.sketch_id
    WHERE (scs.graph_state->'entities' ? $2)
        OR (scs.materialized_geometry->'entities' ? $2)
),
historical_entity AS (
    SELECT
        so.sketch_id,
        so.version,
        COALESCE(so.graph_state->'entities'->$2, '{}'::jsonb) AS entity,
        COALESCE(so.materialized_geometry->'entities'->$2, so.graph_state->'entities'->$2, '{}'::jsonb) AS materialized_geometry
    FROM sketch_ops so
    JOIN authorized a ON a.id = so.sketch_id
    WHERE NOT EXISTS (SELECT 1 FROM current_entity)
        AND (
            so.graph_state->'entities' ? $2
            OR so.materialized_geometry->'entities' ? $2
        )
    ORDER BY so.version DESC
    LIMIT 1
)
SELECT
    sketch_id::text,
    $2::text AS entity_id,
    version,
    entity,
    materialized_geometry
FROM historical_entity
