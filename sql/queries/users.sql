-- name: CreateUser :one
INSERT INTO users(
    name,
    username,
    auth_hash,
    kdf_algo,
    kdf_salt,
    kdf_mem_kib,
    kdf_iterations,
    encrypted_master_key,
    encrypted_master_key_iv,
    pub_key_x25519,
    encrypted_priv_key,
    encrypted_priv_key_iv,
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
    $8,
    $9,
    $10,
    $11,
    $12,
    $13
  )
RETURNING *;
--
-- name: FindUserByUsername :one
SELECT *
FROM users
WHERE username = $1;--
-- Every write calls this first, inside its transaction.
-- It locks the user's row, so the user's writes run one at a time.
-- That way, a higher revision always means a later commit.
-- name: BumpUserRevision :one
UPDATE users
SET revision = revision + 1
WHERE id = $1
RETURNING revision;
--
-- name: GetUserRevision :one
SELECT revision
FROM users
WHERE id = $1;
