package migrations

import (
	"context"
	"database/sql"
)

func noimage(ctx context.Context, domain string, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `UPDATE persons SET actor = json_remove(actor, '$.image') WHERE host = ?`, domain)
	return err
}
