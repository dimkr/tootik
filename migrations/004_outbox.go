package migrations

import (
	"context"
	"database/sql"
)

func outbox(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `CREATE TABLE outbox(activity STRING NOT NULL, inserted INTEGER DEFAULT (UNIXEPOCH()), attempts INTEGER DEFAULT 0, last INTEGER DEFAULT (UNIXEPOCH()), sent INTEGER DEFAULT 0)`); err != nil {
		return err
	}

	if _, err := db.ExecContext(ctx, `CREATE UNIQUE INDEX outboxactivityid ON outbox(activity->>'id')`); err != nil {
		return err
	}

	if _, err := db.ExecContext(ctx, `CREATE INDEX outboxsentattempts ON outbox(sent, attempts)`); err != nil {
		return err
	}

	if _, err := db.ExecContext(ctx, `ALTER TABLE follows ADD accepted INTEGER DEFAULT 1`); err != nil {
		return err
	}

	if _, err := db.ExecContext(ctx, `ALTER TABLE activities RENAME TO inbox`); err != nil {
		return err
	}

	if _, err := db.ExecContext(ctx, `CREATE UNIQUE INDEX inboxid ON inbox(activity->>'id')`); err != nil {
		return err
	}

	if _, err := db.ExecContext(ctx, `DROP INDEX activitiesid`); err != nil {
		return err
	}

	return nil
}
