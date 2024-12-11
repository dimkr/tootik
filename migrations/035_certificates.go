package migrations

import (
	"context"
	"database/sql"
)

func certificates(ctx context.Context, domain string, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `CREATE TABLE certificates(user TEXT NOT NULL, hash TEXT NOT NULL, approved INTEGER DEFAULT 0, inserted INTEGER DEFAULT (UNIXEPOCH()))`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO certificates(user, hash, approved) SELECT actor->>'$.preferredUsername', certhash, 1 FROM persons WHERE certhash IS NOT NULL`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DROP INDEX personscerthash`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE persons DROP COLUMN certhash`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX certificatewaiting ON certificates(inserted) WHERE approved = 0`); err != nil {
		return err
	}

	_, err := tx.ExecContext(ctx, `CREATE UNIQUE INDEX certificateshash ON certificates(hash)`)
	return err
}
