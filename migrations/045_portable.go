package migrations

import (
	"context"
	"crypto/ed25519"
	"database/sql"

	"github.com/dimkr/tootik/data"
)

func portable(ctx context.Context, domain string, tx *sql.Tx) error {
	if rows, err := tx.QueryContext(ctx, `SELECT id, ed25519privkey FROM persons WHERE ed25519privkey IS NOT NULL`); err != nil {
		return err
	} else {
		defer rows.Close()

		for rows.Next() {
			var id, ed25519PrivKeyPem string
			if err := rows.Scan(&id, &ed25519PrivKeyPem); err != nil {
				return err
			}

			ed25519PrivKey, err := data.ParseRSAPrivateKey(ed25519PrivKeyPem)
			if err != nil {
				return err
			}

			ed2551PrivKeyMultibase := data.EncodeEd25519PrivateKey(ed25519PrivKey.(ed25519.PrivateKey))

			if _, err := tx.ExecContext(ctx, `UPDATE persons SET ed25519privkey = ? WHERE id = ?`, ed2551PrivKeyMultibase, id); err != nil {
				return err
			}
		}

		if err := rows.Err(); err != nil {
			return err
		}
	}

	if _, err := tx.ExecContext(ctx, `CREATE TABLE npersons(id TEXT NOT NULL AS (CASE WHEN actor->>'$.id' LIKE '%/.well-known/apgateway/did:key:z6Mk%' THEN 'ap://' || SUBSTR(actor->>'$.id', 9 + INSTR(SUBSTR(actor->>'$.id', 9), '/') + 22, CASE WHEN INSTR(SUBSTR(actor->>'$.id', 9 + INSTR(SUBSTR(actor->>'$.id', 9), '/') + 22), '?') > 0 THEN INSTR(SUBSTR(actor->>'$.id', 9 + INSTR(SUBSTR(actor->>'$.id', 9), '/') + 22), '?') - 1 ELSE LENGTH(actor->>'$.id') END) ELSE actor->>'$.id' END), actor JSONB NOT NULL, inserted INTEGER DEFAULT (UNIXEPOCH()), updated INTEGER DEFAULT (UNIXEPOCH()), rsaprivkey STRING, host TEXT AS (substr(substr(actor->>'$.id', 9), 0, instr(substr(actor->>'$.id', 9), '/'))), fetched INTEGER, ttl INTEGER, ed25519privkey TEXT)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO npersons(actor, inserted, updated, rsaprivkey, fetched, ttl, ed25519privkey) SELECT actor, inserted, updated, rsaprivkey, fetched, ttl, ed25519privkey FROM persons`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DROP TABLE persons`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE npersons RENAME TO persons`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE UNIQUE INDEX personsid ON persons(id)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX personstypeid ON persons(actor->>'$.type', id)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX personsmovedto ON persons(actor->>'$.movedTo') WHERE actor->>'$.movedTo' IS NOT NULL`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE UNIQUE INDEX personspreferredusernamehosttype ON persons(actor->>'$.preferredUsername', host, actor->>'$.type')`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE UNIQUE INDEX personsed25519publickeyid ON persons(actor->>'$.assertionMethod[0].id') WHERE actor->>'$.assertionMethod[0].id' IS NOT NULL`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX personsidlocal ON persons(id) WHERE ed25519privkey IS NOT NULL`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE outbox ADD COLUMN id TEXT NOT NULL AS (CASE WHEN activity->>'$.id' LIKE '%/.well-known/apgateway/did:key:z6Mk%' THEN 'ap://' || SUBSTR(activity->>'$.id', 9 + INSTR(SUBSTR(activity->>'$.id', 9), '/') + 22, CASE WHEN INSTR(SUBSTR(activity->>'$.id', 9 + INSTR(SUBSTR(activity->>'$.id', 9), '/') + 22), '?') > 0 THEN INSTR(SUBSTR(activity->>'$.id', 9 + INSTR(SUBSTR(activity->>'$.id', 9), '/') + 22), '?') - 1 ELSE LENGTH(activity->>'$.id') END) ELSE activity->>'$.id' END)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE UNIQUE INDEX outboxid ON outbox(id)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE UNIQUE INDEX outboxidsender ON outbox(id, sender)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE TABLE nnotes(id TEXT NOT NULL AS (CASE WHEN object->>'$.id' LIKE '%/.well-known/apgateway/did:key:z6Mk%' THEN 'ap://' || SUBSTR(object->>'$.id', 9 + INSTR(SUBSTR(object->>'$.id', 9), '/') + 22, CASE WHEN INSTR(SUBSTR(object->>'$.id', 9 + INSTR(SUBSTR(object->>'$.id', 9), '/') + 22), '?') > 0 THEN INSTR(SUBSTR(object->>'$.id', 9 + INSTR(SUBSTR(object->>'$.id', 9), '/') + 22), '?') - 1 ELSE LENGTH(object->>'$.id') END) ELSE object->>'$.id' END), author TEXT NOT NULL AS (CASE WHEN object->>'$.attributedTo' LIKE '%/.well-known/apgateway/did:key:z6Mk%' THEN 'ap://' || SUBSTR(object->>'$.attributedTo', 9 + INSTR(SUBSTR(object->>'$.attributedTo', 9), '/') + 22, CASE WHEN INSTR(SUBSTR(object->>'$.attributedTo', 9 + INSTR(SUBSTR(object->>'$.attributedTo', 9), '/') + 22), '?') > 0 THEN INSTR(SUBSTR(object->>'$.attributedTo', 9 + INSTR(SUBSTR(object->>'$.attributedTo', 9), '/') + 22), '?') - 1 ELSE LENGTH(object->>'$.attributedTo') END) ELSE object->>'$.attributedTo' END), object JSONB NOT NULL, public INTEGER NOT NULL, inserted INTEGER DEFAULT (UNIXEPOCH()), updated INTEGER DEFAULT 0, host TEXT AS (substr(substr(object->>'$.id', 9), 0, instr(substr(object->>'$.id', 9), '/'))), to0 TEXT AS (object->>'$.to[0]'), to1 TEXT AS (object->>'$.to[1]'), to2 TEXT AS (object->>'$.to[2]'), cc0 TEXT AS (object->>'$.cc[0]'), cc1 TEXT AS (object->>'$.cc[1]'), cc2 TEXT AS (object->>'$.cc[2]'), parent TEXT AS (CASE WHEN object->>'$.inReplyTo' LIKE '%/.well-known/apgateway/did:key:z6Mk%' THEN 'ap://' || SUBSTR(object->>'$.inReplyTo', 9 + INSTR(SUBSTR(object->>'$.inReplyTo', 9), '/') + 22, CASE WHEN INSTR(SUBSTR(object->>'$.inReplyTo', 9 + INSTR(SUBSTR(object->>'$.inReplyTo', 9), '/') + 22), '?') > 0 THEN INSTR(SUBSTR(object->>'$.inReplyTo', 9 + INSTR(SUBSTR(object->>'$.inReplyTo', 9), '/') + 22), '?') - 1 ELSE LENGTH(object->>'$.inReplyTo') END) ELSE object->>'$.inReplyTo' END))`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO nnotes(object, public, inserted, updated) SELECT object, public, inserted, updated FROM notes`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DROP TABLE notes`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE nnotes RENAME TO notes`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE UNIQUE INDEX notesid ON notes(id)`); err != nil {
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

	if _, err := tx.ExecContext(ctx, `CREATE INDEX notesidattributedToauthorinserted ON notes(id, object->>'$.attributedTo', author, inserted) WHERE object->>'$.attributedTo' IS NOT NULL`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX notesaudience ON notes(object->>'$.audience')`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX notesquote ON notes(object->>'$.quote') WHERE object->>'$.quote' IS NOT NULL`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX notesidparentauthorinserted ON notes(id, parent, author, inserted) WHERE parent IS NOT NULL`); err != nil {
		return err
	}

	return nil
}
