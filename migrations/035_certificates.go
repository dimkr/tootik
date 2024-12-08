package migrations

import (
	"context"
	"database/sql"
)

func certificates(ctx context.Context, domain string, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `CREATE TABLE certificates(user TEXT NOT NULL, hash TEXT NOT NULL)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO certificates(user, hash) SELECT id, certhash FROM persons WHERE certhash IS NOT NULL`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DROP INDEX personscerthash`); err != nil {
		return err
	}

	_, err := tx.ExecContext(ctx, `CREATE UNIQUE INDEX certificateshash ON certificates(hash)`)
	return err
}
