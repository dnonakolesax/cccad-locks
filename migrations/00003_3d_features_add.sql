-- +goose Up
-- +goose StatementBegin
CREATE EXTENSION IF NOT EXISTS pgcrypto;
-- +goose StatementEnd

-- +goose StatementBegin
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'feature_3d_type') THEN
        CREATE TYPE feature_3d_type AS ENUM (
            'extrude',
            'boolean',
            'hole',
            'pattern',
            'fillet',
            'chamfer'
        );
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'feature_3d_build_status') THEN
        CREATE TYPE feature_3d_build_status AS ENUM (
            'pending',
            'running',
            'success',
            'failed',
            'cancelled'
        );
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'body_3d_representation_kind') THEN
        CREATE TYPE body_3d_representation_kind AS ENUM (
            'brep',
            'glb',
            'mesh_json',
            'step',
            'stl'
        );
    END IF;
END $$;
-- +goose StatementEnd

-- Ordered parametric 3D feature history.
-- payload examples:
-- extrude: {"sketchId":"...", "profileId":"profile-1", "depth":30, "direction":"forward", "operation":"new_body", "targetBodyId":null}
-- boolean: {"operation":"unite|subtract|intersect", "targetBodyId":"...", "toolBodyIds":["..."]}
-- hole:    {"sketchId":"...", "center":{"x":0,"y":0}, "diameter":10, "depth":20, "throughAll":false, "direction":"forward", "targetBodyId":"..."}
-- pattern: {"sourceFeatureIds":["..."], "linear":{"direction":{"x":1,"y":0,"z":0}, "count":4, "spacing":20}}
-- fillet:  {"targetBodyId":"...", "edgeRefs":["feature:.../edge:..."], "radius":3}
-- chamfer: {"targetBodyId":"...", "edgeRefs":["feature:.../edge:..."], "distance":1}
CREATE TABLE IF NOT EXISTS features_3d (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    part_id UUID NOT NULL REFERENCES parts(id) ON DELETE CASCADE,
    sketch_id UUID REFERENCES sketches(id) ON DELETE RESTRICT,

    type feature_3d_type NOT NULL,
    payload JSONB NOT NULL,

    order_index INTEGER NOT NULL,
    suppressed BOOLEAN NOT NULL DEFAULT FALSE,
    deleted_at TIMESTAMPTZ,

    created_by UUID NOT NULL REFERENCES users(id) ON DELETE RESTRICT,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT features_3d_order_index_nonnegative CHECK (order_index >= 0),
    CONSTRAINT features_3d_payload_object CHECK (jsonb_typeof(payload) = 'object'),
    CONSTRAINT features_3d_unique_order_active UNIQUE (part_id, order_index)
);

CREATE INDEX IF NOT EXISTS idx_features_3d_part_order
    ON features_3d(part_id, order_index)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_features_3d_part_type
    ON features_3d(part_id, type)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_features_3d_sketch
    ON features_3d(sketch_id)
    WHERE sketch_id IS NOT NULL AND deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_features_3d_payload_gin
    ON features_3d USING GIN(payload);

-- Current/generated solid bodies. Bodies are derived from feature history but cached as addressable entities.
CREATE TABLE IF NOT EXISTS part_bodies_3d (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    part_id UUID NOT NULL REFERENCES parts(id) ON DELETE CASCADE,
    name TEXT NOT NULL,
    created_by_feature_id UUID REFERENCES features_3d(id) ON DELETE SET NULL,

    active BOOLEAN NOT NULL DEFAULT TRUE,
    stable_ref TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT part_bodies_3d_name_not_empty CHECK (length(trim(name)) > 0)
);

CREATE INDEX IF NOT EXISTS idx_part_bodies_3d_part_active
    ON part_bodies_3d(part_id, active);

CREATE INDEX IF NOT EXISTS idx_part_bodies_3d_feature
    ON part_bodies_3d(created_by_feature_id)
    WHERE created_by_feature_id IS NOT NULL;

-- BREP/GLB/mesh/STEP/STL artifacts. Store binary data in S3/object storage, not in PostgreSQL.
CREATE TABLE IF NOT EXISTS part_representations_3d (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    part_id UUID NOT NULL REFERENCES parts(id) ON DELETE CASCADE,
    body_id UUID REFERENCES part_bodies_3d(id) ON DELETE CASCADE,

    document_version BIGINT NOT NULL,
    kind body_3d_representation_kind NOT NULL,
    storage_key TEXT NOT NULL,
    content_type TEXT,
    size_bytes BIGINT,
    sha256 TEXT,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT part_representations_3d_version_nonnegative CHECK (document_version >= 0),
    CONSTRAINT part_representations_3d_storage_key_not_empty CHECK (length(trim(storage_key)) > 0),
    CONSTRAINT part_representations_3d_size_nonnegative CHECK (size_bytes IS NULL OR size_bytes >= 0),
    CONSTRAINT part_representations_3d_sha256_format CHECK (sha256 IS NULL OR sha256 ~ '^[a-f0-9]{64}$')
);

CREATE INDEX IF NOT EXISTS idx_part_representations_3d_part_version_kind
    ON part_representations_3d(part_id, document_version DESC, kind);

CREATE INDEX IF NOT EXISTS idx_part_representations_3d_body_kind
    ON part_representations_3d(body_id, kind)
    WHERE body_id IS NOT NULL;

