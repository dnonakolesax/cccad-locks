WITH allowed AS (
    SELECT c.id
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
deleted AS (
    DELETE FROM comment_assignees ca
    USING allowed a
    WHERE ca.comment_id = a.id
    RETURNING 1
)
,
inserted AS (
    INSERT INTO comment_assignees (
        comment_id,
        user_id,
        assigned_by
    )
    SELECT
        a.id,
        assignee.user_id,
        $2
    FROM allowed a
    CROSS JOIN unnest($3::text[]) AS assignee(user_id)
    ON CONFLICT (comment_id, user_id) DO NOTHING
    RETURNING 1
)
SELECT id::text FROM allowed
