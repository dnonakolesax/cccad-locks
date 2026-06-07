SELECT
    id::text,
    part_id::text,
    COALESCE(sketch_id::text, '') AS sketch_id,
    type::text,
    payload,
    order_index,
    suppressed,
    COALESCE(created_by::text, '') AS created_by,
    created_at,
    updated_at
FROM features_3d
WHERE part_id = $1
  AND deleted_at IS NULL
  AND ($2::boolean OR suppressed = FALSE)
ORDER BY order_index, id;
