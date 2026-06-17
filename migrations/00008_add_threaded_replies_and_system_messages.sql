
-- +goose Up
-- Add nested replies and system/service messages to already applied comments migration.
--
-- This migration assumes 00007_comments_tasks.sql is already applied.
-- It keeps user comments, replies and service messages in one ordered client-visible stream.
--
-- User-visible examples:
-- - ordinary user comment: message_type = 'user'
-- - reply to comment: parent_comment_id = <comment_id>, message_type = 'user'
-- - service message: message_type = 'system', system_event_type = 'status_changed'
--
-- Service messages should normally be inserted by the Go service in the same transaction
-- as the action they describe, for example changing status from open to in_progress.

-- +goose StatementBegin
DO $$
BEGIN
    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'cad_comment_message_type') THEN
        CREATE TYPE cad_comment_message_type AS ENUM (
            'user',
            'system'
        );
    END IF;

    IF NOT EXISTS (SELECT 1 FROM pg_type WHERE typname = 'cad_comment_system_event_type') THEN
        CREATE TYPE cad_comment_system_event_type AS ENUM (
            'comment_created',
            'reply_created',
            'body_edited',
            'status_changed',
            'assignees_changed',
            'comment_deleted',
            'comment_restored'
        );
    END IF;
END $$;
-- +goose StatementEnd

ALTER TABLE cad_comments
    ADD COLUMN IF NOT EXISTS message_type cad_comment_message_type NOT NULL DEFAULT 'user';

ALTER TABLE cad_comments
    ADD COLUMN IF NOT EXISTS system_event_type cad_comment_system_event_type;

ALTER TABLE cad_comments
    ADD COLUMN IF NOT EXISTS event_payload JSONB NOT NULL DEFAULT '{}'::jsonb;

ALTER TABLE cad_comments
    ADD COLUMN IF NOT EXISTS parent_comment_id UUID REFERENCES cad_comments(id) ON DELETE CASCADE;

ALTER TABLE cad_comments
    ADD COLUMN IF NOT EXISTS thread_root_id UUID REFERENCES cad_comments(id) ON DELETE CASCADE;

ALTER TABLE cad_comments
    ADD COLUMN IF NOT EXISTS reply_depth INTEGER NOT NULL DEFAULT 0;

UPDATE cad_comments
SET thread_root_id = id,
    reply_depth = 0
WHERE thread_root_id IS NULL;

ALTER TABLE cad_comments
    ALTER COLUMN thread_root_id SET NOT NULL;

-- +goose StatementBegin
DO $$
BEGIN
    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'cad_comments_parent_not_self'
          AND conrelid = 'cad_comments'::regclass
    ) THEN
        ALTER TABLE cad_comments
            ADD CONSTRAINT cad_comments_parent_not_self
            CHECK (parent_comment_id IS NULL OR parent_comment_id <> id);
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'cad_comments_reply_depth_valid'
          AND conrelid = 'cad_comments'::regclass
    ) THEN
        ALTER TABLE cad_comments
            ADD CONSTRAINT cad_comments_reply_depth_valid
            CHECK (reply_depth >= 0 AND reply_depth <= 50);
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'cad_comments_event_payload_object'
          AND conrelid = 'cad_comments'::regclass
    ) THEN
        ALTER TABLE cad_comments
            ADD CONSTRAINT cad_comments_event_payload_object
            CHECK (jsonb_typeof(event_payload) = 'object');
    END IF;

    IF NOT EXISTS (
        SELECT 1
        FROM pg_constraint
        WHERE conname = 'cad_comments_system_event_consistency'
          AND conrelid = 'cad_comments'::regclass
    ) THEN
        ALTER TABLE cad_comments
            ADD CONSTRAINT cad_comments_system_event_consistency
            CHECK (
                (message_type = 'user' AND system_event_type IS NULL)
                OR
                (message_type = 'system' AND system_event_type IS NOT NULL)
            );
    END IF;
