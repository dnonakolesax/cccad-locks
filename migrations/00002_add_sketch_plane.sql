-- +goose Up
ALTER TABLE sketches
    ADD COLUMN IF NOT EXISTS plane JSONB;

UPDATE sketches
SET plane = '{
  "origin": {"x": 0, "y": 0, "z": 0},
  "normal": {"x": 0, "y": 0, "z": 1},
  "xAxis": {"x": 1, "y": 0, "z": 0}
}'::jsonb
WHERE plane IS NULL;

ALTER TABLE sketches
    ALTER COLUMN plane SET NOT NULL;

-- +goose StatementBegin
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'sketches_plane_object'
    ) THEN
        ALTER TABLE sketches
            ADD CONSTRAINT sketches_plane_object CHECK (jsonb_typeof(plane) = 'object');
    END IF;
END;
$$;
-- +goose StatementEnd

-- +goose Down
ALTER TABLE sketches
    DROP CONSTRAINT IF EXISTS sketches_plane_object;

ALTER TABLE sketches
    DROP COLUMN IF EXISTS plane;
