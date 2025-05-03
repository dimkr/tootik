package migrations

import (
	"context"
	"database/sql"
)

func autodel(ctx context.Context, domain string, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `ALTER TABLE persons ADD COLUMN ttl INTEGER`); err != nil {
		return err
	}

	_, err := tx.ExecContext(ctx, `CREATE UNIQUE INDEX personsidttl ON persons(id) WHERE ttl IS NOT NULL`)
	return err
}
