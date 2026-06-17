SELECT s.workspace_id::text
FROM sketches s
WHERE s.id = $1::uuid
    AND s.deleted_at IS NULL
    AND EXISTS (
        SELECT 1
        FROM sketch_permissions sp
        WHERE sp.sketch_id = s.id
            AND sp.user_id = $2
    )
