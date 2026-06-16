SELECT
    c.id::text,
    c.workspace_id::text,
    c.document_id::text,
    c.part_id::text,
    c.target_type::text,
    c.target_id,
    c.kind::text,
    c.status::text,
    c.author_id,
    c.body,
    c.document_version::bigint,
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
        FROM sketch_permissions sp
        WHERE sp.sketch_id = c.document_id
            AND sp.user_id = $2
    )
GROUP BY
    c.id,
    c.workspace_id,
    c.document_id,
    c.part_id,
    c.target_type,
    c.target_id,
    c.kind,
    c.status,
    c.author_id,
    c.body,
    c.document_version,
    c.part_version,
    c.anchor,
    c.metadata,
    c.created_at,
    c.updated_at,
    c.deleted_at
