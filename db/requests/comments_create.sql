INSERT INTO cad_comments (
    workspace_id,
    parent_comment_id,
    sketch_id,
    part_id,
    target_type,
    target_id,
    kind,
    author_user_id,
    body,
    status,
    sketch_version,
    part_version,
    anchor,
    metadata
)
SELECT
    w.id,
    NULLIF(COALESCE($2, ''), '')::uuid,
    NULLIF(COALESCE($3, ''), '')::uuid,
    NULLIF(COALESCE($4, ''), '')::uuid,
    COALESCE(parent.target_type, NULLIF($5, '')::cad_comment_target_type),
    COALESCE(parent.target_id, NULLIF($6, '')),
    $7::cad_comment_kind,
    $8,
    $9,
    $10::cad_comment_status,
    $11::bigint,
    $12::bigint,
    $13::jsonb,
    COALESCE($14::jsonb, '{}'::jsonb)
FROM workspaces w
LEFT JOIN sketches s
    ON s.id = NULLIF(COALESCE($3, ''), '')::uuid
    AND s.workspace_id = w.id
    AND s.deleted_at IS NULL
LEFT JOIN cad_comments parent
    ON parent.id = NULLIF(COALESCE($2, ''), '')::uuid
    AND parent.workspace_id = w.id
    AND parent.deleted_at IS NULL
WHERE w.id = $1::uuid
    AND w.deleted_at IS NULL
    AND (
        (COALESCE($3, '') = '' AND w.created_by_user_id = $8)
        OR EXISTS (
            SELECT 1
            FROM sketch_permissions sp
            WHERE sp.sketch_id = s.id
                AND sp.user_id = $8
                AND sp.role IN ('editor', 'admin')
        )
        OR (
            parent.id IS NOT NULL
            AND (
                w.created_by_user_id = $8
                OR EXISTS (
                    SELECT 1
                    FROM sketch_permissions sp
                    WHERE sp.sketch_id = parent.sketch_id
                        AND sp.user_id = $8
                        AND sp.role IN ('editor', 'admin')
                )
            )
        )
    )
    AND (COALESCE($3, '') = '' OR s.id IS NOT NULL)
    AND (COALESCE($2, '') = '' OR parent.id IS NOT NULL)
RETURNING id::text
