package migrations

import (
	"context"
	"database/sql"
)

func backfill(ctx context.Context, domain string, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `CREATE INDEX historycreateupdateobjectid ON history(activity->>'$.object.id') WHERE (activity->>'$.type' = 'Create' or activity->>'$.type' = 'Update') and activity->>'$.object.id' IS NOT NULL`)
	return err
}
