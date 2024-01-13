package migrations

import (
	"context"
	"database/sql"
)

func iconsname(ctx context.Context, domain string, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `DROP TABLE icons`); err != nil {
		return err
	}

	_, err := tx.ExecContext(ctx, `CREATE TABLE icons(name STRING NOT NULL PRIMARY KEY, buf BLOB NOT NULL)`)
	return err
}
