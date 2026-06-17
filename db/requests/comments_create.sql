INSERT INTO cad_comments (
    workspace_id,
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
    $4::cad_comment_target_type,
    $5,
    $6::cad_comment_kind,
    $7,
    $8,
    $9::cad_comment_status,
    $10::bigint,
    $11::bigint,
    $12::jsonb,
    COALESCE($13::jsonb, '{}'::jsonb)
FROM workspaces w
LEFT JOIN sketches s
    ON s.id = NULLIF(COALESCE($2, ''), '')::uuid
    AND s.workspace_id = w.id
    AND s.deleted_at IS NULL
WHERE w.id = $1::uuid
    AND w.deleted_at IS NULL
    AND (
        (COALESCE($2, '') = '' AND w.created_by_user_id = $7)
        OR EXISTS (
            SELECT 1
            FROM sketch_permissions sp
            WHERE sp.sketch_id = s.id
                AND sp.user_id = $7
                AND sp.role IN ('editor', 'admin')
        )
    )
    AND (COALESCE($2, '') = '' OR s.id IS NOT NULL)
RETURNING id::text
