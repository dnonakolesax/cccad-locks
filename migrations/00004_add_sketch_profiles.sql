-- +goose Up
ALTER TABLE sketch_current_states
    ADD COLUMN IF NOT EXISTS profiles JSONB NOT NULL DEFAULT '[]'::jsonb;

UPDATE sketch_current_states
SET profiles = materialized_geometry->'profiles'
WHERE jsonb_typeof(materialized_geometry->'profiles') = 'array';

-- +goose StatementBegin
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'sketch_current_states_profiles_array'
    ) THEN
        ALTER TABLE sketch_current_states
            ADD CONSTRAINT sketch_current_states_profiles_array CHECK (jsonb_typeof(profiles) = 'array');
    END IF;
END;
$$;
-- +goose StatementEnd

-- +goose Down
ALTER TABLE sketch_current_states
    DROP CONSTRAINT IF EXISTS sketch_current_states_profiles_array;

ALTER TABLE sketch_current_states
    DROP COLUMN IF EXISTS profiles;
