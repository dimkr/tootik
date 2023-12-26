package migrations

import (
	"context"
	"database/sql"
)

func namehost(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `DROP INDEX personspreferredusername`); err != nil {
		return err
	}

	_, err := tx.ExecContext(ctx, `CREATE UNIQUE INDEX personspreferredusernamehost ON persons(actor->>'preferredUsername', host)`)
	return err
}
