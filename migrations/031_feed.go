package migrations

import (
	"context"
	"database/sql"
)

func feed(ctx context.Context, domain string, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `CREATE TABLE feed(follower STRING NOT NULL, note STRING NOT NULL, sharer STRING, inserted INTEGER NOT NULL)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX feedfollower ON feed(follower)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX feedinserted ON feed(inserted)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX feednote ON feed(note)`); err != nil {
		return err
	}

	return nil
}
