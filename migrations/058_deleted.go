package migrations

import (
	"context"
	"database/sql"
)

func deleted(ctx context.Context, domain string, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `ALTER TABLE notes ADD COLUMN deleted INTEGER NOT NULL DEFAULT 0`)
	return err
}
