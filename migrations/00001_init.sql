-- +goose Up
CREATE EXTENSION IF NOT EXISTS pgcrypto;

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION set_updated_at()
RETURNS trigger AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- +goose StatementBegin
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'sketch_unit') THEN
        CREATE TYPE sketch_unit AS ENUM ('mm', 'cm', 'm', 'in', 'ft', 'px');
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'sketch_role') THEN
        CREATE TYPE sketch_role AS ENUM ('reader', 'editor', 'admin');
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'sketch_conflict_status') THEN
        CREATE TYPE sketch_conflict_status AS ENUM ('open', 'resolved', 'ignored');
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'sketch_file_kind') THEN
        CREATE TYPE sketch_file_kind AS ENUM (
            'preview_png',
            'preview_svg',
            'export_svg',
            'export_dxf',
            'export_pdf',
            'snapshot_blob',
            'attachment',
            'fem_mesh',
            'fem_result',
            'other'
        );
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'simulation_job_status') THEN
        CREATE TYPE simulation_job_status AS ENUM (
            'queued',
            'running',
            'succeeded',
            'failed',
            'cancelled'
        );
    END IF;
END $$;
-- +goose StatementEnd

CREATE TABLE IF NOT EXISTS user_profiles (
    user_id        TEXT PRIMARY KEY,
    display_name   TEXT,
    email          TEXT,
    avatar_url     TEXT,
    first_seen_at  TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_seen_at   TIMESTAMPTZ,
    metadata       JSONB NOT NULL DEFAULT '{}'::jsonb
);

CREATE TABLE IF NOT EXISTS workspaces (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name                TEXT NOT NULL,
    description         TEXT,
    created_by_user_id  TEXT NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at          TIMESTAMPTZ,
    metadata            JSONB NOT NULL DEFAULT '{}'::jsonb,

    CONSTRAINT workspaces_name_not_blank CHECK (length(trim(name)) > 0)
);

CREATE INDEX IF NOT EXISTS idx_workspaces_created_by_user_id
    ON workspaces (created_by_user_id);

CREATE INDEX IF NOT EXISTS idx_workspaces_not_deleted
    ON workspaces (id)
    WHERE deleted_at IS NULL;

CREATE TRIGGER trg_workspaces_set_updated_at
BEFORE UPDATE ON workspaces
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

CREATE TABLE IF NOT EXISTS sketches (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    workspace_id        UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
    name                TEXT NOT NULL,
    unit                sketch_unit NOT NULL DEFAULT 'mm',
    created_by_user_id  TEXT NOT NULL,
    version             BIGINT NOT NULL DEFAULT 0,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at          TIMESTAMPTZ,
    metadata            JSONB NOT NULL DEFAULT '{}'::jsonb,

    CONSTRAINT sketches_name_not_blank CHECK (length(trim(name)) > 0),
    CONSTRAINT sketches_version_non_negative CHECK (version >= 0)
);

CREATE INDEX IF NOT EXISTS idx_sketches_workspace_id
    ON sketches (workspace_id);

CREATE INDEX IF NOT EXISTS idx_sketches_created_by_user_id
    ON sketches (created_by_user_id);

CREATE INDEX IF NOT EXISTS idx_sketches_workspace_not_deleted
    ON sketches (workspace_id, updated_at DESC)
    WHERE deleted_at IS NULL;

CREATE TRIGGER trg_sketches_set_updated_at
BEFORE UPDATE ON sketches
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

CREATE TABLE IF NOT EXISTS sketch_permissions (
    sketch_id           UUID NOT NULL REFERENCES sketches(id) ON DELETE CASCADE,
    user_id             TEXT NOT NULL,
    role                sketch_role NOT NULL,
    granted_by_user_id  TEXT NOT NULL,
    granted_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at          TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (sketch_id, user_id)
);

CREATE INDEX IF NOT EXISTS idx_sketch_permissions_user_id
    ON sketch_permissions (user_id);

CREATE INDEX IF NOT EXISTS idx_sketch_permissions_sketch_role
    ON sketch_permissions (sketch_id, role);

CREATE TRIGGER trg_sketch_permissions_set_updated_at
BEFORE UPDATE ON sketch_permissions
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION ensure_sketch_creator_admin_permission()
RETURNS trigger AS $$
BEGIN
    INSERT INTO sketch_permissions (sketch_id, user_id, role, granted_by_user_id)
    VALUES (NEW.id, NEW.created_by_user_id, 'admin', NEW.created_by_user_id)
    ON CONFLICT (sketch_id, user_id)
    DO UPDATE SET role = 'admin', updated_at = now();

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER trg_sketches_creator_admin_permission
AFTER INSERT ON sketches
FOR EACH ROW
EXECUTE FUNCTION ensure_sketch_creator_admin_permission();

