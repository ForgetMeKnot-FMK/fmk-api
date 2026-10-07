-- name: CreateKnot :one
INSERT INTO knots(
    owner_id,
    encrypted_content,
    encrypted_content_iv,
    wrapped_knot_key,
    wrapped_knot_key_iv,
    encrypted_title,
    encrypted_title_iv,
    key_version,
    revision
  )
VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7,
    $8,
    $9
  )
RETURNING *;
--
-- name: UpdateKnot :one
-- expected_revision: the revision the client's version is based on. The update only happens when
-- the knot still has it (NULL: no check, for older clients)
UPDATE knots
SET encrypted_content = @encrypted_content,
  encrypted_content_iv = @encrypted_content_iv,
  wrapped_knot_key = @wrapped_knot_key,
  wrapped_knot_key_iv = @wrapped_knot_key_iv,
  encrypted_title = @encrypted_title,
  encrypted_title_iv = @encrypted_title_iv,
  updated_at = @updated_at,
  revision = @revision
WHERE id = @id
  AND owner_id = @owner_id
  AND deleted_revision IS NULL
  AND (
    sqlc.narg('expected_revision')::bigint IS NULL
    OR revision = sqlc.narg('expected_revision')
  )
RETURNING *;
--
--
-- name: GetKnotsByOwnerID :many
SELECT *
FROM knots AS k
WHERE k.owner_id = $1
  AND k.deleted_revision IS NULL
ORDER BY k.updated_at DESC
LIMIT $2 OFFSET $3;
--
-- name: GetKnotByIDAndOwner :one
SELECT *
FROM knots
WHERE id = $1
  AND owner_id = $2
  AND deleted_revision IS NULL;
--
-- Marks the knot as deleted (`deleted_revision`). The row stays, so GET /sync
-- can report the delete. The other queries here skip deleted knots.
-- name: SoftDeleteKnot :execrows
UPDATE knots
SET deleted_revision = @revision::bigint,
  revision = @revision::bigint,
  updated_at = now()
WHERE id = @id
  AND owner_id = @owner_id
  AND deleted_revision IS NULL;
--
-- name: GetChangedKnotIDs :many
SELECT id
FROM knots
WHERE owner_id = @owner_id
  AND revision > @since::bigint
  AND deleted_revision IS NULL;
--
-- name: GetDeletedKnotIDs :many
SELECT id
FROM knots
WHERE owner_id = @owner_id
  AND deleted_revision > @since::bigint;
--
-- Undoes SoftDeleteKnot. The new revision makes GET /sync report the knot again.
-- name: RestoreKnot :execrows
UPDATE knots
SET deleted_revision = NULL,
  revision = @revision::bigint,
  updated_at = now()
WHERE id = @id
  AND owner_id = @owner_id
  AND deleted_revision IS NOT NULL;
