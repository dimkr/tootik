package migrations

import (
	"context"
	"database/sql"
)

func outboxhost(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `ALTER TABLE outbox ADD COLUMN host TEXT AS (substr(substr(activity->>'id', 9), 0, instr(substr(activity->>'id', 9), '/')))`); err != nil {
		return err
	}

	_, err := tx.ExecContext(ctx, `CREATE INDEX outboxhostinserted ON outbox(host, inserted)`)
	return err
}
