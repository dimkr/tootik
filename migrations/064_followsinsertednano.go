package migrations

import (
	"context"
	"database/sql"
)

func followsinsertednano(ctx context.Context, domain string, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `CREATE TABLE nfollows(id TEXT NOT NULL PRIMARY KEY, follower TEXT NOT NULL, insertednano INTEGER NOT NULL, accepted INTEGER, followed TEXT)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO nfollows(id, follower, insertednano, accepted, followed) SELECT id, follower, inserted * 1000000000, accepted, followed FROM follows`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DROP TABLE follows`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE nfollows RENAME TO follows`); err != nil {
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
