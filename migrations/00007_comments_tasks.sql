-- +goose Up
-- Comments/tasks module for the current cccAD DB schema.
--
-- Current schema notes:
-- - there is no `documents` table;
-- - user identifiers are TEXT values from Keycloak/user_profiles, not UUID FKs;
-- - comments are scoped by workspace_id and may additionally point to sketch_id or part_id;
-- - target_type + target_id is the polymorphic CAD entity reference.

-- +goose StatementBegin
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'cad_comment_target_type') THEN
        CREATE TYPE cad_comment_target_type AS ENUM (
            'workspace',
            'sketch',
            'sketch_entity',
            'constraint',
            'part',
            'feature_3d',
            'body',
            'face',
            'edge',
            'vertex',
            'profile',
            'topology_ref_3d',
            'representation_3d',
            'simulation_job',
            'simulation_result',
            'mesh_entity'
        );
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'cad_comment_kind') THEN
        CREATE TYPE cad_comment_kind AS ENUM (
            'comment',
            'task'
        );
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'cad_comment_status') THEN
        CREATE TYPE cad_comment_status AS ENUM (
            'open',
            'in_progress',
            'resolved',
            'reopened',
            'closed',
            'rejected'
        );
    END IF;
END $$;
-- +goose StatementEnd

CREATE TABLE IF NOT EXISTS cad_comments (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    workspace_id UUID NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,

    -- Optional coarse domain scope.
    -- A sketch comment should usually have sketch_id.
    -- A part/feature/body/topology comment should usually have part_id.
    sketch_id UUID REFERENCES sketches(id) ON DELETE CASCADE,
    part_id UUID REFERENCES parts(id) ON DELETE CASCADE,

    -- Universal target reference.
    -- target_id is TEXT because some CAD/topology IDs are not UUID:
    -- e.g. "line_42", "constraint_7", "profile_1", generated body ids after 00006.
    target_type cad_comment_target_type NOT NULL,
    target_id TEXT NOT NULL,

    kind cad_comment_kind NOT NULL DEFAULT 'comment',

    -- Keycloak/user_profiles identifier.
    -- Existing schema uses TEXT user ids instead of UUID user FKs.
    author_user_id TEXT NOT NULL,

    body TEXT NOT NULL,
    status cad_comment_status NOT NULL DEFAULT 'open',

    -- Version of the entity/context at comment creation time.
    sketch_version BIGINT,
    part_version BIGINT,

    -- Optional geometric/viewport pin:
    -- sketch point, face UV point, viewport rect, etc.
    anchor JSONB,

    -- Free extension point: severity, labels, external tracker id, etc.
    metadata JSONB NOT NULL DEFAULT '{}'::jsonb,

    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    deleted_at TIMESTAMPTZ,

    CONSTRAINT cad_comments_body_nonempty CHECK (length(trim(body)) > 0),
    CONSTRAINT cad_comments_target_id_nonempty CHECK (length(trim(target_id)) > 0),
    CONSTRAINT cad_comments_author_user_id_nonempty CHECK (length(trim(author_user_id)) > 0),
    CONSTRAINT cad_comments_sketch_version_nonnegative CHECK (
        sketch_version IS NULL OR sketch_version >= 0
    ),
    CONSTRAINT cad_comments_part_version_nonnegative CHECK (
        part_version IS NULL OR part_version >= 0
    ),
    CONSTRAINT cad_comments_anchor_object CHECK (
        anchor IS NULL OR jsonb_typeof(anchor) = 'object'
    ),
    CONSTRAINT cad_comments_metadata_object CHECK (
        jsonb_typeof(metadata) = 'object'
    )
);

CREATE INDEX IF NOT EXISTS idx_cad_comments_workspace_active
    ON cad_comments(workspace_id, created_at DESC)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_cad_comments_sketch_active
    ON cad_comments(sketch_id, created_at DESC)
    WHERE deleted_at IS NULL AND sketch_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_cad_comments_part_active
    ON cad_comments(part_id, created_at DESC)
    WHERE deleted_at IS NULL AND part_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_cad_comments_target_active
    ON cad_comments(target_type, target_id, created_at DESC)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_cad_comments_status_active
    ON cad_comments(status, created_at DESC)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_cad_comments_task_status_active
    ON cad_comments(workspace_id, status, created_at DESC)
    WHERE deleted_at IS NULL AND kind = 'task';

CREATE INDEX IF NOT EXISTS idx_cad_comments_author_active
    ON cad_comments(author_user_id, created_at DESC)
    WHERE deleted_at IS NULL;

