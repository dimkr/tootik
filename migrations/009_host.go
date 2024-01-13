package migrations

import (
	"context"
	"database/sql"
)

func host(ctx context.Context, domain string, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `ALTER TABLE persons ADD COLUMN host TEXT AS (substr(substr(id, 9), 0, instr(substr(id, 9), '/')))`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX personshost on persons(host)`); err != nil {
		return err
	}

	return nil
}
