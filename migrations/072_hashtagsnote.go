package migrations

import (
	"context"
	"database/sql"
)

func hashtagsnote(ctx context.Context, domain string, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `CREATE INDEX hashtagsnote ON hashtags(note)`)
	return err
}
