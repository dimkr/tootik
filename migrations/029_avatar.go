package migrations

import (
	"context"
	"database/sql"
)

func avatar(ctx context.Context, domain string, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `CREATE UNIQUE INDEX iconsname ON icons(name)`)
	return err
}
