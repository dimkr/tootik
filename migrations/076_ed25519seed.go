package migrations

import (
	"context"
	"database/sql"
)

func ed25519seed(ctx context.Context, domain string, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `ALTER TABLE persons RENAME COLUMN ed25519privkey TO ed25519seed`)
	return err
}
