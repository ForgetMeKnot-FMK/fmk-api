-- +goose Up
CREATE TABLE users (
  --
  -- Identity
  --
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  name TEXT NOT NULL,
  username TEXT UNIQUE NOT NULL,
  --
  -- Auth
  --
  auth_hash TEXT NOT NULL,
  --
  -- KDF descriptor (client reproduces stretchedKEK from password + these)
  --
  kdf_algo INT NOT NULL DEFAULT 2,
  kdf_salt BYTEA NOT NULL,
  kdf_mem_kib INT NOT NULL,
  kdf_iterations INT NOT NULL,
  --
  -- Encryption
  --
  -- -- AES-GCM(stretchedKEK, masterKey)
  key_version INT NOT NULL DEFAULT 1,
  encrypted_master_key BYTEA NOT NULL,
  encrypted_master_key_iv BYTEA NOT NULL,
  --
  --   -- AES-GCM(stretchedKEK, privKey)
  encrypted_priv_key BYTEA NOT NULL,
  encrypted_priv_key_iv BYTEA NOT NULL,
  --
    -- Asymmetric key encryption for sharing only (not needed now)
  pub_key_x25519 BYTEA NOT NULL,
  --
  -- Invalidation: rotate on password/key change to kill all live sessions.
  -- security_stamp UUID NOT NULL DEFAULT gen_random_uuid(),
  --
  -- Dates
  --
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
--
-- +goose Down
DROP TABLE users;