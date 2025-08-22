package migrations

import (
	"context"
	"database/sql"
)

func portable(ctx context.Context, domain string, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `ALTER TABLE persons ADD COLUMN did TEXT`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX personsdid ON persons(did) WHERE did IS NOT NULL`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX personspreferreduserlocal ON persons(actor->>'$.preferredUsername') WHERE ed25519privkey IS NOT NULL`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX personsidlocal ON persons(id) WHERE ed25519privkey IS NOT NULL`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX personsdidlocal ON persons(did) WHERE ed25519privkey IS NOT NULL AND did IS NOT NULL`); err != nil {
		return err
	}

	return nil
}
