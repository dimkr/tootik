package migrations

import (
	"context"
	"database/sql"
)

func received(ctx context.Context, domain string, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `ALTER TABLE outbox ADD COLUMN received TEXT`)
	return err
}
