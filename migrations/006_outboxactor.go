package migrations

import (
	"context"
	"database/sql"
)

func outboxactor(ctx context.Context, domain string, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `CREATE INDEX outboxactor ON outbox(activity->>'actor')`); err != nil {
		return err
	}

	return nil
}
