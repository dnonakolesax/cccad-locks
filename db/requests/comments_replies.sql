WITH parent AS (
    SELECT c.*
    FROM cad_comments c
    JOIN workspaces w ON w.id = c.workspace_id
    WHERE c.id = $1::uuid
        AND w.deleted_at IS NULL
        AND EXISTS (
            SELECT 1
            FROM workspaces w2
            LEFT JOIN sketches s
                ON s.workspace_id = w2.id
                AND s.deleted_at IS NULL
            LEFT JOIN sketch_permissions sp
                ON sp.sketch_id = s.id
                AND sp.user_id = $2
                AND sp.role IN ('reader', 'editor', 'admin')
            WHERE w2.id = c.workspace_id
                AND w2.deleted_at IS NULL
                AND (
                    w2.created_by_user_id = $2
                    OR sp.user_id IS NOT NULL
                )
        )
),
filtered AS (
    SELECT c.*, count(*) OVER ()::integer AS total_count
    FROM cad_comments c
    JOIN parent p ON p.id = c.parent_comment_id
    WHERE ($3::boolean OR c.message_type = 'user')
        AND ($4::boolean OR c.deleted_at IS NULL)
    ORDER BY c.created_at ASC
    LIMIT $5::integer
    OFFSET $6::integer
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
    c.total_count
FROM filtered c
LEFT JOIN comment_assignees ca ON ca.comment_id = c.id
GROUP BY
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
    c.deleted_at,
    c.total_count
ORDER BY c.created_at ASC
