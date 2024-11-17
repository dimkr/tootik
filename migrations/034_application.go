package migrations

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

func application(ctx context.Context, domain string, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `update persons set actor = json_set(actor, '$.type', 'Application', '$.updated', ?) where id = ?`, time.Now().Format(time.RFC3339Nano), fmt.Sprintf("https://%s/user/nobody", domain))
	return err
}
