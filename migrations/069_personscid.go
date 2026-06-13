package migrations

import (
	"context"
	"database/sql"
)

func personscid(ctx context.Context, domain string, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `CREATE INDEX personscid ON persons(cid)`)
	return err
}
