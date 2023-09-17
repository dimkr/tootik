package migrations

import (
	"context"
	"database/sql"
)

func edits(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `CREATE INDEX outboxobjectid ON outbox(activity->>'object.id')`); err != nil {
		return err
	}

	return nil
}
