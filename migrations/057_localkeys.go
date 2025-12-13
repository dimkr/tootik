package migrations

import (
	"context"
	"database/sql"
)

func localkeys(ctx context.Context, domain string, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO keys(id, actor) SELECT actor->>'$.publicKey.id', id FROM persons WHERE ed25519privkey IS NOT NULL UNION ALL SELECT actor->>'$.assertionMethod[0].id', id FROM persons WHERE ed25519privkey IS NOT NULL`)
	return err
}
