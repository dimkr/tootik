package migrations

import (
	"context"
	"database/sql"
	"fmt"
)

func outbox(ctx context.Context, domain string, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `CREATE TABLE outbox(activity STRING NOT NULL, inserted INTEGER DEFAULT (UNIXEPOCH()), attempts INTEGER DEFAULT 0, last INTEGER DEFAULT (UNIXEPOCH()), sent INTEGER DEFAULT 0)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE UNIQUE INDEX outboxactivityid ON outbox(activity->>'id')`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX outboxsentattempts ON outbox(sent, attempts)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE follows ADD accepted INTEGER DEFAULT 0`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `UPDATE follows SET accepted = 1`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE activities RENAME TO inbox`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE UNIQUE INDEX inboxid ON inbox(activity->>'id')`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DROP INDEX activitiesid`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE persons ADD certhash STRING`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE UNIQUE INDEX personscerthash ON persons(certhash)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `UPDATE persons SET certhash = actor->>'clientCertificate'`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `UPDATE persons SET actor = json_remove(actor, '$.clientCertificate')`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE persons ADD privkey STRING`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `UPDATE persons SET privkey = actor->>'privateKey'`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `UPDATE persons SET actor = json_remove(actor, '$.privateKey')`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `UPDATE notes SET object = json_remove(object, '$.url') where id like ?`, fmt.Sprintf("https://%s/%%", domain)); err != nil {
		return err
	}

	return nil
}
