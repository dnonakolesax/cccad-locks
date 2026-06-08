-- +goose Up
ALTER TABLE part_representations_3d
    DROP CONSTRAINT IF EXISTS part_representations_3d_body_id_fkey;

ALTER TABLE topology_refs_3d
    DROP CONSTRAINT IF EXISTS topology_refs_3d_body_id_fkey;

ALTER TABLE feature_build_results_3d
    DROP CONSTRAINT IF EXISTS feature_build_results_3d_created_body_id_fkey;

ALTER TABLE part_bodies_3d
    ALTER COLUMN id TYPE TEXT USING id::text;

ALTER TABLE part_representations_3d
    ALTER COLUMN body_id TYPE TEXT USING body_id::text;

ALTER TABLE topology_refs_3d
    ALTER COLUMN body_id TYPE TEXT USING body_id::text;

ALTER TABLE feature_build_results_3d
    ALTER COLUMN created_body_id TYPE TEXT USING created_body_id::text;

ALTER TABLE part_representations_3d
    ADD CONSTRAINT part_representations_3d_body_id_fkey
    FOREIGN KEY (body_id) REFERENCES part_bodies_3d(id) ON DELETE CASCADE;

ALTER TABLE topology_refs_3d
    ADD CONSTRAINT topology_refs_3d_body_id_fkey
    FOREIGN KEY (body_id) REFERENCES part_bodies_3d(id) ON DELETE CASCADE;

ALTER TABLE feature_build_results_3d
    ADD CONSTRAINT feature_build_results_3d_created_body_id_fkey
    FOREIGN KEY (created_body_id) REFERENCES part_bodies_3d(id) ON DELETE SET NULL;

-- +goose Down
ALTER TABLE part_representations_3d
    DROP CONSTRAINT IF EXISTS part_representations_3d_body_id_fkey;

ALTER TABLE topology_refs_3d
    DROP CONSTRAINT IF EXISTS topology_refs_3d_body_id_fkey;

ALTER TABLE feature_build_results_3d
    DROP CONSTRAINT IF EXISTS feature_build_results_3d_created_body_id_fkey;

ALTER TABLE part_bodies_3d
    ALTER COLUMN id TYPE UUID USING id::uuid;

ALTER TABLE part_representations_3d
    ALTER COLUMN body_id TYPE UUID USING body_id::uuid;

ALTER TABLE topology_refs_3d
    ALTER COLUMN body_id TYPE UUID USING body_id::uuid;

ALTER TABLE feature_build_results_3d
    ALTER COLUMN created_body_id TYPE UUID USING created_body_id::uuid;

ALTER TABLE part_representations_3d
    ADD CONSTRAINT part_representations_3d_body_id_fkey
    FOREIGN KEY (body_id) REFERENCES part_bodies_3d(id) ON DELETE CASCADE;

ALTER TABLE topology_refs_3d
    ADD CONSTRAINT topology_refs_3d_body_id_fkey
    FOREIGN KEY (body_id) REFERENCES part_bodies_3d(id) ON DELETE CASCADE;

ALTER TABLE feature_build_results_3d
    ADD CONSTRAINT feature_build_results_3d_created_body_id_fkey
    FOREIGN KEY (created_body_id) REFERENCES part_bodies_3d(id) ON DELETE SET NULL;
