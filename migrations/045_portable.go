package migrations

import (
	"context"
	"crypto/ed25519"
	"database/sql"

	"github.com/dimkr/tootik/data"
)

func portable(ctx context.Context, domain string, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `CREATE TABLE npersons(id TEXT NOT NULL PRIMARY KEY, cid TEXT NOT NULL, actor JSONB NOT NULL, inserted INTEGER DEFAULT (UNIXEPOCH()), updated INTEGER DEFAULT (UNIXEPOCH()), rsaprivkey TEXT, host TEXT AS (substr(substr(id, 9), 0, instr(substr(id, 9), '/'))), fetched INTEGER, ttl INTEGER, ed25519privkey TEXT)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO npersons(id, cid, actor, inserted, updated, fetched, ttl) SELECT id, id, actor, inserted, updated, fetched, ttl FROM persons WHERE ed25519privkey IS NULL`); err != nil {
		return err
	}

	if rows, err := tx.QueryContext(ctx, `SELECT id, json(actor), inserted, updated, rsaprivkey, ed25519privkey FROM persons WHERE ed25519privkey IS NOT NULL`); err != nil {
		return err
	} else {
		defer rows.Close()

		for rows.Next() {
			var id, actorString, rsaPrivKeyPem, ed25519PrivKeyPem string
			var inserted, updated sql.NullInt64
			if err := rows.Scan(&id, &actorString, &inserted, &updated, &rsaPrivKeyPem, &ed25519PrivKeyPem); err != nil {
				return err
			}

			ed25519PrivKey, err := data.ParseRSAPrivateKey(ed25519PrivKeyPem)
			if err != nil {
				return err
			}

			ed2551PrivKeyMultibase := data.EncodeEd25519PrivateKey(ed25519PrivKey.(ed25519.PrivateKey))

			if _, err := tx.ExecContext(ctx, `INSERT INTO npersons(id, cid, actor, inserted, updated, rsaprivkey, ed25519privkey) VALUES($1, $1, $2, $3, $4, $5, $6)`, id, actorString, inserted, updated, rsaPrivKeyPem, ed2551PrivKeyMultibase); err != nil {
				return err
			}
		}

		if err := rows.Err(); err != nil {
			return err
		}
	}

	if _, err := tx.ExecContext(ctx, `DROP TABLE persons`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE npersons RENAME TO persons`); err != nil {
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

	if _, err := tx.ExecContext(ctx, `CREATE INDEX personscid ON persons(cid)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX personspreferreduserlocal ON persons(actor->>'$.preferredUsername') WHERE ed25519privkey IS NOT NULL`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX personsidlocal ON persons(id) WHERE ed25519privkey IS NOT NULL`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX personscidlocal ON persons(cid) WHERE ed25519privkey IS NOT NULL AND cid IS NOT NULL`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE TABLE noutbox(cid TEXT NOT NULL, activity JSONB NOT NULL, inserted INTEGER DEFAULT (UNIXEPOCH()), attempts INTEGER DEFAULT 0, last INTEGER DEFAULT (UNIXEPOCH()), sent INTEGER DEFAULT 0, sender TEXT, host TEXT AS (substr(substr(activity->>'$.id', 9), 0, instr(substr(activity->>'$.id', 9), '/'))))`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO noutbox(cid, activity, inserted, attempts, last, sent, sender) SELECT activity->>'$.id', activity, inserted, attempts, last, sent, sender FROM outbox`); err != nil {
		return err

	}
	if _, err := tx.ExecContext(ctx, `DROP TABLE outbox`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE noutbox RENAME TO outbox`); err != nil {
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

	if _, err := tx.ExecContext(ctx, `CREATE UNIQUE INDEX outboxcidsender ON outbox(cid, sender)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE TABLE nfollows(id TEXT NOT NULL, follower TEXT NOT NULL, inserted INTEGER DEFAULT (UNIXEPOCH()), accepted INTEGER, followed TEXT NOT NULL, followedcid TEXT NOT NULL)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO nfollows(id, follower, inserted, accepted, followed, followedcid) SELECT id, follower, inserted, accepted, followed, followed FROM follows`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DROP TABLE follows`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE nfollows RENAME TO follows`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX followsfollower ON follows(follower)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX followsfollowed ON follows(followed)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE UNIQUE INDEX followsfollowerfollowed ON follows(follower, followed)`); err != nil {
		return err
	}
	return nil
}
