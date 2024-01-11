package migrations

import (
	"context"
	"database/sql"
)

func notesfts(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `CREATE VIRTUAL TABLE notesfts USING fts5(id UNINDEXED, content, tokenize = "unicode61 tokenchars '#@'")`)
	return err
}
