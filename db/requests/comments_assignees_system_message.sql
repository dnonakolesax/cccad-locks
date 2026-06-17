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
    c.workspace_id,
    c.id,
    'system',
    'assignees_changed',
    $2,
    'Пользователь ' || $2 || ' изменил исполнителей.',
    jsonb_build_object(
        'commentId', c.id::text,
        'actorUserId', $2,
        'assigneeUserIds', COALESCE(to_jsonb($3::text[]), '[]'::jsonb)
    ),
    'comment',
    'open'
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
RETURNING id::text
