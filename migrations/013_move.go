package migrations

import (
	"context"
	"database/sql"
)

func move(ctx context.Context, tx *sql.Tx) error {
	_, err := tx.ExecContext(ctx, `CREATE INDEX personsmovedto ON persons(actor->>'movedTo') WHERE actor->>'movedTo' IS NOT NULL`)
	return err
}
