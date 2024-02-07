package migrations

import (
	"context"
	"database/sql"
)

func followeds(ctx context.Context, domain string, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `ALTER TABLE follows ADD COLUMN followeds STRING`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `UPDATE follows SET followeds = followed`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DROP INDEX followsfollowed`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE follows DROP COLUMN followed`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE follows RENAME COLUMN followeds TO followed`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX followsfollowed ON follows(followed)`); err != nil {
		return err
	}

	return nil
}
