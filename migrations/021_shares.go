package migrations

import (
	"context"
	"database/sql"
)

func shares(ctx context.Context, domain string, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `CREATE TABLE shares(note STRING NOT NULL, by STRING NOT NULL, inserted INTEGER DEFAULT (UNIXEPOCH()))`); err != nil {
		return err
	}

	_, err := tx.ExecContext(ctx, `CREATE UNIQUE INDEX sharesnote ON shares(note)`)
	return err
}
