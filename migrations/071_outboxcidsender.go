package migrations

import (
	"context"
	"database/sql"
)

func outboxcidsender(ctx context.Context, domain string, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `CREATE INDEX outboxcidsender ON outbox(cid, sender)`)
	return err
}
