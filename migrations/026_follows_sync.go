package migrations

import (
	"context"
	"database/sql"
)

func follows_sync(ctx context.Context, domain string, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `CREATE TABLE follows_sync(actor STRING NOT NULL PRIMARY KEY, url STRING NOT NULL, digest STRING NOT NULL, changed INTEGER NOT NULL DEFAULT (UNIXEPOCH()), synced INTEGER DEFAULT 0)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX followssyncsynced ON follows_sync(synced) WHERE synced < changed`); err != nil {
		return err
	}

	return nil
}
