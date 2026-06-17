SELECT
    c.id::text,
    c.workspace_id::text,
    c.sketch_id::text,
    c.part_id::text,
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
FROM cad_comments c
LEFT JOIN comment_assignees ca ON ca.comment_id = c.id
WHERE c.id = $1::uuid
    AND EXISTS (
        SELECT 1
        FROM workspaces w
        LEFT JOIN sketches s
            ON s.workspace_id = w.id
            AND s.deleted_at IS NULL
        LEFT JOIN sketch_permissions sp
            ON sp.sketch_id = s.id
            AND sp.user_id = $2
            AND sp.role IN ('reader', 'editor', 'admin')
        WHERE w.id = c.workspace_id
            AND w.deleted_at IS NULL
            AND (
                w.created_by_user_id = $2
                OR sp.user_id IS NOT NULL
            )
    )
GROUP BY
    c.id,
    c.workspace_id,
    c.sketch_id,
    c.part_id,
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