CREATE TABLE IF NOT EXISTS comment_assignees (
    comment_id UUID NOT NULL REFERENCES cad_comments(id) ON DELETE CASCADE,

    -- Keycloak/user_profiles identifier.
    user_id TEXT NOT NULL,

    assigned_by_user_id TEXT NOT NULL,
    assigned_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    PRIMARY KEY (comment_id, user_id),

    CONSTRAINT comment_assignees_user_id_nonempty CHECK (length(trim(user_id)) > 0),
    CONSTRAINT comment_assignees_assigned_by_nonempty CHECK (length(trim(assigned_by_user_id)) > 0)
);

CREATE INDEX IF NOT EXISTS idx_comment_assignees_user
    ON comment_assignees(user_id);

CREATE INDEX IF NOT EXISTS idx_comment_assignees_comment
    ON comment_assignees(comment_id);

CREATE TABLE IF NOT EXISTS comment_status_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    comment_id UUID NOT NULL REFERENCES cad_comments(id) ON DELETE CASCADE,

    old_status cad_comment_status,
    new_status cad_comment_status NOT NULL,

    changed_by_user_id TEXT NOT NULL,
    changed_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    reason TEXT,

    CONSTRAINT comment_status_history_changed_by_nonempty CHECK (length(trim(changed_by_user_id)) > 0),
    CONSTRAINT comment_status_history_reason_nonempty CHECK (
        reason IS NULL OR length(trim(reason)) > 0
    )
);

CREATE INDEX IF NOT EXISTS idx_comment_status_history_comment
    ON comment_status_history(comment_id, changed_at ASC);

CREATE INDEX IF NOT EXISTS idx_comment_status_history_changed_by
    ON comment_status_history(changed_by_user_id, changed_at DESC);

CREATE TABLE IF NOT EXISTS comment_edit_history (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),

    comment_id UUID NOT NULL REFERENCES cad_comments(id) ON DELETE CASCADE,

    old_body TEXT NOT NULL,
    new_body TEXT NOT NULL,

    edited_by_user_id TEXT NOT NULL,
    edited_at TIMESTAMPTZ NOT NULL DEFAULT now(),

    CONSTRAINT comment_edit_history_old_body_nonempty CHECK (length(trim(old_body)) > 0),
    CONSTRAINT comment_edit_history_new_body_nonempty CHECK (length(trim(new_body)) > 0),
    CONSTRAINT comment_edit_history_edited_by_nonempty CHECK (length(trim(edited_by_user_id)) > 0),
    CONSTRAINT comment_edit_history_body_changed CHECK (old_body <> new_body)
);

CREATE INDEX IF NOT EXISTS idx_comment_edit_history_comment
    ON comment_edit_history(comment_id, edited_at ASC);

CREATE INDEX IF NOT EXISTS idx_comment_edit_history_edited_by
    ON comment_edit_history(edited_by_user_id, edited_at DESC);

-- goose must not split PL/pgSQL function bodies by internal semicolons.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION cad_comments_set_updated_at()
RETURNS TRIGGER AS $$
BEGIN
    NEW.updated_at = now();
    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

DROP TRIGGER IF EXISTS trg_cad_comments_updated_at ON cad_comments;

CREATE TRIGGER trg_cad_comments_updated_at
BEFORE UPDATE ON cad_comments
FOR EACH ROW
EXECUTE FUNCTION cad_comments_set_updated_at();

-- Initial status history can be created by trigger because author_user_id is present on NEW.
-- Later status transitions should be written by the Go service transactionally.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION cad_comments_insert_initial_status_history()
RETURNS TRIGGER AS $$
BEGIN
    INSERT INTO comment_status_history (
        comment_id,
        old_status,
        new_status,
        changed_by_user_id,
        reason
    )
    VALUES (
        NEW.id,
        NULL,
        NEW.status,
        NEW.author_user_id,
        'created'
    );

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

DROP TRIGGER IF EXISTS trg_cad_comments_initial_status_history ON cad_comments;

CREATE TRIGGER trg_cad_comments_initial_status_history
AFTER INSERT ON cad_comments
FOR EACH ROW
EXECUTE FUNCTION cad_comments_insert_initial_status_history();

-- +goose Down
DROP TRIGGER IF EXISTS trg_cad_comments_initial_status_history ON cad_comments;
DROP FUNCTION IF EXISTS cad_comments_insert_initial_status_history();

DROP TRIGGER IF EXISTS trg_cad_comments_updated_at ON cad_comments;
DROP FUNCTION IF EXISTS cad_comments_set_updated_at();

DROP TABLE IF EXISTS comment_edit_history;
DROP TABLE IF EXISTS comment_status_history;
DROP TABLE IF EXISTS comment_assignees;
DROP TABLE IF EXISTS cad_comments;

DROP TYPE IF EXISTS cad_comment_status;
DROP TYPE IF EXISTS cad_comment_kind;
DROP TYPE IF EXISTS cad_comment_target_type;
