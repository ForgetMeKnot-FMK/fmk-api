-- name: CreateKnotAttributes :one
INSERT INTO knot_attributes (knot_id, owner_id, data, data_iv, key_version, revision)
SELECT k.id,
  k.owner_id,
  @data::bytea,
  @data_iv::bytea,
  @key_version::int,
  @revision::bigint
FROM knots k
WHERE k.id = @knot_id
  AND k.owner_id = @owner_id
  AND k.deleted_revision IS NULL
RETURNING *;
--
-- name: UpdateKnotAttributes :one
UPDATE knot_attributes
SET data = @data,
  data_iv = @data_iv,
  revision = @revision,
  updated_at = now()
WHERE knot_attributes.knot_id = @knot_id
  AND knot_attributes.owner_id = @owner_id
  AND EXISTS (
    SELECT 1
    FROM knots k
    WHERE k.id = knot_attributes.knot_id
      AND k.deleted_revision IS NULL
  )
RETURNING *;
--
-- name: GetKnotAttributesByKnotID :one
SELECT a.*
FROM knot_attributes a
  INNER JOIN knots k ON k.id = a.knot_id
WHERE a.knot_id = $1
  AND a.owner_id = $2
  AND k.deleted_revision IS NULL;
--
-- name: GetKnotAttributesByOwnerID :many
SELECT a.*
FROM knot_attributes a
  INNER JOIN knots k ON k.id = a.knot_id
WHERE a.owner_id = $1
  AND k.deleted_revision IS NULL
ORDER BY a.updated_at DESC;
--
-- name: GetChangedKnotAttributes :many
SELECT a.*
FROM knot_attributes a
  INNER JOIN knots k ON k.id = a.knot_id
WHERE a.owner_id = @owner_id
  AND a.revision > @since::bigint
  AND k.deleted_revision IS NULL;
--
-- Attributes of deleted knots (the trash), last deleted first
-- name: GetDeletedKnotAttributes :many
SELECT a.*
FROM knot_attributes a
  INNER JOIN knots k ON k.id = a.knot_id
WHERE a.owner_id = @owner_id
  AND k.deleted_revision IS NOT NULL
ORDER BY k.deleted_revision DESC;
--
-- Gives a restored knot's attributes the new revision, so GET /sync sends them again
-- name: StampKnotAttributes :one
UPDATE knot_attributes
SET revision = @revision::bigint
WHERE knot_id = @knot_id
  AND owner_id = @owner_id
RETURNING *;
