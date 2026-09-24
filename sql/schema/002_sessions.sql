-- +goose Up
CREATE TABLE sessions (
  id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
  user_id UUID NOT NULL REFERENCES users(id) ON DELETE CASCADE,
  -- SHA-256 of the opaque bearer token (for lookup)
  token_hash BYTEA NOT NULL UNIQUE,
  device_label TEXT,
  ip INET,
  user_agent TEXT,
  last_used_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
  expires_at TIMESTAMPTZ NOT NULL,
  revoked_at TIMESTAMPTZ
);
--
CREATE INDEX idx_sessions_user_id ON sessions (user_id);
--
CREATE INDEX idx_sessions_token_hash ON sessions (token_hash);
--
-- +goose Down
DROP INDEX idx_sessions_user_id;
--
DROP INDEX idx_sessions_token_hash;
--
DROP TABLE sessions;