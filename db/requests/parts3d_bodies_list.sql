SELECT
    id::text,
    part_id::text,
    name,
    active,
    COALESCE(created_by_feature_id::text, '') AS created_by_feature_id,
    COALESCE(stable_ref, '') AS stable_ref,
    created_at,
    updated_at
FROM part_bodies_3d
WHERE part_id = $1
  AND active = TRUE
ORDER BY name, id;
