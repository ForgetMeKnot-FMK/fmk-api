-- +goose Up
-- Small per-knot attribute rows for bulk list rendering; data encrypted under masterKey
CREATE TABLE knot_attributes (
  -- PK = FK enforces the 1:1 relationship with knots
  knot_id UUID PRIMARY KEY REFERENCES knots(id) ON DELETE CASCADE,
  -- duplicated here so the bulk list query needs no join to knots
  owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  -- XChaCha20-Poly1305(masterKey, arbitrary client JSON): ciphertext||tag
  data BYTEA NOT NULL,
  -- 24-byte XChaCha20 nonce for data
  data_iv BYTEA NOT NULL,
  -- note: need to update this when the encryption process changes
  key_version INT NOT NULL DEFAULT 1,
  updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
-- single indexed scan to pull all attribute rows for one user
CREATE INDEX idx_knot_attributes_owner_id ON knot_attributes (owner_id);
-- +goose Down
DROP TABLE knot_attributes;