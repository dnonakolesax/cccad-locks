UPDATE cad_comments c
SET deleted_at = COALESCE(c.deleted_at, now())
WHERE c.id = $1::uuid
    AND EXISTS (
        SELECT 1
        FROM sketch_permissions sp
        WHERE sp.sketch_id = c.document_id
            AND sp.user_id = $2
            AND sp.role IN ('editor', 'admin')
    )
RETURNING id::text
