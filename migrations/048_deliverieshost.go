package migrations

import (
	"context"
	"database/sql"
)

func deliverieshost(ctx context.Context, domain string, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `CREATE TABLE ndeliveries(activity TEXT NOT NULL, host TEXT NOT NULL)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO ndeliveries(activity, host) SELECT DISTINCT activity, substr(substr(inbox, 9), 0, instr(substr(inbox, 9), '/')) FROM deliveries`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DROP TABLE deliveries`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE ndeliveries RENAME TO deliveries`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX deliveriesactivity ON deliveries(activity)`); err != nil {
		return err
	}

	return nil
}
