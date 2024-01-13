package migrations

import (
	"context"
	"database/sql"
)

func noteshost(ctx context.Context, domain string, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `ALTER TABLE notes ADD COLUMN host TEXT AS (substr(substr(author, 9), 0, instr(substr(author, 9), '/')))`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX noteshostinserted on notes(host, inserted)`); err != nil {
		return err
	}

	return nil
}
