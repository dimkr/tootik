package migrations

import (
	"context"
	"database/sql"
)

func nomadic(ctx context.Context, domain string, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `DROP INDEX personspreferredusernamehosttype`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE persons DROP COLUMN host`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE persons ADD COLUMN host TEXT AS (CASE WHEN id LIKE 'ap://%' THEN substr(substr(id, 6), 0, instr(substr(id, 6), '/')) ELSE substr(substr(id, 9), 0, instr(substr(id, 9), '/')) END)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE UNIQUE INDEX personspreferredusernamehosttype ON persons(actor->>'$.preferredUsername', host, actor->>'$.type')`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DROP INDEX noteshostinserted`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE notes DROP COLUMN host`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE notes ADD COLUMN host TEXT AS (CASE WHEN id LIKE 'ap://%' THEN substr(substr(id, 6), 0, instr(substr(id, 6), '/')) ELSE substr(substr(id, 9), 0, instr(substr(id, 9), '/')) END)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX noteshostinserted on notes(host, inserted)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE UNIQUE INDEX personslocal ON persons(id) WHERE ed25519privkey IS NOT NULL`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE UNIQUE INDEX personsnomadic ON persons(id, actor->>'$.assertionMethod[0].id') WHERE ed25519privkey IS NOT NULL AND id LIKE 'ap://%'`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE UNIQUE INDEX personslocalusername ON persons(actor->>'$.preferredUsername') WHERE ed25519privkey IS NOT NULL`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS noutbox(id TEXT PRIMARY KEY, activity JSONB NOT NULL, inserted INTEGER DEFAULT (UNIXEPOCH()), attempts INTEGER DEFAULT 0, last INTEGER DEFAULT (UNIXEPOCH()), sent INTEGER DEFAULT 0, sender STRING, host TEXT AS (CASE WHEN id LIKE 'ap://%' THEN substr(substr(id, 6), 0, instr(substr(id, 6), '/')) ELSE substr(substr(id, 9), 0, instr(substr(id, 9), '/')) END))`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO noutbox(id, activity, inserted, attempts, last, sent, sender) SELECT activity->>'$.id', activity, inserted, attempts, last, sent, sender FROM outbox`); err != nil {
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

	if _, err := tx.ExecContext(ctx, `CREATE UNIQUE INDEX outboxidsender ON outbox(id, sender)`); err != nil {
		return err
	}

	return nil
}
