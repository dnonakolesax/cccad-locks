SELECT
    h.id::text,
    h.comment_id::text,
    h.old_status::text,
    h.new_status::text,
    h.changed_by,
    h.changed_at,
    h.reason
FROM comment_status_history h
JOIN cad_comments c ON c.id = h.comment_id
WHERE h.comment_id = $1::uuid
    AND EXISTS (
        SELECT 1
        FROM sketch_permissions sp
        WHERE sp.sketch_id = c.document_id
            AND sp.user_id = $2
    )
ORDER BY h.changed_at ASC
