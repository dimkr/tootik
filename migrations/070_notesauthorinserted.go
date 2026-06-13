package migrations

import (
	"context"
	"database/sql"
)

func notesauthorinserted(ctx context.Context, domain string, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `CREATE INDEX notesauthorinserted ON notes(author, inserted)`); err != nil {
		return err
	}

	_, err := tx.ExecContext(ctx, `DROP INDEX notesauthor`)
	return err
}
