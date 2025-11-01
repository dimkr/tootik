package migrations

import (
	"context"
	"database/sql"
)

func didhost(ctx context.Context, domain string, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `DELETE FROM servers WHERE host LIKE 'did:key:%'`)
	return err
}
