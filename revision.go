package main

import (
	"context"

	"github.com/ishraq10199/fmk-api/internal/database"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgtype"
)

/*
withRevision runs one write for a user inside a transaction.

Steps:
 1. Start a transaction.
 2. Add 1 to the user's revision (users.revision) and read the new value.
    This locks the user's row. Another write for the same user waits here
    until this one commits or rolls back.
 3. Call `write` with the new revision. It saves its rows with that revision.
 4. If anything fails, roll back. The revision change is undone too.
 5. Commit, and return the new revision.

Writes wait for each other at step 2. Higher revision == more recent commit
The GET `/sync` depends on this ordering.

Example: the user's current revision is 41.
  - Device A updates knot k1. The revision becomes 42, and k1 gets 42.
  - Device B (soft) deletes knot k2 at the same moment. It waits for A, then gets 43.
  - Device C calls GET `/sync?since=41` and gets both k1 and k2.
*/
func (apiCfg *apiConfig) withRevision(
	ctx context.Context,
	userID pgtype.UUID,
	write func(q *database.Queries, rev int64) error,
) (int64, error) {
	tx, err := apiCfg.Pool.Begin(ctx)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback(ctx) // no-op once committed

	q := apiCfg.DB.WithTx(tx)
	rev, err := q.BumpUserRevision(ctx, userID)
	if err != nil {
		return 0, err
	}
	if err := write(q, rev); err != nil {
		return 0, err
	}
	return rev, tx.Commit(ctx)
}

/*
readSnapshot runs several reads on one fixed view of the database.
Writes that commit in the meantime are not seen, so the reads match each
other. Example: GET `/sync` reads the revision and then the changed rows.
*/
func (apiCfg *apiConfig) readSnapshot(ctx context.Context, read func(q *database.Queries) error) error {
	tx, err := apiCfg.Pool.BeginTx(ctx, pgx.TxOptions{
		IsoLevel:   pgx.RepeatableRead,
		AccessMode: pgx.ReadOnly,
	})
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if err := read(apiCfg.DB.WithTx(tx)); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