END $$;
-- +goose StatementEnd

CREATE INDEX IF NOT EXISTS idx_cad_comments_parent_active
    ON cad_comments(parent_comment_id, created_at ASC)
    WHERE deleted_at IS NULL AND parent_comment_id IS NOT NULL;

CREATE INDEX IF NOT EXISTS idx_cad_comments_thread_active
    ON cad_comments(thread_root_id, reply_depth ASC, created_at ASC)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_cad_comments_message_type_active
    ON cad_comments(workspace_id, message_type, created_at DESC)
    WHERE deleted_at IS NULL;

CREATE INDEX IF NOT EXISTS idx_cad_comments_system_event_active
    ON cad_comments(system_event_type, created_at DESC)
    WHERE deleted_at IS NULL AND message_type = 'system';

-- Prepare thread fields for both user replies and system messages.
-- If parent_comment_id is set, the row inherits workspace/sketch/part/target from parent.
-- This makes service messages naturally appear inside the same thread.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION cad_comments_prepare_thread_fields()
RETURNS TRIGGER AS $$
DECLARE
    parent_row cad_comments%ROWTYPE;
BEGIN
    IF NEW.parent_comment_id IS NULL THEN
        NEW.thread_root_id = COALESCE(NEW.thread_root_id, NEW.id);
        NEW.reply_depth = 0;
        RETURN NEW;
    END IF;

    IF NEW.parent_comment_id = NEW.id THEN
        RAISE EXCEPTION 'comment cannot be a reply to itself';
    END IF;

    SELECT *
    INTO parent_row
    FROM cad_comments
    WHERE id = NEW.parent_comment_id
      AND deleted_at IS NULL;

    IF NOT FOUND THEN
        RAISE EXCEPTION 'parent comment % does not exist or is deleted', NEW.parent_comment_id;
    END IF;

    IF NEW.workspace_id IS DISTINCT FROM parent_row.workspace_id THEN
        RAISE EXCEPTION 'reply/system message workspace_id must match parent comment workspace_id';
    END IF;

    IF NEW.sketch_id IS NOT NULL AND NEW.sketch_id IS DISTINCT FROM parent_row.sketch_id THEN
        RAISE EXCEPTION 'reply/system message sketch_id must match parent comment sketch_id';
    END IF;
    NEW.sketch_id = parent_row.sketch_id;

    IF NEW.part_id IS NOT NULL AND NEW.part_id IS DISTINCT FROM parent_row.part_id THEN
        RAISE EXCEPTION 'reply/system message part_id must match parent comment part_id';
    END IF;
    NEW.part_id = parent_row.part_id;

    IF NEW.target_type IS NOT NULL AND NEW.target_type IS DISTINCT FROM parent_row.target_type THEN
        RAISE EXCEPTION 'reply/system message target_type must match parent comment target_type';
    END IF;
    NEW.target_type = parent_row.target_type;

    IF NEW.target_id IS NOT NULL
       AND length(trim(NEW.target_id)) > 0
       AND NEW.target_id IS DISTINCT FROM parent_row.target_id THEN
        RAISE EXCEPTION 'reply/system message target_id must match parent comment target_id';
    END IF;
    NEW.target_id = parent_row.target_id;

    NEW.thread_root_id = COALESCE(parent_row.thread_root_id, parent_row.id);
    NEW.reply_depth = parent_row.reply_depth + 1;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

DROP TRIGGER IF EXISTS trg_cad_comments_prepare_thread_fields ON cad_comments;

CREATE TRIGGER trg_cad_comments_prepare_thread_fields
BEFORE INSERT ON cad_comments
FOR EACH ROW
EXECUTE FUNCTION cad_comments_prepare_thread_fields();

