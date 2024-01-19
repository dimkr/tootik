package migrations

import (
	"context"
	"database/sql"
)

func shares(ctx context.Context, domain string, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `CREATE TABLE shares(note STRING NOT NULL, by STRING NOT NULL, inserted INTEGER DEFAULT (UNIXEPOCH()))`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX sharesnote ON shares(note)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DROP INDEX notesgroupid`); err != nil {
		return err
	}

	_, err := tx.ExecContext(ctx, `CREATE INDEX notesaudience ON notes(object->>'audience')`)
	return err
}
