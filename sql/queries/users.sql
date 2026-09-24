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
WHERE username = $1;