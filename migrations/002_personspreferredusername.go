package migrations

import (
	"context"
	"database/sql"
)

func personspreferredusername(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS personspreferredusername ON persons(actor->>'preferredUsername')`); err != nil {
		return err
	}

	return nil
}
