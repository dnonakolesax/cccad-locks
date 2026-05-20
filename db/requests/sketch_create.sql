WITH new_sketch AS (
    INSERT INTO sketches (
        id,
        workspace_id,
        name,
        unit,
        created_by_user_id,
        version
    )
    VALUES (
        gen_random_uuid(),
        $1::uuid,
        $2,
        COALESCE(NULLIF($3, ''), 'mm')::sketch_unit,
        $4,
        0
    )
    RETURNING
        id,
        workspace_id,
        name,
        unit,
        created_by_user_id,
        version,
        created_at,
        updated_at
),
new_state AS (
    INSERT INTO sketch_current_states (
        sketch_id,
        version,
        graph_state,
        materialized_geometry,
        solve_status
    )
    SELECT
        id,
        0,
        '{
          "entities": {},
          "constraints": {},
          "dimensions": {},
          "groups": {}
        }'::jsonb,
        '{
          "entities": {}
        }'::jsonb,
        '{
          "status": "ok",
          "degreesOfFreedom": 0,
          "diagnostics": []
        }'::jsonb
    FROM new_sketch
    RETURNING sketch_id
)
SELECT
    ns.id::text,
    ns.workspace_id::text,
    ns.name,
    ns.created_by_user_id,
    ns.unit::text,
    ns.version,
    ns.created_at,
    ns.updated_at
FROM new_sketch ns
JOIN new_state st ON st.sketch_id = ns.id
