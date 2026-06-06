WITH new_sketch AS (
    INSERT INTO sketches (
        id,
        workspace_id,
        name,
        unit,
        plane,
        created_by_user_id,
        version
    )
    VALUES (
        gen_random_uuid(),
        $1::uuid,
        $2,
        COALESCE(NULLIF($3, ''), 'mm')::sketch_unit,
        $4::jsonb,
        $5,
        0
    )
    RETURNING
        id,
        workspace_id,
        name,
        unit,
        plane,
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
        solve_status,
        profiles
    )
    SELECT
        id,
        0,
        '{
          "entities": {
            "zero-point": {
              "id": "zero-point",
              "type": "point",
              "x": 0,
              "y": 0,
              "fixed": true,
              "isConstruction": true
            },
            "x-axis-start": {
              "id": "x-axis-start",
              "type": "point",
              "x": -9999,
              "y": 0,
              "fixed": true,
              "isConstruction": true
            },
            "x-axis-end": {
              "id": "x-axis-end",
              "type": "point",
              "x": 9999,
              "y": 0,
              "fixed": true,
              "isConstruction": true
            },
            "x-axis": {
              "id": "x-axis",
              "type": "line",
              "startPointId": "x-axis-start",
              "endPointId": "x-axis-end",
              "isConstruction": true
            },
            "y-axis-start": {
              "id": "y-axis-start",
              "type": "point",
              "x": 0,
              "y": -9999,
              "fixed": true,
              "isConstruction": true
            },
            "y-axis-end": {
              "id": "y-axis-end",
              "type": "point",
              "x": 0,
              "y": 9999,
              "fixed": true,
              "isConstruction": true
            },
            "y-axis": {
              "id": "y-axis",
              "type": "line",
              "startPointId": "y-axis-start",
              "endPointId": "y-axis-end",
              "isConstruction": true
            }
          },
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
        }'::jsonb,
        '[]'::jsonb
    FROM new_sketch
    RETURNING sketch_id
)
SELECT
    ns.id::text,
    ns.workspace_id::text,
    ns.name,
    ns.created_by_user_id,
    ns.unit::text,
    ns.plane,
    ns.version,
    ns.created_at,
    ns.updated_at
FROM new_sketch ns
JOIN new_state st ON st.sketch_id = ns.id
