package migrations

import (
	"context"
	"database/sql"
)

func invites(ctx context.Context, domain string, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `CREATE TABLE invites(ed25519privkey TEXT NOT NULL, by TEXT NOT NULL, inserted INTEGER DEFAULT (UNIXEPOCH()))`); err != nil {
		return err
	}

	_, err := tx.ExecContext(ctx, `CREATE INDEX invitesby ON invites(by)`)
	return err
}