-- Thread structure and message type are immutable after insert.
-- Body/metadata/status may still be edited through application logic.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION cad_comments_prevent_thread_mutation()
RETURNS TRIGGER AS $$
BEGIN
    IF OLD.parent_comment_id IS DISTINCT FROM NEW.parent_comment_id
       OR OLD.thread_root_id IS DISTINCT FROM NEW.thread_root_id
       OR OLD.reply_depth IS DISTINCT FROM NEW.reply_depth
       OR OLD.message_type IS DISTINCT FROM NEW.message_type
       OR OLD.system_event_type IS DISTINCT FROM NEW.system_event_type THEN
        RAISE EXCEPTION 'comment thread/message-type fields are immutable';
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

DROP TRIGGER IF EXISTS trg_cad_comments_prevent_thread_mutation ON cad_comments;

CREATE TRIGGER trg_cad_comments_prevent_thread_mutation
BEFORE UPDATE ON cad_comments
FOR EACH ROW
EXECUTE FUNCTION cad_comments_prevent_thread_mutation();

-- Replace the previous initial-status trigger function.
-- System messages are timeline entries, not workflow tasks/comments, so they should not
-- pollute comment_status_history.
-- +goose StatementBegin
CREATE OR REPLACE FUNCTION cad_comments_insert_initial_status_history()
RETURNS TRIGGER AS $$
BEGIN
    IF NEW.message_type = 'user' THEN
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
    END IF;

    RETURN NEW;
END;
$$ LANGUAGE plpgsql;
-- +goose StatementEnd

-- +goose Down
DROP TRIGGER IF EXISTS trg_cad_comments_prevent_thread_mutation ON cad_comments;
DROP FUNCTION IF EXISTS cad_comments_prevent_thread_mutation();

DROP TRIGGER IF EXISTS trg_cad_comments_prepare_thread_fields ON cad_comments;
DROP FUNCTION IF EXISTS cad_comments_prepare_thread_fields();

DROP INDEX IF EXISTS idx_cad_comments_system_event_active;
DROP INDEX IF EXISTS idx_cad_comments_message_type_active;
DROP INDEX IF EXISTS idx_cad_comments_thread_active;
DROP INDEX IF EXISTS idx_cad_comments_parent_active;

-- Restore simpler initial status trigger function. It will work for old user-only rows.
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

-- +goose StatementBegin
DO $$
BEGIN
    IF EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'cad_comments_system_event_consistency'
          AND conrelid = 'cad_comments'::regclass
    ) THEN
        ALTER TABLE cad_comments DROP CONSTRAINT cad_comments_system_event_consistency;
    END IF;

    IF EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'cad_comments_event_payload_object'
          AND conrelid = 'cad_comments'::regclass
    ) THEN
        ALTER TABLE cad_comments DROP CONSTRAINT cad_comments_event_payload_object;
    END IF;

    IF EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'cad_comments_reply_depth_valid'
          AND conrelid = 'cad_comments'::regclass
    ) THEN
        ALTER TABLE cad_comments DROP CONSTRAINT cad_comments_reply_depth_valid;
    END IF;

    IF EXISTS (
        SELECT 1 FROM pg_constraint
        WHERE conname = 'cad_comments_parent_not_self'
          AND conrelid = 'cad_comments'::regclass
    ) THEN
        ALTER TABLE cad_comments DROP CONSTRAINT cad_comments_parent_not_self;
    END IF;
END $$;
-- +goose StatementEnd

ALTER TABLE cad_comments DROP COLUMN IF EXISTS reply_depth;
ALTER TABLE cad_comments DROP COLUMN IF EXISTS thread_root_id;
ALTER TABLE cad_comments DROP COLUMN IF EXISTS parent_comment_id;
ALTER TABLE cad_comments DROP COLUMN IF EXISTS event_payload;
ALTER TABLE cad_comments DROP COLUMN IF EXISTS system_event_type;
ALTER TABLE cad_comments DROP COLUMN IF EXISTS message_type;

DROP TYPE IF EXISTS cad_comment_system_event_type;
DROP TYPE IF EXISTS cad_comment_message_type;
