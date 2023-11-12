package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/dimkr/tootik/cfg"
)

func sharedinbox(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `UPDATE persons SET actor = json_set(actor, '$.endpoints.sharedInbox', $1) WHERE host = $2`, fmt.Sprintf("https://%s/inbox/nobody", cfg.Domain), cfg.Domain)
	return err
}
