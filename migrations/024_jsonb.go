package migrations

import (
	"context"
	"database/sql"
)

func jsonb(ctx context.Context, domain string, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `ALTER TABLE notes ADD COLUMN objectb JSONB NOT NULL`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `UPDATE notes SET objectb = object`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE notes DROP COLUMN to0`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE notes DROP COLUMN to1`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE notes DROP COLUMN to2`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE notes DROP COLUMN cc0`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE notes DROP COLUMN cc1`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE notes DROP COLUMN cc2`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DROP INDEX noteshostinserted`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE notes DROP COLUMN host`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DROP INDEX notesidinreplytoauthorinserted`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DROP INDEX notesaudience`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE notes DROP COLUMN object`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE notes RENAME COLUMN objectb TO object`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE notes ADD COLUMN to0 STRING AS (object->>'to[0]')`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE notes ADD COLUMN to1 STRING AS (object->>'to[1]')`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE notes ADD COLUMN to2 STRING AS (object->>'to[2]')`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE notes ADD COLUMN cc0 STRING AS (object->>'cc[0]')`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE notes ADD COLUMN cc1 STRING AS (object->>'cc[1]')`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE notes ADD COLUMN cc2 STRING AS (object->>'cc[2]')`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE notes ADD COLUMN host TEXT AS (substr(substr(author, 9), 0, instr(substr(author, 9), '/')))`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX noteshostinserted on notes(host, inserted)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX notesidinreplytoauthorinserted ON notes(id, object->>'inReplyTo', author, inserted) WHERE object->>'inReplyTo' IS NOT NULL`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX notesaudience ON notes(object->>'audience') WHERE object->>'audience' IS NOT NULL`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE persons ADD COLUMN actorb JSONB NOT NULL`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `UPDATE persons SET actorb = actor`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DROP INDEX personstypeid`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DROP INDEX personsmovedto`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DROP INDEX personspreferredusernamehost`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE persons DROP COLUMN actor`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE persons RENAME COLUMN actorb TO actor`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX personstypeid ON persons(actor->>'type', id)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX personsmovedto ON persons(actor->>'movedTo') WHERE actor->>'movedTo' IS NOT NULL`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE UNIQUE INDEX personspreferredusernamehost ON persons(actor->>'preferredUsername', host)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE follows ADD COLUMN followeds STRING NOT NULL`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `UPDATE follows SET followeds = followed`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DROP INDEX followsfollowed`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE follows DROP COLUMN followed`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE follows RENAME COLUMN followeds TO followed`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX followsfollowed ON follows(followed)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE outbox ADD COLUMN activityb JSONB NOT NULL`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `UPDATE outbox SET activityb = activity`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DROP INDEX outboxhostinserted`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE outbox DROP COLUMN host`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DROP INDEX outboxactivityid`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DROP INDEX outboxactor`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DROP INDEX outboxobjectid`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE outbox DROP COLUMN activity`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE outbox RENAME COLUMN activityb TO activity`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE UNIQUE INDEX outboxactivityid ON outbox(activity->>'id')`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE outbox ADD COLUMN host TEXT AS (substr(substr(activity->>'id', 9), 0, instr(substr(activity->>'id', 9), '/')))`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX outboxhostinserted ON outbox(host, inserted)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX outboxactor ON outbox(activity->>'actor')`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX outboxobjectid ON outbox(activity->>'object.id') WHERE activity->>'object.id' IS NOT NULL`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE inbox ADD COLUMN activityb JSONB NOT NULL`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `UPDATE inbox SET activityb = activity`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DROP INDEX inboxid`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE inbox DROP COLUMN activity`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE inbox RENAME COLUMN activityb TO activity`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE UNIQUE INDEX inboxid ON inbox(activity->>'id')`); err != nil {
		return err
	}

	return nil
}
