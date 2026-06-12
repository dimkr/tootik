package migrations

import (
	"context"
	"database/sql"
)

func inboxraw(ctx context.Context, domain string, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `CREATE TABLE newinbox(id INTEGER PRIMARY KEY, sender TEXT NOT NULL, activity JSONB, raw TEXT NOT NULL, inserted INTEGER DEFAULT (UNIXEPOCH()), path TEXT)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO newinbox(id, sender, activity, raw, inserted, path) SELECT id, sender, activity, raw, inserted, path FROM inbox`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DROP TABLE inbox`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE newinbox RENAME TO inbox`); err != nil {
		return err
	}

	_, err := tx.ExecContext(ctx, `CREATE UNIQUE INDEX inboxid ON inbox(coalesce(activity->>'$.id', raw->>'$.id'))`)
	return err
}
