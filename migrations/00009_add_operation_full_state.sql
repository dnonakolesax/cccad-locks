-- +goose Up
ALTER TABLE sketch_ops
    ADD COLUMN IF NOT EXISTS graph_state JSONB,
    ADD COLUMN IF NOT EXISTS materialized_geometry JSONB;

-- +goose StatementBegin
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'sketch_ops_graph_state_object'
            AND conrelid = 'sketch_ops'::regclass
    ) THEN
        ALTER TABLE sketch_ops
            ADD CONSTRAINT sketch_ops_graph_state_object CHECK (
                graph_state IS NULL OR jsonb_typeof(graph_state) = 'object'
            );
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'sketch_ops_materialized_geometry_object'
            AND conrelid = 'sketch_ops'::regclass
    ) THEN
        ALTER TABLE sketch_ops
            ADD CONSTRAINT sketch_ops_materialized_geometry_object CHECK (
                materialized_geometry IS NULL OR jsonb_typeof(materialized_geometry) = 'object'
            );
    END IF;
END $$;
-- +goose StatementEnd

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
FROM sketch_current_states
ON CONFLICT (sketch_id, version) DO NOTHING;

-- +goose Down
ALTER TABLE sketch_ops
    DROP CONSTRAINT IF EXISTS sketch_ops_materialized_geometry_object,
    DROP CONSTRAINT IF EXISTS sketch_ops_graph_state_object;

ALTER TABLE sketch_ops
    DROP COLUMN IF EXISTS materialized_geometry,
    DROP COLUMN IF EXISTS graph_state;
