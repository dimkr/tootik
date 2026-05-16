package migrations

import (
	"context"
	"database/sql"
)

func openpolls(ctx context.Context, domain string, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `CREATE INDEX notesopenpolls ON notes(id) WHERE object->>'$.type' = 'Question' AND deleted = 0 AND object->>'$.closed' IS NULL`)
	return err
}
