package migrations

import (
	"context"
	"database/sql"
)

func history(ctx context.Context, domain string, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `ALTER TABLE inbox ADD COLUMN path TEXT`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE TABLE history(path TEXT, public INTEGER NOT NULL, activity JSONB NOT NULL, inserted INTEGER NOT NULL)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX historyinserted ON history(inserted)`); err != nil {
		return err
	}

	_, err := tx.ExecContext(ctx, `CREATE UNIQUE INDEX historyactivityid ON history(activity->>'$.id')`)
	return err
}
