package migrations

import (
	"context"
	"database/sql"
)

func shareactivity(ctx context.Context, domain string, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `ALTER TABLE shares ADD COLUMN activity TEXT`); err != nil {
		return err
	}

	_, err := tx.ExecContext(ctx, `CREATE UNIQUE INDEX sharesactivity ON shares(activity)`)
	return err
}
