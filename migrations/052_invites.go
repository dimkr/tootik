package migrations

import (
	"context"
	"database/sql"
)

func invites(ctx context.Context, domain string, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `CREATE TABLE invites(id TEXT NOT NULL, certhash TEXT, inviter TEXT NOT NULL, invited TEXT, inserted INTEGER DEFAULT (UNIXEPOCH()))`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE UNIQUE INDEX invitescerthash ON invites(certhash)`); err != nil {
		return err
	}

	_, err := tx.ExecContext(ctx, `CREATE INDEX invitesinviter ON invites(inviter)`)
	return err
}
