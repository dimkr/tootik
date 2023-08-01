package migrations

import (
	"context"
	"database/sql"
)

func notesupdated(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `ALTER TABLE notes ADD COLUMN updated INTEGER DEFAULT 0`); err != nil {
		return err
	}

	return nil
}
