package migrations

import (
	"context"
	"database/sql"
)

func cache(ctx context.Context, domain string, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `CREATE TABLE cache(module TEXT, key TEXT, data BLOB, PRIMARY KEY (module, key))`)
	return err
}
