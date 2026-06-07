WITH latest AS (
    SELECT max(document_version) AS document_version
    FROM topology_refs_3d
    WHERE part_id = $1
      AND body_id = $2
)
SELECT
    COALESCE(surface_or_curve_type, 'unknown') AS surface_type,
    payload
FROM topology_refs_3d
WHERE part_id = $1
  AND body_id = $2
  AND document_version = (SELECT document_version FROM latest)
  AND ref_kind = 'face'
  AND ref_id = $3
LIMIT 1;
