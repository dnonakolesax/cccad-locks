WITH latest AS (
    SELECT max(document_version) AS document_version
    FROM topology_refs_3d
    WHERE part_id = $1
      AND ($2::text IS NULL OR body_id = $2::text)
)
SELECT
    body_id::text,
    ref_kind,
    ref_id,
    COALESCE(stable_ref, '') AS stable_ref,
    COALESCE(parent_ref_id, '') AS parent_ref_id,
    COALESCE(surface_or_curve_type, '') AS surface_or_curve_type,
    payload
FROM topology_refs_3d
WHERE part_id = $1
  AND document_version = (SELECT document_version FROM latest)
  AND ($2::text IS NULL OR body_id = $2::text)
ORDER BY
    body_id,
    CASE ref_kind
        WHEN 'body' THEN 0
        WHEN 'shell' THEN 1
        WHEN 'face' THEN 2
        WHEN 'loop' THEN 3
        WHEN 'edge' THEN 4
        WHEN 'vertex' THEN 5
        ELSE 6
    END,
    ref_id;
