SELECT
    h.id::text,
    h.comment_id::text,
    h.old_body,
    h.new_body,
    h.edited_by_user_id,
    h.edited_at
FROM comment_edit_history h
JOIN cad_comments c ON c.id = h.comment_id
WHERE h.comment_id = $1::uuid
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
ORDER BY h.edited_at ASC
