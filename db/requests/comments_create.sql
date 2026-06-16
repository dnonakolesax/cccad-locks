INSERT INTO cad_comments (
    workspace_id,
    document_id,
    part_id,
    target_type,
    target_id,
    kind,
    author_id,
    body,
    status,
    document_version,
    part_version,
    anchor,
    metadata
)
SELECT
    s.workspace_id,
    s.id,
    NULLIF($2, '')::uuid,
    $3::cad_comment_target_type,
    $4,
    $5::cad_comment_kind,
    $6,
    $7,
    $8::cad_comment_status,
    $9::integer,
    $10::integer,
    $11::jsonb,
    COALESCE($12::jsonb, '{}'::jsonb)
FROM sketches s
WHERE s.id = $1::uuid
    AND s.deleted_at IS NULL
    AND EXISTS (
        SELECT 1
        FROM sketch_permissions sp
        WHERE sp.sketch_id = s.id
            AND sp.user_id = $6
            AND sp.role IN ('editor', 'admin')
    )
RETURNING id::text
