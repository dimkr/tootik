package migrations

import (
	"context"
	"database/sql"
)

func notesinreplytoinserted(ctx context.Context, domain string, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `DROP INDEX notesidinreplytoauthorinserted`); err != nil {
		return err
	}

	_, err := tx.ExecContext(ctx, `CREATE INDEX notesinreplytoinserted ON notes(object->>'$.inReplyTo', inserted) WHERE object->>'$.inReplyTo' IS NOT NULL`)
	return err
}
