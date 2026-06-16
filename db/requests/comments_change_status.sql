WITH existing AS (
    SELECT c.*
    FROM cad_comments c
    WHERE c.id = $1::uuid
        AND c.deleted_at IS NULL
        AND EXISTS (
            SELECT 1
            FROM sketch_permissions sp
            WHERE sp.sketch_id = c.document_id
                AND sp.user_id = $2
                AND sp.role IN ('editor', 'admin')
        )
),
updated AS (
    UPDATE cad_comments c
    SET status = $3::cad_comment_status
    FROM existing e
    WHERE c.id = e.id
    RETURNING c.*, e.status AS old_status
),
status_history AS (
    INSERT INTO comment_status_history (
        comment_id,
        old_status,
        new_status,
        changed_by,
        reason
    )
    SELECT
        id,
        old_status,
        status,
        $2,
        $4
    FROM updated
    WHERE old_status <> status
    RETURNING 1
)
SELECT
    u.id::text,
    u.workspace_id::text,
    u.document_id::text,
    u.part_id::text,
    u.target_type::text,
    u.target_id,
    u.kind::text,
    u.status::text,
    u.author_id,
    u.body,
    u.document_version::bigint,
    u.part_version::bigint,
    u.anchor,
    u.metadata,
    COALESCE(
        jsonb_agg(to_jsonb(ca.user_id) ORDER BY ca.assigned_at ASC) FILTER (WHERE ca.user_id IS NOT NULL),
        '[]'::jsonb
    ),
    u.created_at,
    u.updated_at,
    u.deleted_at,
    1::integer
FROM updated u
LEFT JOIN comment_assignees ca ON ca.comment_id = u.id
GROUP BY
    u.id,
    u.workspace_id,
    u.document_id,
    u.part_id,
    u.target_type,
    u.target_id,
    u.kind,
    u.status,
    u.author_id,
    u.body,
    u.document_version,
    u.part_version,
    u.anchor,
    u.metadata,
    u.created_at,
    u.updated_at,
    u.deleted_at
