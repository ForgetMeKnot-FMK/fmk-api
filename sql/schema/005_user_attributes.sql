-- +goose Up
-- One encrypted attribute blob per user; data encrypted under masterKey
CREATE TABLE user_attributes (
  -- PK = FK enforces exactly one row per user
  user_id UUID PRIMARY KEY REFERENCES users(id) ON DELETE CASCADE,
  -- XChaCha20-Poly1305(masterKey, arbitrary client JSON): ciphertext||tag
  data BYTEA NOT NULL,
  -- 24-byte XChaCha20 nonce for data
  data_iv BYTEA NOT NULL,
  -- note: need to update this when the encryption process changes
  key_version INT NOT NULL DEFAULT 1,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- +goose Down
DROP TABLE user_attributes;