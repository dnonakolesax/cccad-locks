WITH authorized AS (
    SELECT
        s.id,
        s.version
    FROM sketches s
    JOIN sketch_permissions sp ON sp.sketch_id = s.id
    WHERE s.id = $1::uuid
        AND sp.user_id = $2
        AND sp.role IN ('editor', 'admin')
        AND s.deleted_at IS NULL
    FOR UPDATE OF s
),
duplicate AS (
    SELECT
        so.id::text,
        so.version
    FROM sketch_ops so
    WHERE so.sketch_id = $1::uuid
        AND so.actor_user_id = $2
        AND so.client_op_id = $3
),
inserted AS (
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
        a.version + 1,
        $2,
        $3,
        $4,
        $5,
        $6::jsonb,
        $7::jsonb,
        $8::jsonb,
        $9::jsonb,
        $10::jsonb
    FROM authorized a
    WHERE NOT EXISTS (SELECT 1 FROM duplicate)
        AND a.version = $4
    RETURNING
        id::text,
        sketch_id,
        version
),
state_update AS (
    UPDATE sketch_current_states st
    SET
        version = i.version,
        graph_state = $8::jsonb,
        materialized_geometry = $9::jsonb,
        solve_status = $10::jsonb,
        profiles = $11::jsonb
    FROM inserted i
    WHERE st.sketch_id = i.sketch_id
    RETURNING st.sketch_id
),
sketch_update AS (
    UPDATE sketches s
    SET version = i.version
    FROM inserted i
    WHERE s.id = i.sketch_id
    RETURNING s.id
),
current_version AS (
    SELECT COALESCE(
        (SELECT version FROM authorized),
        (SELECT version FROM sketches WHERE id = $1::uuid)
    ) AS version
)
SELECT
    'committed'::text AS status,
    i.id,
    i.version,
    i.version AS current_version,
    false AS duplicate
FROM inserted i
UNION ALL
SELECT
    'duplicate'::text AS status,
    d.id,
    d.version,
    d.version AS current_version,
    true AS duplicate
FROM duplicate d
WHERE NOT EXISTS (SELECT 1 FROM inserted)
UNION ALL
SELECT
    'stale_version'::text AS status,
    ''::text AS id,
    0::bigint AS version,
    cv.version AS current_version,
    false AS duplicate
FROM current_version cv
WHERE EXISTS (SELECT 1 FROM authorized)
    AND NOT EXISTS (SELECT 1 FROM duplicate)
    AND NOT EXISTS (SELECT 1 FROM inserted)
    AND cv.version <> $4
UNION ALL
SELECT
    CASE
        WHEN EXISTS (SELECT 1 FROM sketches WHERE id = $1::uuid AND deleted_at IS NULL)
            THEN 'permission_denied'
        ELSE 'not_found'
    END::text AS status,
    ''::text AS id,
    0::bigint AS version,
    0::bigint AS current_version,
    false AS duplicate
WHERE NOT EXISTS (SELECT 1 FROM authorized)
    AND NOT EXISTS (SELECT 1 FROM duplicate)
    AND NOT EXISTS (SELECT 1 FROM inserted)
LIMIT 1
