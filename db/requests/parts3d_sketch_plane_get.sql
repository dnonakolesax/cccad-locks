SELECT plane
FROM sketches
WHERE id = $1::uuid
  AND deleted_at IS NULL
