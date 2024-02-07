package migrations

import (
	"context"
	"database/sql"
)

func certhash(ctx context.Context, domain string, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `DROP INDEX personscerthash`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE UNIQUE INDEX personscerthash ON persons(certhash) WHERE certhash IS NOT NULL`); err != nil {
		return err
	}

	return nil
}
