package migrations

import (
	"context"
	"database/sql"
)

func actorttl(ctx context.Context, domain string, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `CREATE INDEX personsupdated ON persons(updated)`)
	return err
}
