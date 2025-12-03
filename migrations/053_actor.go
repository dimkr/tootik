package migrations

import (
	"context"
	"database/sql"
)

func actor(ctx context.Context, domain string, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `UPDATE persons SET actor = JSONB_SET(actor, '$.endpoints.sharedInbox', 'https://' || $1 || '/inbox/actor') WHERE host = $1 AND actor->>'$.endpoints.sharedInbox' = 'https://' || $1 || '/inbox/nobody'`, domain)
	return err
}
