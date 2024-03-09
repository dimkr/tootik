package migrations

import (
	"context"
	"database/sql"
)

func localforward(ctx context.Context, domain string, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `DROP INDEX outboxactivityid`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX outboxactivityid ON outbox(activity->>'$.id')`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE TABLE deliveries(activity STRING NOT NULL, inbox STRING NOT NULL)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX deliveriesactivity ON deliveries(activity)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE outbox DROP COLUMN received`); err != nil {
		return err
	}

	return nil
}
