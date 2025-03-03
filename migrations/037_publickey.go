package migrations

import (
	"context"
	"database/sql"
)

func publickey(ctx context.Context, domain string, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `CREATE UNIQUE INDEX personspublickeyid ON persons(actor->>'$.publicKey.id')`)
	return err
}
