package migrations

import (
	"context"
	"database/sql"
)

func quote(ctx context.Context, domain string, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `CREATE INDEX notesquote ON notes(object->>'$.quote') WHERE object->>'$.quote' IS NOT NULL`)
	return err
}
