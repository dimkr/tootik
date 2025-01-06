package migrations

import (
	"context"
	"database/sql"
)

func rawforward(ctx context.Context, domain string, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `CREATE TABLE newinbox(id INTEGER PRIMARY KEY, sender STRING NOT NULL, activity STRING NOT NULL, raw STRING NOT NULL, inserted INTEGER DEFAULT (UNIXEPOCH()))`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO newinbox(id, sender, activity, raw, inserted) SELECT id, sender, activity, activity AS raw, inserted FROM inbox`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DROP INDEX inboxid`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DROP TABLE inbox`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE newinbox RENAME TO inbox`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE UNIQUE INDEX inboxid ON inbox(activity->>'$.id')`); err != nil {
		return err
	}

	return nil
}
