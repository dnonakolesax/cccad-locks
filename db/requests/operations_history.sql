WITH latest AS (
    SELECT
        so.id,
        so.sketch_id,
        so.version,
        so.actor_user_id,
        so.client_op_id,
        so.created_at,
        so.payload,
        so.materialized_patch,
        so.solve_status
    FROM sketch_ops so
    JOIN sketches s ON s.id = so.sketch_id
    JOIN sketch_permissions sp ON sp.sketch_id = so.sketch_id
    WHERE so.sketch_id = $1::uuid
        AND sp.user_id = $2
        AND sp.role IN ('reader', 'editor', 'admin')
        AND s.deleted_at IS NULL
    ORDER BY so.version DESC
    LIMIT $3
)
SELECT
    latest.id::text,
    latest.sketch_id::text,
    latest.version,
    latest.actor_user_id,
    latest.client_op_id,
    latest.created_at,
    latest.payload,
    COALESCE(latest.materialized_patch, '{}'::jsonb),
    COALESCE(latest.solve_status, '{}'::jsonb)
FROM latest
ORDER BY latest.version ASC
