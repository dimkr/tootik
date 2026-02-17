package migrations

import (
	"context"
	"database/sql"
)

func backfillcontext(ctx context.Context, domain string, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `CREATE INDEX localnotescontext ON notes(object->>'$.context') WHERE object->>'$.context' IS NOT NULL`)
	return err
}
