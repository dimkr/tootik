package migrations

import (
	"context"
	"database/sql"
)

func nomadic(ctx context.Context, domain string, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `ALTER TABLE persons ADD COLUMN did TEXT`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX personsdid ON persons(did) WHERE did IS NOT NULL`); err != nil {
		return err
	}

	return nil
}
