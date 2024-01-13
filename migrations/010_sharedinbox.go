package migrations

import (
	"context"
	"database/sql"
	"fmt"
)

func sharedinbox(ctx context.Context, domain string, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `UPDATE persons SET actor = json_set(actor, '$.endpoints.sharedInbox', $1) WHERE host = $2`, fmt.Sprintf("https://%s/inbox/nobody", domain), domain)
	return err
}
