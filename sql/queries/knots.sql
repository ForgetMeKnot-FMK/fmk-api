-- name: CreateKnot :one
INSERT INTO knots(
    owner_id,
    encrypted_content,
    encrypted_content_iv,
    wrapped_knot_key,
    wrapped_knot_key_iv,
    encrypted_title,
    encrypted_title_iv,
    key_version
  )
VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6,
    $7,
    $8
  )
RETURNING *;
--
-- name: UpdateKnot :one
UPDATE knots
SET encrypted_content = $3,
  encrypted_content_iv = $4,
  wrapped_knot_key = $5,
  wrapped_knot_key_iv = $6,
  encrypted_title = $7,
  encrypted_title_iv = $8,
  updated_at = $9
WHERE id = $1
  AND owner_id = $2
RETURNING *;
--
--
-- name: GetKnotsByOwnerID :many
SELECT *
FROM knots AS k
WHERE k.owner_id = $1
ORDER BY k.updated_at DESC
LIMIT $2 OFFSET $3;
--
-- name: GetKnotByIDAndOwner :one
SELECT *
FROM knots
WHERE id = $1
  AND owner_id = $2;