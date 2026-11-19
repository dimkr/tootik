package migrations

import (
	"context"
	"database/sql"
)

func iconscididx(ctx context.Context, domain string, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `CREATE UNIQUE INDEX iconscid ON icons(cid)`)
	return err
}
