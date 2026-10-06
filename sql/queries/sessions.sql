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
  AND s.expires_at > now()
ORDER BY s.last_used_at DESC;
-- SELECT *
-- FROM sessions as s
--   INNER JOIN users as u ON s.user_id = u.id
-- WHERE s.user_id = u.id
--   AND s.revoked_at IS NULL
--   AND s.expires_at > now();
--

-- name: GetSessionByTokenHash :one
SELECT *
FROM sessions AS s
WHERE token_hash = $1
  AND s.revoked_at IS NULL
  AND s.expires_at > now();
--
-- name: GetUserByTokenHash :one
SELECT u.*
FROM sessions AS s
  INNER JOIN users AS u ON u.id = s.user_id
WHERE token_hash = $1
  AND s.revoked_at IS NULL
  AND s.expires_at > now();
--
-- name: DeleteSessionByID :exec
DELETE FROM sessions AS s
WHERE s.id = $1;
--
-- The user and the session a token belongs to (live sessions only)
-- name: GetAuthByTokenHash :one
SELECT sqlc.embed(u),
  s.id AS session_id,
  s.last_used_at AS session_last_used_at
FROM sessions AS s
  INNER JOIN users AS u ON u.id = s.user_id
WHERE s.token_hash = $1
  AND s.revoked_at IS NULL
  AND s.expires_at > now();
--
-- name: TouchSession :exec
UPDATE sessions
SET last_used_at = now()
WHERE id = $1;
--
-- Ends one live session of a user (0 rows: not theirs, or already ended)
-- name: RevokeSession :execrows
UPDATE sessions
SET revoked_at = now()
WHERE id = $1
  AND user_id = $2
  AND revoked_at IS NULL
  AND expires_at > now();
--
-- Ends all live sessions of a user but one (the current one)
-- name: RevokeOtherSessions :execrows
UPDATE sessions
SET revoked_at = now()
WHERE user_id = $1
  AND id <> $2
  AND revoked_at IS NULL
  AND expires_at > now();
