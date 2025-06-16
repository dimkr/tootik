package migrations

import (
	"context"
	"database/sql"
)

func jsonb(ctx context.Context, domain string, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `CREATE TABLE newnotes(id STRING NOT NULL PRIMARY KEY, author STRING NOT NULL, object JSONB NOT NULL, public INTEGER NOT NULL, inserted INTEGER DEFAULT (UNIXEPOCH()), updated INTEGER DEFAULT 0, host TEXT AS (substr(substr(author, 9), 0, instr(substr(author, 9), '/'))), to0 STRING AS (object->>'$.to[0]'), to1 STRING AS (object->>'$.to[1]'), to2 STRING AS (object->>'$.to[2]'), cc0 STRING AS (object->>'$.cc[0]'), cc1 STRING AS (object->>'$.cc[1]'), cc2 STRING AS (object->>'$.cc[2]'))`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO newnotes(id, author, object, public, inserted, updated) SELECT id, author, JSONB(object), public, inserted, updated FROM notes`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DROP TABLE notes`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE newnotes RENAME TO notes`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX notesauthor ON notes(author)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX notesinserted ON notes(inserted)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX notespublicauthor ON notes(public, author)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX noteshostinserted on notes(host, inserted)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX notesidinreplytoauthorinserted ON notes(id, object->>'$.inReplyTo', author, inserted) WHERE object->>'$.inReplyTo' IS NOT NULL`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX notesaudience ON notes(object->>'$.audience')`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE TABLE newpersons(id STRING NOT NULL PRIMARY KEY, actor JSONB NOT NULL, inserted INTEGER DEFAULT (UNIXEPOCH()), updated INTEGER DEFAULT (UNIXEPOCH()), privkey STRING, host TEXT AS (substr(substr(id, 9), 0, instr(substr(id, 9), '/'))), fetched INTEGER, ttl INTEGER)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO persons(id, actor, inserted, updated, privkey, fetched, ttl) SELECT id, JSONB(actor), inserted, updated, privkey, fetched, ttl FROM persons`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DROP TABLE persons`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE newpersons RENAME TO persons`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX personstypeid ON persons(actor->>'$.type', id)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX personsmovedto ON persons(actor->>'$.movedTo') WHERE actor->>'$.movedTo' IS NOT NULL`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE UNIQUE INDEX personspublickeyid ON persons(actor->>'$.publicKey.id')`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE UNIQUE INDEX personspreferredusernamehosttype ON persons(actor->>'$.preferredUsername', host, actor->>'$.type')`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE TABLE newoutbox(activity JSONB NOT NULL, inserted INTEGER DEFAULT (UNIXEPOCH()), attempts INTEGER DEFAULT 0, last INTEGER DEFAULT (UNIXEPOCH()), sent INTEGER DEFAULT 0, sender STRING, host TEXT AS (substr(substr(activity->>'$.id', 9), 0, instr(substr(activity->>'$.id', 9), '/'))))`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO newoutbox(activity, inserted, attempts, last, sent, sender) SELECT JSONB(activity), inserted, attempts, last, sent, sender FROM outbox`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DROP TABLE outbox`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE newoutbox RENAME TO outbox`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX outboxsentattempts ON outbox(sent, attempts)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX outboxhostinserted ON outbox(host, inserted)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX outboxactor ON outbox(activity->>'$.actor')`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX outboxobjectid ON outbox(activity->>'$.object.id') WHERE activity->>'$.object.id' IS NOT NULL`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE UNIQUE INDEX outboxactivityidsender ON outbox(activity->>'$.id', sender)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE TABLE newfeed(follower STRING NOT NULL, note JSONB NOT NULL, author JSONB NOT NULL, sharer JSONB, inserted INTEGER NOT NULL)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO feed(follower, note, author, sharer, inserted) SELECT follower, JSONB(note), JSONB(author), JSONB(sharer), inserted FROM feed`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DROP TABLE feed`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE newfeed RENAME TO feed`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX feedfollowerinserted ON feed(follower, inserted)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX feedinserted ON feed(inserted)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX feednote ON feed(note->>'$.id')`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX feedauthorid ON feed(author->>'$.id')`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX feedshareid ON feed(sharer->>'$.id') WHERE sharer->>'$.id' IS NOT NULL`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE TABLE newinbox(id INTEGER PRIMARY KEY, sender STRING NOT NULL, activity JSONB NOT NULL, raw STRING NOT NULL, inserted INTEGER DEFAULT (UNIXEPOCH()))`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO newinbox(id, sender, activity, raw, inserted) SELECT id, sender, JSONB(activity), raw, inserted FROM inbox`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DROP TABLE inbox`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE newinbox RENAME TO inbox`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE UNIQUE INDEX inboxid ON inbox(activity->>'$.id')`); err != nil {
		return err
	}

	return nil
}
