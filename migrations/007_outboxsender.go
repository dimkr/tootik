package migrations

import (
	"context"
	"database/sql"
)

func outboxsender(ctx context.Context, domain string, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `ALTER TABLE outbox ADD COLUMN sender STRING`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `UPDATE outbox SET sender = activity->>'actor'`); err != nil {
		return err
	}

	return nil
}
