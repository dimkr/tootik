package migrations

import (
	"context"
	"database/sql"
)

func personspreferredusername(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS personspreferredusername ON persons(actor->>'preferredUsername')`); err != nil {
		return err
	}

	return nil
}
