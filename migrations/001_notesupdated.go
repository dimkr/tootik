package migrations

import (
	"context"
	"database/sql"
)

func notesupdated(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `ALTER TABLE notes ADD COLUMN updated INTEGER DEFAULT 0`); err != nil {
		return err
	}

	return nil
}
