WITH filtered AS (
    SELECT c.*
    FROM cad_comments c
    WHERE c.document_id = $1::uuid
        AND EXISTS (
            SELECT 1
            FROM sketch_permissions sp
            WHERE sp.sketch_id = c.document_id
                AND sp.user_id = $2
        )
        AND ($3 = '' OR c.part_id = NULLIF($3, '')::uuid)
        AND ($4 = '' OR c.target_type = $4::cad_comment_target_type)
        AND ($5 = '' OR c.target_id = $5)
        AND ($6 = '' OR c.kind = $6::cad_comment_kind)
        AND ($7 = '' OR c.status = $7::cad_comment_status)
        AND (
            $8 = ''
            OR EXISTS (
                SELECT 1
                FROM comment_assignees ca
                WHERE ca.comment_id = c.id
                    AND ca.user_id = $8
            )
        )
        AND ($9::boolean OR c.deleted_at IS NULL)
),
counted AS (
    SELECT
        filtered.*,
        count(*) OVER ()::integer AS total_count
    FROM filtered
    ORDER BY filtered.created_at DESC
    LIMIT $10::integer
    OFFSET $11::integer
)
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
    c.total_count
FROM counted c
LEFT JOIN comment_assignees ca ON ca.comment_id = c.id
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
    c.deleted_at,
    c.total_count
ORDER BY c.created_at DESC
