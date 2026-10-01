-- name: CreateKnotAttributes :one
INSERT INTO knot_attributes (knot_id, owner_id, data, data_iv, key_version)
SELECT k.id,
  k.owner_id,
  @data::bytea,
  @data_iv::bytea,
  @key_version::int
FROM knots k
WHERE k.id = @knot_id
  AND k.owner_id = @owner_id
RETURNING *;
--
-- name: UpdateKnotAttributes :one
UPDATE knot_attributes
SET data = @data,
  data_iv = @data_iv,
  updated_at = now()
WHERE knot_id = @knot_id
  AND owner_id = @owner_id
RETURNING *;
--
-- name: GetKnotAttributesByKnotID :one
SELECT *
FROM knot_attributes
WHERE knot_id = $1
  AND owner_id = $2;
--
-- name: GetKnotAttributesByOwnerID :many
SELECT *
FROM knot_attributes
WHERE owner_id = $1
ORDER BY updated_at DESC;