WITH latest AS (
    SELECT max(document_version) AS document_version
    FROM part_representations_3d
    WHERE part_id = $1
      AND ($2::text IS NULL OR kind::text = $2::text)
)
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
  AND document_version = (SELECT document_version FROM latest)
  AND ($2::text IS NULL OR kind::text = $2::text)
ORDER BY
    body_id NULLS FIRST,
    kind,
    storage_key;