-- +goose StatementBegin
CREATE OR REPLACE FUNCTION protect_sketch_creator_admin_permission()
RETURNS trigger AS $$
DECLARE
    creator_id TEXT;
BEGIN
    SELECT created_by_user_id INTO creator_id
    FROM sketches
    WHERE id = COALESCE(OLD.sketch_id, NEW.sketch_id);

    IF TG_OP = 'DELETE' THEN
        IF OLD.user_id = creator_id AND OLD.role = 'admin' THEN
            RAISE EXCEPTION 'cannot delete sketch creator admin permission'
                USING ERRCODE = 'check_violation';
        END IF;
        RETURN OLD;
    END IF;

    IF TG_OP = 'UPDATE' THEN
        IF OLD.user_id = creator_id AND OLD.role = 'admin' AND NEW.role <> 'admin' THEN
            RAISE EXCEPTION 'cannot downgrade sketch creator admin permission'
                USING ERRCODE = 'check_violation';
        END IF;
        RETURN NEW;
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

CREATE TRIGGER trg_protect_sketch_creator_admin_permission_update
BEFORE UPDATE ON sketch_permissions
FOR EACH ROW
EXECUTE FUNCTION protect_sketch_creator_admin_permission();

CREATE TRIGGER trg_protect_sketch_creator_admin_permission_delete
BEFORE DELETE ON sketch_permissions
FOR EACH ROW
EXECUTE FUNCTION protect_sketch_creator_admin_permission();

