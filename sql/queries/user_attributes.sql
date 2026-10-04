-- name: GetUserAttributes :one
SELECT *
FROM user_attributes
WHERE user_id = $1;
--
-- name: CreateUserAttributes :one
INSERT INTO user_attributes (user_id, data, data_iv, key_version, revision)
VALUES ($1, $2, $3, $4, $5)
RETURNING *;
--
-- Updates only if the client saw the latest row (`expected_revision`).
-- No row back means the row is missing, or another device changed it first.
-- name: UpdateUserAttributes :one
UPDATE user_attributes
SET data = @data,
  data_iv = @data_iv,
  revision = @revision,
  updated_at = now()
WHERE user_id = @user_id
  AND revision = @expected_revision
RETURNING *;
