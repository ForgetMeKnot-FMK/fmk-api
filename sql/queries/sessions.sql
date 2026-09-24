-- name: CreateSession :one
INSERT INTO sessions(
    user_id,
    token_hash,
    device_label,
    ip,
    user_agent,
    expires_at
  )
VALUES (
    $1,
    $2,
    $3,
    $4,
    $5,
    $6
  )
RETURNING *;
--
-- name: GetSessionsByUserID :many
SELECT *
FROM sessions AS s
WHERE s.user_id = $1
  AND s.revoked_at IS NULL
  AND s.expires_at > now();
-- SELECT *
-- FROM sessions as s
--   INNER JOIN users as u ON s.user_id = u.id
-- WHERE s.user_id = u.id
--   AND s.revoked_at IS NULL
--   AND s.expires_at > now();
--

-- name: GetSessionByTokenHash :one
SELECT *
FROM sessions
WHERE token_hash = $1
  AND s.revoked_at IS NULL
  AND s.expires_at > now();
--
-- name: DeleteSessionByID :exec
DELETE FROM sessions AS s
WHERE s.id = $1;