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
--
-- A new password: the hash that proves it, the KDF settings, and the keys wrapped with the
-- key it derives (the master key itself is the same; the X25519 pair is new)
-- name: UpdateUserPassword :exec
UPDATE users
SET auth_hash = $2,
  kdf_algo = $3,
  kdf_salt = $4,
  kdf_mem_kib = $5,
  kdf_iterations = $6,
  encrypted_master_key = $7,
  encrypted_master_key_iv = $8,
  encrypted_priv_key = $9,
  encrypted_priv_key_iv = $10,
  pub_key_x25519 = $11,
  updated_at = now()
WHERE id = $1;
