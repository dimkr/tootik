package migrations

import (
	"context"
	"database/sql"
)

func reject(ctx context.Context, domain string, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `DROP INDEX followsfollower`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DROP INDEX followsfollowed`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE TABLE newfollows(id STRING NOT NULL PRIMARY KEY, follower STRING NOT NULL, inserted INTEGER DEFAULT (UNIXEPOCH()), accepted INTEGER, followed STRING)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO newfollows(id, follower, inserted, accepted, followed) SELECT id, follower, inserted, accepted, followed FROM follows`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DROP TABLE follows`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE newfollows RENAME TO follows`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX followsfollower ON follows(follower)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX followsfollowed ON follows(followed)`); err != nil {
		return err
	}

	_, err := tx.ExecContext(ctx, `CREATE UNIQUE INDEX followsfollowerfollowed ON follows(follower, followed)`)
	return err
}
