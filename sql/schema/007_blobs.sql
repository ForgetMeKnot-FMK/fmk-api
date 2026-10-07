-- +goose Up
-- Encrypted images (and later other files) used by knots. The bytes live in a folder on disk
-- (BLOB_DIR), one file per blob, named by its id. The server can't read them.
CREATE TABLE blobs (
  -- made by the client, so a blob added offline keeps its id
  id UUID PRIMARY KEY,
  owner_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  -- the blob's random key, wrapped as XChaCha20-Poly1305(masterKey, blobKey), ad "blob_key"
  wrapped_key BYTEA NOT NULL,
  -- 24-byte nonce for wrapped_key
  wrapped_key_iv BYTEA NOT NULL,
  -- size of the encrypted bytes
  size BIGINT NOT NULL,
  -- SHA-256 of the encrypted bytes (checks uploads and the disk; tells nothing about the content)
  sha256 BYTEA NOT NULL,
  -- keyed hash of the original file, for the same user's duplicates (NULL: none sent)
  dedup_tag BYTEA,
  key_version INT NOT NULL DEFAULT 1,
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  -- since when no knot uses it (NULL: in use); deleted after a while
  unreferenced_since TIMESTAMPTZ DEFAULT now()
);
--
-- one blob per file and user: the same file uploaded again reuses it
CREATE UNIQUE INDEX idx_blobs_owner_dedup_tag ON blobs (owner_id, dedup_tag)
WHERE dedup_tag IS NOT NULL;
--
-- for the cleanup job
CREATE INDEX idx_blobs_unreferenced_since ON blobs (unreferenced_since)
WHERE unreferenced_since IS NOT NULL;
--
-- Which knots use which blobs: one row per knot and blob, however often the knot shows it.
-- A blob's reference count is its number of rows
CREATE TABLE knot_blobs (
  knot_id UUID NOT NULL REFERENCES knots(id) ON DELETE CASCADE,
  blob_id UUID NOT NULL REFERENCES blobs(id) ON DELETE CASCADE,
  PRIMARY KEY (knot_id, blob_id)
);
--
CREATE INDEX idx_knot_blobs_blob_id ON knot_blobs (blob_id);
--
-- +goose Down
DROP TABLE knot_blobs;
--
DROP TABLE blobs;