CREATE UNIQUE INDEX IF NOT EXISTS uq_part_representations_3d_storage_key
    ON part_representations_3d(storage_key);

-- Rebuild jobs/results for the geometry solver.
CREATE TABLE IF NOT EXISTS part_rebuilds_3d (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    part_id UUID NOT NULL REFERENCES parts(id) ON DELETE CASCADE,
    document_version BIGINT NOT NULL,

    status feature_3d_build_status NOT NULL DEFAULT 'pending',
    started_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,

    failed_feature_id UUID REFERENCES features_3d(id) ON DELETE SET NULL,
    diagnostics JSONB NOT NULL DEFAULT '[]'::jsonb,

    created_by UUID REFERENCES users(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT part_rebuilds_3d_version_nonnegative CHECK (document_version >= 0),
    CONSTRAINT part_rebuilds_3d_diagnostics_array CHECK (jsonb_typeof(diagnostics) = 'array'),
    CONSTRAINT part_rebuilds_3d_time_order CHECK (finished_at IS NULL OR started_at IS NULL OR finished_at >= started_at)
);

CREATE INDEX IF NOT EXISTS idx_part_rebuilds_3d_part_version
    ON part_rebuilds_3d(part_id, document_version DESC);

CREATE INDEX IF NOT EXISTS idx_part_rebuilds_3d_status
    ON part_rebuilds_3d(status);

-- Per-feature build status. Useful for feature tree diagnostics.
CREATE TABLE IF NOT EXISTS feature_build_results_3d (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    rebuild_id UUID REFERENCES part_rebuilds_3d(id) ON DELETE CASCADE,
    feature_id UUID NOT NULL REFERENCES features_3d(id) ON DELETE CASCADE,
    part_id UUID NOT NULL REFERENCES parts(id) ON DELETE CASCADE,

    status feature_3d_build_status NOT NULL,
    diagnostics JSONB NOT NULL DEFAULT '[]'::jsonb,

    created_body_id UUID REFERENCES part_bodies_3d(id) ON DELETE SET NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT feature_build_results_3d_diagnostics_array CHECK (jsonb_typeof(diagnostics) = 'array')
);

CREATE INDEX IF NOT EXISTS idx_feature_build_results_3d_feature
    ON feature_build_results_3d(feature_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_feature_build_results_3d_rebuild
    ON feature_build_results_3d(rebuild_id);

-- Lightweight topology cache for selection. Detailed BREP remains in object storage.
CREATE TABLE IF NOT EXISTS topology_refs_3d (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    part_id UUID NOT NULL REFERENCES parts(id) ON DELETE CASCADE,
    body_id UUID NOT NULL REFERENCES part_bodies_3d(id) ON DELETE CASCADE,
    document_version BIGINT NOT NULL,

    ref_kind TEXT NOT NULL,
    ref_id TEXT NOT NULL,
    stable_ref TEXT,
    parent_ref_id TEXT,

    surface_or_curve_type TEXT,
    payload JSONB NOT NULL DEFAULT '{}'::jsonb,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT topology_refs_3d_ref_kind_check CHECK (ref_kind IN ('body', 'shell', 'face', 'loop', 'edge', 'vertex')),
    CONSTRAINT topology_refs_3d_version_nonnegative CHECK (document_version >= 0),
    CONSTRAINT topology_refs_3d_payload_object CHECK (jsonb_typeof(payload) = 'object')
);

CREATE INDEX IF NOT EXISTS idx_topology_refs_3d_part_version
    ON topology_refs_3d(part_id, document_version DESC);

CREATE INDEX IF NOT EXISTS idx_topology_refs_3d_body_kind
    ON topology_refs_3d(body_id, ref_kind);

CREATE UNIQUE INDEX IF NOT EXISTS uq_topology_refs_3d_version_ref
    ON topology_refs_3d(part_id, document_version, body_id, ref_kind, ref_id);

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION set_updated_at_3d()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

DROP TRIGGER IF EXISTS trg_features_3d_updated_at ON features_3d;
CREATE TRIGGER trg_features_3d_updated_at
BEFORE UPDATE ON features_3d
FOR EACH ROW EXECUTE FUNCTION set_updated_at_3d();

DROP TRIGGER IF EXISTS trg_part_bodies_3d_updated_at ON part_bodies_3d;
CREATE TRIGGER trg_part_bodies_3d_updated_at
BEFORE UPDATE ON part_bodies_3d
FOR EACH ROW EXECUTE FUNCTION set_updated_at_3d();

-- +goose Down
DROP TRIGGER IF EXISTS trg_part_bodies_3d_updated_at ON part_bodies_3d;
DROP TRIGGER IF EXISTS trg_features_3d_updated_at ON features_3d;
DROP FUNCTION IF EXISTS set_updated_at_3d();

DROP TABLE IF EXISTS topology_refs_3d;
DROP TABLE IF EXISTS feature_build_results_3d;
DROP TABLE IF EXISTS part_representations_3d;
DROP TABLE IF EXISTS part_rebuilds_3d;
DROP TABLE IF EXISTS part_bodies_3d;
DROP TABLE IF EXISTS features_3d;

DROP TYPE IF EXISTS body_3d_representation_kind;
DROP TYPE IF EXISTS feature_3d_build_status;
DROP TYPE IF EXISTS feature_3d_type;
