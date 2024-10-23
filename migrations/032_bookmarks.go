package migrations

import (
	"context"
	"database/sql"
)

func bookmarks(ctx context.Context, domain string, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `CREATE TABLE bookmarks(note STRING NOT NULL, by STRING NOT NULL, inserted INTEGER DEFAULT (UNIXEPOCH()))`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX bookmarksnote ON bookmarks(note)`); err != nil {
		return err
	}

	_, err := tx.ExecContext(ctx, `CREATE UNIQUE INDEX bookmarksbynote ON bookmarks(by, note)`)
	return err
}
