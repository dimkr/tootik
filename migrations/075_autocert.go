package migrations

import (
	"context"
	"database/sql"
)

func autocert(ctx context.Context, domain string, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `CREATE TABLE autocert_cache(key TEXT PRIMARY KEY, value BLOB)`)
	return err
}
