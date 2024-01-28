package migrations

import (
	"context"
	"database/sql"
)

func jsonb(ctx context.Context, domain string, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `CREATE TABLE notesb(id STRING NOT NULL PRIMARY KEY, author STRING NOT NULL, object JSONB NOT NULL, public INTEGER NOT NULL, to0 STRING, to1 STRING, to2 STRING, cc0 STRING, cc1 STRING, cc2 STRING, inserted INTEGER DEFAULT (UNIXEPOCH()), updated INTEGER DEFAULT 0, host TEXT AS (substr(substr(author, 9), 0, instr(substr(author, 9), '/'))));`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO notesb(id, author, object, public, to0, to1, to2, cc0, cc1, cc2, inserted, updated) SELECT id, author, object, public, to0, to1, to2, cc0, cc1, cc2, inserted, updated FROM notes`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DROP TABLE notes`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE notesb RENAME TO notes`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE TABLE personsb(id STRING NOT NULL PRIMARY KEY, actor JSONB NOT NULL, inserted INTEGER DEFAULT (UNIXEPOCH()), updated INTEGER DEFAULT (UNIXEPOCH()), certhash STRING, privkey STRING, host TEXT AS (substr(substr(id, 9), 0, instr(substr(id, 9), '/'))), fetched INTEGER)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO personsb(id, actor, inserted, updated, certhash, privkey, fetched) SELECT id, actor, inserted, updated, certhash, privkey, fetched FROM persons`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DROP TABLE persons`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE personsb RENAME TO persons`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE TABLE follows(id STRING NOT NULL PRIMARY KEY, follower STRING NOT NULL, followed STRING NOT NULL, inserted INTEGER DEFAULT (UNIXEPOCH()), accepted INTEGER DEFAULT 0)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO followsb(id, follower, followed, inserted, accepted) SELECT id, follower, followed, inserted, accepted FROM follows`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DROP TABLE follows`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE followsb RENAME TO follows`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE TABLE outboxb(activity JSONB NOT NULL, inserted INTEGER DEFAULT (UNIXEPOCH()), attempts INTEGER DEFAULT 0, last INTEGER DEFAULT (UNIXEPOCH()), sent INTEGER DEFAULT 0, sender STRING, received TEXT, host TEXT AS (substr(substr(activity->>'id', 9), 0, instr(substr(activity->>'id', 9), '/'))))`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO outboxb(activity, inserted, attempts, last, sent, sender, received) SELECT activity, inserted, attempts, last, sent, sender, received outbox`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DROP TABLE outbox`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE outboxb RENAME TO outbox`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE TABLE inboxb(id INTEGER PRIMARY KEY, sender STRING NOT NULL, activity JSONB NOT NULL, inserted INTEGER DEFAULT (UNIXEPOCH()))`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO inboxb(id, sender, activity, inserted) SELECT id, sender, activity, inserted FROM inbox`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DROP TABLE inbox`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE inboxb RENAME TO inbox`); err != nil {
		return err
	}

	return nil
}
