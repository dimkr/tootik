package migrations

import (
	"context"
	"database/sql"
)

func resolvegroup(ctx context.Context, domain string, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `DROP INDEX personspreferredusernamehost`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE UNIQUE INDEX personspreferredusernametypehost ON persons(actor->>'$.preferredUsername', actor->>'$.type', host)`); err != nil {
		return err
	}

	return nil
}
