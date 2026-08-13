package migrations

import (
	"context"
	"database/sql"
	"strings"

	"github.com/cloudflare/circl/sign/mldsa/mldsa44"
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/data"
)

func insertSlugs(ctx context.Context, tx *sql.Tx, query string) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM slugs`); err != nil {
		return err
	}

	rows, err := tx.QueryContext(ctx, query)
	if err != nil {
		return err
	}

	var ids []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			rows.Close()
			return err
		}

		ids = append(ids, id)
	}
	rows.Close()

	if err := rows.Err(); err != nil {
		return err
	}

	for _, id := range ids {
		if _, err := tx.ExecContext(ctx, `INSERT INTO slugs(id, slug) VALUES(?,?)`, id, ap.Slug(id)); err != nil {
			return err
		}
	}

	return nil
}

func mldsa44slug(ctx context.Context, domain string, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `CREATE TEMP TABLE slugs(id TEXT NOT NULL PRIMARY KEY, slug TEXT NOT NULL)`); err != nil {
		return err
	}

	if err := insertSlugs(ctx, tx, `select id from notes`); err != nil {
		return err
	}

	for _, stmt := range []string{
		`DROP TRIGGER nshares_insert`,
		`DROP TRIGGER nshares_delete`,

		`CREATE TABLE nnotes(slug TEXT NOT NULL PRIMARY KEY, id TEXT NOT NULL UNIQUE, author TEXT NOT NULL, object JSONB NOT NULL, public INTEGER NOT NULL, inserted INTEGER DEFAULT (UNIXEPOCH()), updated INTEGER DEFAULT 0, host TEXT AS (substr(substr(author, 9), 0, instr(substr(author, 9), '/'))), to0 TEXT AS (object->>'$.to[0]'), to1 TEXT AS (object->>'$.to[1]'), to2 TEXT AS (object->>'$.to[2]'), cc0 TEXT AS (object->>'$.cc[0]'), cc1 TEXT AS (object->>'$.cc[1]'), cc2 TEXT AS (object->>'$.cc[2]'), deleted INTEGER NOT NULL DEFAULT 0, nreplies INTEGER DEFAULT 0, nquotes INTEGER DEFAULT 0, nshares INTEGER DEFAULT 0, pulse INTEGER DEFAULT 0, cid TEXT NOT NULL UNIQUE AS (CASE WHEN id LIKE 'https://%' AND (id LIKE '%/.well-known/apgateway/did:key:z6Mk%' OR id LIKE '%/.well-known/apgateway/did:key:ukC%') THEN 'ap://' || SUBSTR(id, 9 + INSTR(SUBSTR(id, 9), '/') + 22, CASE WHEN INSTR(SUBSTR(id, 9 + INSTR(SUBSTR(id, 9), '/') + 22), '?') > 0 THEN INSTR(SUBSTR(id, 9 + INSTR(SUBSTR(id, 9), '/') + 22), '?') - 1 ELSE LENGTH(id) END) WHEN id LIKE 'https://%' THEN id ELSE NULL END))`,
		`INSERT INTO nnotes(slug, id, author, object, public, inserted, updated, deleted, nreplies, nquotes, nshares, pulse) SELECT slugs.slug, notes.id, author, object, public, inserted, updated, deleted, nreplies, nquotes, nshares, pulse FROM notes JOIN slugs ON slugs.id = notes.id`,
		`CREATE VIRTUAL TABLE nnotesfts USING fts5(slug UNINDEXED, content, tokenize = "unicode61 tokenchars '#@'")`,
		`INSERT INTO nnotesfts(slug, content) SELECT slugs.slug, notesfts.content FROM notesfts JOIN notes ON notes.rowid = notesfts.rowid JOIN slugs ON slugs.id = notes.id`,
		`DROP TABLE notesfts`,
		`ALTER TABLE nnotesfts RENAME TO notesfts`,
		`DROP TABLE notes`,
		`ALTER TABLE nnotes RENAME TO notes`,

		`CREATE INDEX notesinserted ON notes(inserted)`,
		`CREATE INDEX notespublicauthor ON notes(public, author)`,
		`CREATE INDEX noteshostinserted on notes(host, inserted)`,
		`CREATE INDEX notesaudience ON notes(object->>'$.audience')`,
		`CREATE INDEX notesquote ON notes(object->>'$.quote') WHERE object->>'$.quote' IS NOT NULL`,
		`CREATE INDEX localnotescontext ON notes(object->>'$.context') WHERE object->>'$.context' IS NOT NULL`,
		`CREATE INDEX notesopenpolls ON notes(id) WHERE object->>'$.type' = 'Question' AND deleted = 0 AND object->>'$.closed' IS NULL`,
		`CREATE TRIGGER nreplies_insert AFTER INSERT ON notes
		WHEN NEW.object->>'$.inReplyTo' IS NOT NULL
		BEGIN
			UPDATE notes
			SET nreplies = nreplies + 1
			WHERE id = NEW.object->>'$.inReplyTo';

			UPDATE notes
			SET pulse = MAX(pulse, NEW.inserted)
			WHERE id IN (
				WITH RECURSIVE thread(id, depth) AS (
					SELECT NEW.object->>'$.inReplyTo', 1
					UNION ALL
					SELECT n.object->>'$.inReplyTo', t.depth + 1
					FROM notes n
					JOIN thread t ON n.id = t.id
					WHERE n.object->>'$.inReplyTo' IS NOT NULL AND t.depth <= 5
				)
				SELECT id FROM thread WHERE id IS NOT NULL
			);
		END`,
		`CREATE TRIGGER nreplies_delete AFTER DELETE ON notes
		WHEN OLD.object->>'$.inReplyTo' IS NOT NULL
		BEGIN
			UPDATE notes
			SET nreplies = MAX(0, nreplies - 1)
			WHERE id = OLD.object->>'$.inReplyTo';
		END`,
		`CREATE TRIGGER nquotes_insert AFTER INSERT ON notes
		WHEN NEW.object->>'$.quote' IS NOT NULL
		BEGIN
			UPDATE notes
			SET nquotes = nquotes + 1, pulse = MAX(pulse, NEW.inserted)
			WHERE id = NEW.object->>'$.quote';
		END`,
		`CREATE TRIGGER nquotes_delete AFTER DELETE ON notes
		WHEN OLD.object->>'$.quote' IS NOT NULL
		BEGIN
			UPDATE notes
			SET nquotes = MAX(0, nquotes - 1)
			WHERE id = OLD.object->>'$.quote';
		END`,
		`CREATE TRIGGER notes_insert AFTER INSERT ON notes
		BEGIN
			UPDATE notes SET
				nreplies = (SELECT COUNT(*) FROM notes WHERE object->>'$.inReplyTo' = NEW.id),
				nquotes = (SELECT COUNT(*) FROM notes WHERE object->>'$.quote' = NEW.id),
				nshares = (SELECT COUNT(*) FROM shares WHERE note = NEW.id AND shares.by IS NOT NEW.object->>'$.audience'),
				pulse = COALESCE(
					(SELECT MAX(v) FROM (
						SELECT MAX(replies.inserted) as v FROM notes replies WHERE replies.object->>'$.inReplyTo' = NEW.id
						UNION ALL
						SELECT MAX(quotes.inserted) as v FROM notes quotes WHERE quotes.object->>'$.quote' = NEW.id
					)),
					NEW.inserted
				)
			WHERE id = NEW.id;
		END`,
		`CREATE TRIGGER nshares_insert AFTER INSERT ON shares
		BEGIN
			UPDATE notes
			SET nshares = nshares + 1
			WHERE id = NEW.note AND NEW.by IS NOT object->>'$.audience';
		END`,
		`CREATE TRIGGER nshares_delete AFTER DELETE ON shares
		BEGIN
			UPDATE notes
			SET nshares = MAX(0, nshares - 1)
			WHERE id = OLD.note AND OLD.by IS NOT object->>'$.audience';
		END`,
		`CREATE INDEX notesinreplytoinserted ON notes(object->>'$.inReplyTo', inserted) WHERE object->>'$.inReplyTo' IS NOT NULL`,
		`CREATE INDEX notesauthorinserted ON notes(author, inserted)`,
		`CREATE TRIGGER noteshashtagsinserted AFTER INSERT ON notes
		BEGIN
			INSERT INTO hashtags (note, hashtag)
			SELECT DISTINCT new.id, CASE WHEN SUBSTR(value->>'$.name', 1, 1) = '#' THEN SUBSTR(value->>'$.name', 2) ELSE value->>'$.name' END COLLATE NOCASE
			FROM JSON_EACH(new.object->'$.tag')
			WHERE new.deleted = 0 AND value->>'$.type' = 'Hashtag' AND value->>'$.name' IS NOT NULL AND value->>'$.name' != '';
		END`,
		`CREATE TRIGGER noteshashtagsupdated AFTER UPDATE ON notes
		BEGIN
			DELETE FROM hashtags WHERE note = new.id AND hashtag NOT IN (
				SELECT CASE WHEN SUBSTR(value->>'$.name', 1, 1) = '#' THEN SUBSTR(value->>'$.name', 2) ELSE value->>'$.name' END COLLATE NOCASE
				FROM JSON_EACH(new.object->'$.tag')
				WHERE new.deleted = 0 AND value->>'$.type' = 'Hashtag' AND value->>'$.name' IS NOT NULL AND value->>'$.name' != ''
			);

			INSERT INTO hashtags (note, hashtag)
			SELECT candidates.note, candidates.hashtag FROM (
				SELECT DISTINCT new.id AS note, CASE WHEN SUBSTR(value->>'$.name', 1, 1) = '#' THEN SUBSTR(value->>'$.name', 2) ELSE value->>'$.name' END COLLATE NOCASE AS hashtag
				FROM JSON_EACH(new.object->'$.tag')
				WHERE new.deleted = 0 AND value->>'$.type' = 'Hashtag' AND value->>'$.name' IS NOT NULL AND value->>'$.name' != ''
			) candidates
			WHERE candidates.hashtag NOT IN (SELECT hashtag COLLATE NOCASE FROM hashtags WHERE hashtags.note = candidates.note);
		END`,
		`CREATE TRIGGER noteshashtagsdeleted AFTER DELETE ON notes
		BEGIN
			DELETE FROM hashtags WHERE note = old.id;
		END`,
	} {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}

	if err := insertSlugs(ctx, tx, `select id from persons`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE TABLE npersons(slug TEXT NOT NULL PRIMARY KEY, id TEXT NOT NULL UNIQUE, actor JSONB NOT NULL, inserted INTEGER DEFAULT (UNIXEPOCH()), updated INTEGER DEFAULT (UNIXEPOCH()), host TEXT AS (substr(substr(id, 9), 0, instr(substr(id, 9), '/'))), fetched INTEGER, ttl INTEGER, rsaprivkey BLOB, ed25519seed BLOB, mldsa44seed BLOB, cid TEXT NOT NULL AS (CASE WHEN id LIKE 'https://%' AND (id LIKE '%/.well-known/apgateway/did:key:z6Mk%' OR id LIKE '%/.well-known/apgateway/did:key:ukC%') THEN 'ap://' || SUBSTR(id, 9 + INSTR(SUBSTR(id, 9), '/') + 22, CASE WHEN INSTR(SUBSTR(id, 9 + INSTR(SUBSTR(id, 9), '/') + 22), '?') > 0 THEN INSTR(SUBSTR(id, 9 + INSTR(SUBSTR(id, 9), '/') + 22), '?') - 1 ELSE LENGTH(id) END) WHEN id LIKE 'https://%' THEN id ELSE NULL END))`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO npersons(slug, id, actor, inserted, updated, fetched, ttl, rsaprivkey, ed25519seed) SELECT slugs.slug, persons.id, actor, inserted, updated, fetched, ttl, rsaprivkey, ed25519privkey FROM persons JOIN slugs ON slugs.id = persons.id`); err != nil {
		return err
	}

	rows, err := tx.QueryContext(ctx, `SELECT id, JSON(actor) FROM npersons WHERE ed25519seed IS NOT NULL`, domain)
	if err != nil {
		return err
	}

	defer rows.Close()

	for rows.Next() {
		var id string
		var actor ap.Actor
		if err := rows.Scan(&id, &actor); err != nil {
			return err
		}

		if len(actor.AssertionMethod) == 0 {
			continue
		}

		last := actor.AssertionMethod[len(actor.AssertionMethod)-1]

		prefix, ok := strings.CutSuffix(last.ID, "#ed25519-key")
		if !ok {
			continue
		}

		mldsa44Pub, mldsa44Priv, err := mldsa44.GenerateKey(nil)
		if err != nil {
			return err
		}

		actor.AssertionMethod = append(actor.AssertionMethod, ap.AssertionMethod{
			ID:                 prefix + "#ml-dsa-44-key",
			Type:               "Multikey",
			Controller:         last.Controller,
			PublicKeyMultibase: data.EncodeMLDSA44Publickey(mldsa44Pub),
		})

		if _, err := tx.ExecContext(ctx, `UPDATE npersons SET actor = JSONB(?), mldsa44seed = ? WHERE id = ?`, &actor, mldsa44Priv.Seed(), id); err != nil {
			return err
		}
	}

	if err := rows.Err(); err != nil {
		return err
	}

	for _, stmt := range []string{
		`DROP TABLE persons`,
		`ALTER TABLE npersons RENAME TO persons`,
		`CREATE INDEX personstypeid ON persons(actor->>'$.type', id)`,
		`CREATE INDEX personsmovedto ON persons(actor->>'$.movedTo') WHERE actor->>'$.movedTo' IS NOT NULL`,
		`CREATE UNIQUE INDEX personspreferredusernamehosttype ON persons(actor->>'$.preferredUsername', host, actor->>'$.type')`,
		`CREATE INDEX personscid ON persons(cid)`,
		`CREATE UNIQUE INDEX personscidlocal ON persons(cid) WHERE ed25519seed IS NOT NULL`,
		`DROP INDEX outboxcidsender`,
		`ALTER TABLE outbox DROP COLUMN cid`,
		`ALTER TABLE outbox ADD COLUMN cid TEXT NOT NULL AS (CASE WHEN activity->>'$.id' LIKE 'https://%' AND (activity->>'$.id' LIKE '%/.well-known/apgateway/did:key:z6Mk%' OR activity->>'$.id' LIKE '%/.well-known/apgateway/did:key:ukC%') THEN 'ap://' || SUBSTR(activity->>'$.id', 9 + INSTR(SUBSTR(activity->>'$.id', 9), '/') + 22, CASE WHEN INSTR(SUBSTR(activity->>'$.id', 9 + INSTR(SUBSTR(activity->>'$.id', 9), '/') + 22), '?') > 0 THEN INSTR(SUBSTR(activity->>'$.id', 9 + INSTR(SUBSTR(activity->>'$.id', 9), '/') + 22), '?') - 1 ELSE LENGTH(activity->>'$.id') END) WHEN activity->>'$.id' LIKE 'https://%' THEN activity->>'$.id' ELSE NULL END)`,
		`CREATE INDEX outboxcidsender ON outbox(cid, sender)`,
	} {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}

	_, err = tx.ExecContext(ctx, `DROP TABLE slugs`)
	return err
}
