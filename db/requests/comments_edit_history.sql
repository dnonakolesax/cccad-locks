SELECT
    h.id::text,
    h.comment_id::text,
    h.old_body,
    h.new_body,
    h.edited_by,
    h.edited_at
FROM comment_edit_history h
JOIN cad_comments c ON c.id = h.comment_id
WHERE h.comment_id = $1::uuid
    AND EXISTS (
        SELECT 1
        FROM sketch_permissions sp
        WHERE sp.sketch_id = c.document_id
            AND sp.user_id = $2
    )
ORDER BY h.edited_at ASC