CREATE TABLE IF NOT EXISTS sketch_permission_audit (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sketch_id           UUID NOT NULL REFERENCES sketches(id) ON DELETE CASCADE,
    target_user_id      TEXT NOT NULL,
    actor_user_id       TEXT NOT NULL,
    old_role            sketch_role,
    new_role            sketch_role,
    action              TEXT NOT NULL CHECK (action IN ('grant', 'update', 'revoke')),
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_sketch_permission_audit_sketch_id
    ON sketch_permission_audit (sketch_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_sketch_permission_audit_target_user_id
    ON sketch_permission_audit (target_user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS sketch_ops (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sketch_id               UUID NOT NULL REFERENCES sketches(id) ON DELETE CASCADE,
    version                 BIGINT NOT NULL,
    actor_user_id           TEXT NOT NULL,
    client_op_id            TEXT NOT NULL,
    base_version            BIGINT,
    op_type                 TEXT NOT NULL,
    payload                 JSONB NOT NULL,
    materialized_patch      JSONB,
    solve_status            JSONB,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT sketch_ops_version_positive CHECK (version > 0),
    CONSTRAINT sketch_ops_base_version_non_negative CHECK (base_version IS NULL OR base_version >= 0),
    CONSTRAINT sketch_ops_op_type_not_blank CHECK (length(trim(op_type)) > 0),
    CONSTRAINT sketch_ops_payload_is_object CHECK (jsonb_typeof(payload) = 'object'),
    CONSTRAINT sketch_ops_materialized_patch_is_object CHECK (
        materialized_patch IS NULL OR jsonb_typeof(materialized_patch) = 'object'
    ),
    CONSTRAINT sketch_ops_solve_status_is_object CHECK (
        solve_status IS NULL OR jsonb_typeof(solve_status) = 'object'
    ),

    UNIQUE (sketch_id, version),
    UNIQUE (sketch_id, actor_user_id, client_op_id)
);

CREATE INDEX IF NOT EXISTS idx_sketch_ops_sketch_version
    ON sketch_ops (sketch_id, version);

CREATE INDEX IF NOT EXISTS idx_sketch_ops_actor_user_id
    ON sketch_ops (actor_user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_sketch_ops_created_at
    ON sketch_ops (created_at DESC);

CREATE TABLE IF NOT EXISTS sketch_current_states (
    sketch_id               UUID PRIMARY KEY REFERENCES sketches(id) ON DELETE CASCADE,
    version                 BIGINT NOT NULL,
    graph_state             JSONB NOT NULL,
    materialized_geometry   JSONB NOT NULL,
    solve_status            JSONB NOT NULL,
    updated_at              TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT sketch_current_states_version_non_negative CHECK (version >= 0),
    CONSTRAINT sketch_current_states_graph_state_object CHECK (jsonb_typeof(graph_state) = 'object'),
    CONSTRAINT sketch_current_states_materialized_geometry_object CHECK (jsonb_typeof(materialized_geometry) = 'object'),
    CONSTRAINT sketch_current_states_solve_status_object CHECK (jsonb_typeof(solve_status) = 'object')
);

CREATE INDEX IF NOT EXISTS idx_sketch_current_states_version
    ON sketch_current_states (sketch_id, version);

CREATE TRIGGER trg_sketch_current_states_set_updated_at
BEFORE UPDATE ON sketch_current_states
FOR EACH ROW
EXECUTE FUNCTION set_updated_at();

CREATE TABLE IF NOT EXISTS sketch_snapshots (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sketch_id               UUID NOT NULL REFERENCES sketches(id) ON DELETE CASCADE,
    version                 BIGINT NOT NULL,
    graph_state             JSONB NOT NULL,
    materialized_geometry   JSONB NOT NULL,
    solve_status            JSONB NOT NULL,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT sketch_snapshots_version_non_negative CHECK (version >= 0),
    CONSTRAINT sketch_snapshots_graph_state_object CHECK (jsonb_typeof(graph_state) = 'object'),
    CONSTRAINT sketch_snapshots_materialized_geometry_object CHECK (jsonb_typeof(materialized_geometry) = 'object'),
    CONSTRAINT sketch_snapshots_solve_status_object CHECK (jsonb_typeof(solve_status) = 'object'),

    UNIQUE (sketch_id, version)
);

CREATE INDEX IF NOT EXISTS idx_sketch_snapshots_sketch_version_desc
    ON sketch_snapshots (sketch_id, version DESC);

CREATE TABLE IF NOT EXISTS sketch_conflicts (
    id                      UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sketch_id               UUID NOT NULL REFERENCES sketches(id) ON DELETE CASCADE,
    conflict_type           TEXT NOT NULL,
    status                  sketch_conflict_status NOT NULL DEFAULT 'open',
    caused_by_ops           JSONB NOT NULL,
    affected_entity_ids     JSONB NOT NULL,
    payload                 JSONB NOT NULL,
    created_at              TIMESTAMPTZ NOT NULL DEFAULT now(),
    resolved_at             TIMESTAMPTZ,
    resolved_by_user_id     TEXT,

    CONSTRAINT sketch_conflicts_conflict_type_not_blank CHECK (length(trim(conflict_type)) > 0),
    CONSTRAINT sketch_conflicts_caused_by_ops_array CHECK (jsonb_typeof(caused_by_ops) = 'array'),
    CONSTRAINT sketch_conflicts_affected_entity_ids_array CHECK (jsonb_typeof(affected_entity_ids) = 'array'),
    CONSTRAINT sketch_conflicts_payload_object CHECK (jsonb_typeof(payload) = 'object'),
    CONSTRAINT sketch_conflicts_resolved_consistency CHECK (
        (status = 'resolved' AND resolved_at IS NOT NULL AND resolved_by_user_id IS NOT NULL)
        OR
        (status <> 'resolved')
    )
);

CREATE INDEX IF NOT EXISTS idx_sketch_conflicts_open
    ON sketch_conflicts (sketch_id, created_at DESC)
    WHERE status = 'open';

CREATE INDEX IF NOT EXISTS idx_sketch_conflicts_sketch_status
    ON sketch_conflicts (sketch_id, status, created_at DESC);

CREATE TABLE IF NOT EXISTS sketch_files (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sketch_id           UUID REFERENCES sketches(id) ON DELETE CASCADE,
    workspace_id        UUID REFERENCES workspaces(id) ON DELETE CASCADE,
    kind                sketch_file_kind NOT NULL,

    bucket              TEXT NOT NULL,
    object_key          TEXT NOT NULL,
    object_version      TEXT,
    etag                TEXT,
    content_type        TEXT NOT NULL,
    size_bytes          BIGINT NOT NULL,
    sha256_hex          TEXT,

    created_by_user_id  TEXT NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    metadata            JSONB NOT NULL DEFAULT '{}'::jsonb,

    CONSTRAINT sketch_files_size_non_negative CHECK (size_bytes >= 0),
    CONSTRAINT sketch_files_bucket_not_blank CHECK (length(trim(bucket)) > 0),
    CONSTRAINT sketch_files_object_key_not_blank CHECK (length(trim(object_key)) > 0),
    CONSTRAINT sketch_files_content_type_not_blank CHECK (length(trim(content_type)) > 0),
    CONSTRAINT sketch_files_owner_scope CHECK (sketch_id IS NOT NULL OR workspace_id IS NOT NULL),
    CONSTRAINT sketch_files_metadata_object CHECK (jsonb_typeof(metadata) = 'object'),
    CONSTRAINT sketch_files_sha256_hex_format CHECK (
        sha256_hex IS NULL OR sha256_hex ~ '^[a-f0-9]{64}$'
    ),

    UNIQUE (bucket, object_key, object_version)
);

CREATE INDEX IF NOT EXISTS idx_sketch_files_sketch_id
    ON sketch_files (sketch_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_sketch_files_workspace_id
    ON sketch_files (workspace_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_sketch_files_kind
    ON sketch_files (kind, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_sketch_files_created_by_user_id
    ON sketch_files (created_by_user_id, created_at DESC);

CREATE TABLE IF NOT EXISTS simulation_jobs (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    sketch_id           UUID NOT NULL REFERENCES sketches(id) ON DELETE CASCADE,
    status              simulation_job_status NOT NULL DEFAULT 'queued',
    input_payload       JSONB NOT NULL,
    result_file_id      UUID REFERENCES sketch_files(id) ON DELETE SET NULL,
    created_by_user_id  TEXT NOT NULL,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),
    started_at          TIMESTAMPTZ,
    finished_at         TIMESTAMPTZ,
    error_message       TEXT,

    CONSTRAINT simulation_jobs_input_payload_object CHECK (jsonb_typeof(input_payload) = 'object'),
    CONSTRAINT simulation_jobs_time_order CHECK (
        started_at IS NULL OR finished_at IS NULL OR started_at <= finished_at
    )
);

CREATE INDEX IF NOT EXISTS idx_simulation_jobs_sketch_id
    ON simulation_jobs (sketch_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_simulation_jobs_status
    ON simulation_jobs (status, created_at ASC);

CREATE TABLE IF NOT EXISTS audit_events (
    id                  UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    actor_user_id       TEXT,
    workspace_id        UUID REFERENCES workspaces(id) ON DELETE SET NULL,
    sketch_id           UUID REFERENCES sketches(id) ON DELETE SET NULL,
    event_type          TEXT NOT NULL,
    payload             JSONB NOT NULL DEFAULT '{}'::jsonb,
    ip_address          INET,
    user_agent          TEXT,
    created_at          TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT audit_events_event_type_not_blank CHECK (length(trim(event_type)) > 0),
    CONSTRAINT audit_events_payload_object CHECK (jsonb_typeof(payload) = 'object')
);

CREATE INDEX IF NOT EXISTS idx_audit_events_actor_user_id
    ON audit_events (actor_user_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_audit_events_workspace_id
    ON audit_events (workspace_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_audit_events_sketch_id
    ON audit_events (sketch_id, created_at DESC);

CREATE INDEX IF NOT EXISTS idx_audit_events_event_type
    ON audit_events (event_type, created_at DESC);

CREATE OR REPLACE VIEW sketch_permission_view AS
SELECT
    sp.sketch_id,
    s.workspace_id,
    s.created_by_user_id AS sketch_creator_user_id,
    sp.user_id,
    sp.role,
    sp.granted_by_user_id,
    sp.granted_at,
    sp.updated_at
FROM sketch_permissions sp
JOIN sketches s ON s.id = sp.sketch_id;

-- +goose Down
DROP VIEW IF EXISTS sketch_permission_view;

DROP TABLE IF EXISTS audit_events;
DROP TABLE IF EXISTS simulation_jobs;
DROP TABLE IF EXISTS sketch_files;
DROP TABLE IF EXISTS sketch_conflicts;
DROP TABLE IF EXISTS sketch_snapshots;
DROP TABLE IF EXISTS sketch_current_states;
DROP TABLE IF EXISTS sketch_ops;
DROP TABLE IF EXISTS sketch_permission_audit;
DROP TABLE IF EXISTS sketch_permissions;
DROP TABLE IF EXISTS sketches;
DROP TABLE IF EXISTS workspaces;
DROP TABLE IF EXISTS user_profiles;

DROP FUNCTION IF EXISTS protect_sketch_creator_admin_permission();
DROP FUNCTION IF EXISTS ensure_sketch_creator_admin_permission();
DROP FUNCTION IF EXISTS set_updated_at();

DROP TYPE IF EXISTS simulation_job_status;
DROP TYPE IF EXISTS sketch_file_kind;
DROP TYPE IF EXISTS sketch_conflict_status;
DROP TYPE IF EXISTS sketch_role;
DROP TYPE IF EXISTS sketch_unit;
