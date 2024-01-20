package migrations

import (
	"context"
	"database/sql"
)

func shares(ctx context.Context, domain string, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `DROP INDEX notesidinreplytoauthorinserted`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX notesidinreplytoauthorinserted ON notes(id, object->>'inReplyTo', author, inserted) WHERE object->>'inReplyTo' IS NOT NULL`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE TABLE shares(note STRING NOT NULL, by STRING NOT NULL, inserted INTEGER DEFAULT (UNIXEPOCH()))`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX sharesnote ON shares(note)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX sharesbyinserted ON shares(by, inserted)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DROP INDEX notesgroupid`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE notes DROP COLUMN groupid`); err != nil {
		return err
	}

	_, err := tx.ExecContext(ctx, `CREATE INDEX notesaudience ON notes(object->>'audience')`)
	return err
}
