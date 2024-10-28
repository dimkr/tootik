package migrations

import (
	"context"
	"database/sql"
)

func checkers(ctx context.Context, domain string, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `CREATE TABLE checkers(human STRING, orc STRING, state STRING NOT NULL, inserted INTEGER DEFAULT (UNIXEPOCH()), winner STRING, ended INTEGER)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX checkersendedinserted ON checkers(ended, inserted)`); err != nil {
		return err
	}

	return nil
}
