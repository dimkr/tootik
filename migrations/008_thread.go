package migrations

import (
	"context"
	"database/sql"
)

func thread(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `CREATE INDEX notesidinreplytoauthorinserted ON notes(id, object->>'inReplyTo', author, inserted)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DROP INDEX notesinreplyto`); err != nil {
		return err
	}

	return nil
}
