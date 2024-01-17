package migrations

import (
	"context"
	"database/sql"
)

func nohash(ctx context.Context, domain string, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `DROP INDEX notesidhash`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE notes DROP COLUMN hash`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DROP INDEX personsidhash`); err != nil {
		return err
	}

	_, err := tx.ExecContext(ctx, `ALTER TABLE persons DROP COLUMN hash`)
	return err
}
