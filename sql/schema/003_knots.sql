-- +goose Up
-- Encrypted markdown knots. Content is to be encrypted under a per-knot
-- key wrapped by the user's `masterKey`
CREATE TABLE knots (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  -- XChaCha20-Poly1305(knotKey, markdown): [ciphertext||tag]
  encrypted_content BYTEA NOT NULL,
  -- 24-byte XChaCha20 nonce for encrypted_content
  encrypted_content_iv BYTEA NOT NULL,
  -- per-knot random key, wrapped as XChaCha20-Poly1305(masterKey, knotKey)
  wrapped_knot_key BYTEA NOT NULL,
  -- 24-byte nonce for wrapped_knot_key
  wrapped_knot_key_iv BYTEA NOT NULL,
  -- optional encrypted title (ciphertext||tag)
  encrypted_title BYTEA,
  -- 24-byte nonce for encrypted_title
  encrypted_title_iv BYTEA,
  -- note: need to update this when the encryption process changes
  key_version INT NOT NULL DEFAULT 1,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
--
-- fast lookup of all knots for one user
CREATE INDEX idx_knots_owner_id ON knots (owner_id);
-- +goose Down
DROP TABLE knots;