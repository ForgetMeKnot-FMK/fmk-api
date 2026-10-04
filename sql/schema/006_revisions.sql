-- +goose Up
-- Adds a revision counter to each user.
-- Every write adds 1 to users.revision, and saves the new value on the rows it changes.
-- Clients then ask "what changed since revision r?" (GET /sync).
ALTER TABLE users
ADD COLUMN revision BIGINT NOT NULL DEFAULT 0;
--
ALTER TABLE knots
ADD COLUMN revision BIGINT NOT NULL DEFAULT 0,
  -- soft delete: the revision that deleted the knot (NULL = not deleted)
ADD COLUMN deleted_revision BIGINT;
--
ALTER TABLE knot_attributes
ADD COLUMN revision BIGINT NOT NULL DEFAULT 0;
--
ALTER TABLE user_attributes
ADD COLUMN revision BIGINT NOT NULL DEFAULT 0;
--
-- for "changed since r" lookups of one user
CREATE INDEX idx_knots_owner_revision ON knots (owner_id, revision);
--
CREATE INDEX idx_knot_attributes_owner_revision ON knot_attributes (owner_id, revision);
--
-- +goose Down
DROP INDEX idx_knot_attributes_owner_revision;
--
DROP INDEX idx_knots_owner_revision;
--
ALTER TABLE user_attributes DROP COLUMN revision;
--
ALTER TABLE knot_attributes DROP COLUMN revision;
--
ALTER TABLE knots DROP COLUMN deleted_revision,
  DROP COLUMN revision;
--
ALTER TABLE users DROP COLUMN revision;
