-- name: CreateBlob :one
INSERT INTO blobs (
    id,
    owner_id,
    wrapped_key,
    wrapped_key_iv,
    size,
    sha256,
    dedup_tag
  )
VALUES (
    @id,
    @owner_id,
    @wrapped_key,
    @wrapped_key_iv,
    @size,
    @sha256,
    sqlc.narg('dedup_tag')
  )
RETURNING *;
--
-- name: GetBlob :one
SELECT *
FROM blobs
WHERE id = $1;
--
-- name: GetBlobByOwner :one
SELECT *
FROM blobs
WHERE id = $1
  AND owner_id = $2;
--
-- name: GetBlobByTag :one
SELECT *
FROM blobs
WHERE owner_id = $1
  AND dedup_tag = $2;
--
-- Takes off a knot the blobs it no longer uses (not in `blob_ids`), and gives their ids
-- name: RemoveKnotBlobsNotIn :many
DELETE FROM knot_blobs
WHERE knot_id = @knot_id
  AND NOT (blob_id = ANY(@blob_ids::uuid []))
RETURNING blob_id;
--
-- Links a knot to the blobs it uses, only the owner's (others are skipped)
-- name: AddKnotBlobs :exec
INSERT INTO knot_blobs (knot_id, blob_id)
SELECT @knot_id,
  b.id
FROM blobs b
WHERE b.id = ANY(@blob_ids::uuid [])
  AND b.owner_id = @owner_id ON CONFLICT DO NOTHING;
--
-- name: MarkBlobsReferenced :exec
UPDATE blobs
SET unreferenced_since = NULL
WHERE id = ANY(@blob_ids::uuid [])
  AND owner_id = @owner_id
  AND unreferenced_since IS NOT NULL;
--
-- Notes since when the given blobs are unused, for those no knot uses any more
-- name: MarkBlobsUnreferenced :exec
UPDATE blobs
SET unreferenced_since = now()
WHERE id = ANY(@blob_ids::uuid [])
  AND unreferenced_since IS NULL
  AND NOT EXISTS (
    SELECT 1
    FROM knot_blobs kb
    WHERE kb.blob_id = blobs.id
  );
--
-- name: ListExpiredBlobs :many
SELECT id
FROM blobs
WHERE unreferenced_since < @before;
--
-- Deletes a blob, only if it is still unused since before `before` (checked again here)
-- name: DeleteExpiredBlob :execrows
DELETE FROM blobs
WHERE id = @id
  AND unreferenced_since < @before
  AND NOT EXISTS (
    SELECT 1
    FROM knot_blobs kb
    WHERE kb.blob_id = blobs.id
  );
--
-- Which of these ids have a row (for removing files that have none)
-- name: ExistingBlobIDs :many
SELECT id
FROM blobs
WHERE id = ANY(@ids::uuid []);
--
-- name: KnotBlobIDs :many
SELECT blob_id
FROM knot_blobs
WHERE knot_id = $1;
--
-- How many of these blobs the user owns, and their total size (for the per-knot quota)
-- name: BlobsTotal :one
SELECT count(*)::bigint AS count,
  coalesce(sum(size), 0)::bigint AS size
FROM blobs
WHERE id = ANY(@ids::uuid [])
  AND owner_id = @owner_id;
