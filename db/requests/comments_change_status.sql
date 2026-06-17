WITH existing AS (
    SELECT c.*
    FROM cad_comments c
    JOIN workspaces w ON w.id = c.workspace_id
    WHERE c.id = $1::uuid
        AND c.deleted_at IS NULL
        AND c.message_type = 'user'
        AND w.deleted_at IS NULL
        AND (
            w.created_by_user_id = $2
            OR EXISTS (
                SELECT 1
                FROM sketch_permissions sp
                WHERE sp.sketch_id = c.sketch_id
                    AND sp.user_id = $2
                    AND sp.role IN ('editor', 'admin')
            )
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
        changed_by_user_id,
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
),
system_message AS (
    INSERT INTO cad_comments (
        workspace_id,
        parent_comment_id,
        message_type,
        system_event_type,
        author_user_id,
        body,
        event_payload,
        kind,
        status
    )
    SELECT
        workspace_id,
        id,
        'system',
        'status_changed',
        $2,
        'Пользователь ' || $2 || ' поменял статус с «' ||
            CASE old_status::text
                WHEN 'open' THEN 'открыто'
                WHEN 'in_progress' THEN 'в работе'
                WHEN 'resolved' THEN 'решено'
                WHEN 'reopened' THEN 'переоткрыто'
                WHEN 'closed' THEN 'закрыто'
                WHEN 'rejected' THEN 'отклонено'
                ELSE old_status::text
            END || '» на «' ||
            CASE status::text
                WHEN 'open' THEN 'открыто'
                WHEN 'in_progress' THEN 'в работе'
                WHEN 'resolved' THEN 'решено'
                WHEN 'reopened' THEN 'переоткрыто'
                WHEN 'closed' THEN 'закрыто'
                WHEN 'rejected' THEN 'отклонено'
                ELSE status::text
            END || '».',
        jsonb_build_object(
            'commentId', id::text,
            'actorUserId', $2,
            'oldStatus', old_status::text,
            'newStatus', status::text,
            'reason', $4
        ),
        'comment',
        'open'
    FROM updated
    RETURNING *
),
combined AS (
    SELECT
        1 AS ord,
        id,
        workspace_id,
        sketch_id,
        part_id,
        target_type,
        target_id,
        kind,
        author_user_id,
        body,
        status,
        sketch_version,
        part_version,
        anchor,
        metadata,
        created_at,
        updated_at,
        deleted_at,
        message_type,
        system_event_type,
        event_payload,
        parent_comment_id,
        thread_root_id,
        reply_depth
    FROM updated
    UNION ALL
    SELECT
        2 AS ord,
        id,
        workspace_id,
        sketch_id,
        part_id,
        target_type,
        target_id,
        kind,
        author_user_id,
        body,
        status,
        sketch_version,
        part_version,
        anchor,
        metadata,
        created_at,
        updated_at,
        deleted_at,
        message_type,
        system_event_type,
        event_payload,
        parent_comment_id,
        thread_root_id,
        reply_depth
    FROM system_message
)
SELECT
    c.id::text,
    c.workspace_id::text,
    c.sketch_id::text,
    c.part_id::text,
    c.parent_comment_id::text,
    c.thread_root_id::text,
    c.reply_depth,
    (
        SELECT count(*)::integer
        FROM cad_comments child
        WHERE child.parent_comment_id = c.id
            AND child.deleted_at IS NULL
    ),
    c.message_type::text,
    c.system_event_type::text,
    c.event_payload,
    c.target_type::text,
    c.target_id,
    c.kind::text,
    c.status::text,
    c.author_user_id,
    c.body,
    c.sketch_version::bigint,
    c.part_version::bigint,
    c.anchor,
    c.metadata,
    COALESCE(
        jsonb_agg(to_jsonb(ca.user_id) ORDER BY ca.assigned_at ASC) FILTER (WHERE ca.user_id IS NOT NULL),
        '[]'::jsonb
    ),
    c.created_at,
    c.updated_at,
    c.deleted_at,
    1::integer
FROM combined c
LEFT JOIN comment_assignees ca ON ca.comment_id = c.id
GROUP BY
    c.ord,
    c.id,
    c.workspace_id,
    c.sketch_id,
    c.part_id,
    c.parent_comment_id,
    c.thread_root_id,
    c.reply_depth,
    c.message_type,
    c.system_event_type,
    c.event_payload,
    c.target_type,
    c.target_id,
    c.kind,
    c.status,
    c.author_user_id,
    c.body,
    c.sketch_version,
    c.part_version,
    c.anchor,
    c.metadata,
    c.created_at,
    c.updated_at,
    c.deleted_at
ORDER BY c.ord ASC
