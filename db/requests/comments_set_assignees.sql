WITH target AS (
    SELECT id
    FROM cad_comments
    WHERE id = $1::uuid
),
inserted AS (
    INSERT INTO comment_assignees (
        comment_id,
        user_id,
        assigned_by_user_id
    )
    SELECT
        t.id,
        assignee.user_id,
        $2
    FROM target t
    CROSS JOIN unnest($3::text[]) AS assignee(user_id)
    ON CONFLICT (comment_id, user_id) DO NOTHING
    RETURNING 1
)
SELECT id::text FROM target
