WITH filtered AS (
    SELECT c.*
    FROM cad_comments c
    WHERE c.workspace_id = $1::uuid
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
        AND ($3 = '' OR c.sketch_id = NULLIF($3, '')::uuid)
        AND ($4 = '' OR c.part_id = NULLIF($4, '')::uuid)
        AND ($5 = '' OR c.target_type = $5::cad_comment_target_type)
        AND ($6 = '' OR c.target_id = $6)
        AND ($7 = '' OR c.kind = $7::cad_comment_kind)
        AND ($8 = '' OR c.status = $8::cad_comment_status)
        AND (
            $9 = ''
            OR EXISTS (
                SELECT 1
                FROM comment_assignees ca
                WHERE ca.comment_id = c.id
                    AND ca.user_id = $9
            )
        )
        AND ($10::boolean OR c.deleted_at IS NULL)
),
counted AS (
    SELECT
        filtered.*,
        count(*) OVER ()::integer AS total_count
    FROM filtered
    ORDER BY filtered.created_at DESC
    LIMIT $11::integer
    OFFSET $12::integer
)
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
    c.total_count
FROM counted c
LEFT JOIN comment_assignees ca ON ca.comment_id = c.id
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
    c.deleted_at,
    c.total_count
ORDER BY c.created_at DESC
