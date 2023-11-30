package migrations

import (
	"context"
	"database/sql"
)

func fetched(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `ALTER TABLE persons ADD COLUMN fetched INTEGER`)
	return err
}
