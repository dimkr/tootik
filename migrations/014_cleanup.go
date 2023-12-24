package migrations

import (
	"context"
	"database/sql"
)

func cleanup(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `DROP TABLE deliveries`)
	if err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DROP INDEX outboxobjectid`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX outboxobjectid ON outbox(activity->>'object.id') WHERE activity->>'object.id' IS NOT NULL`); err != nil {
		return err
	}

	return nil
}
