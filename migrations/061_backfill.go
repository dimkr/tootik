package migrations

import (
	"context"
	"database/sql"
)

func backfill(ctx context.Context, domain string, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `CREATE INDEX historycreateupdateobjectid ON history(activity->>'$.object.id') WHERE (activity->>'$.type' = 'Create' OR activity->>'$.type' = 'Update') AND activity->>'$.object.id' IS NOT NULL`)
	return err
}
