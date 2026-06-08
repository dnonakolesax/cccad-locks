SELECT
    id::text,
    part_id::text,
    COALESCE(body_id::text, '') AS body_id,
    document_version,
    kind::text,
    storage_key,
    COALESCE(content_type, '') AS content_type,
    COALESCE(size_bytes, 0) AS size_bytes,
    COALESCE(sha256, '') AS sha256,
    created_at
FROM part_representations_3d
WHERE part_id = $1
  AND id = $2
LIMIT 1;
