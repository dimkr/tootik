package migrations

import (
	"context"
	"database/sql"
)

func audience(ctx context.Context, domain string, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `DROP INDEX notesgroupid`); err != nil {
		return err
	}

	_, err := tx.ExecContext(ctx, `CREATE INDEX notesaudience ON notes(object->>'audience')`)
	return err
}
