package migrations

import (
	"context"
	"database/sql"
)

func outboxnano(ctx context.Context, domain string, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `CREATE TABLE noutbox(activity JSONB NOT NULL, inserted INTEGER NOT NULL, attempts INTEGER DEFAULT 0, last INTEGER DEFAULT (UNIXEPOCH()), sent INTEGER DEFAULT 0, sender TEXT NOT NULL, host TEXT AS (substr(substr(activity->>'$.id', 9), 0, instr(substr(activity->>'$.id', 9), '/'))), cid TEXT NOT NULL AS (CASE WHEN activity->>'$.id' LIKE 'https://%' AND activity->>'$.id' LIKE '%/.well-known/apgateway/did:key:z6Mk%' THEN 'ap://' || SUBSTR(activity->>'$.id', 9 + INSTR(SUBSTR(activity->>'$.id', 9), '/') + 22, CASE WHEN INSTR(SUBSTR(activity->>'$.id', 9 + INSTR(SUBSTR(activity->>'$.id', 9), '/') + 22), '?') > 0 THEN INSTR(SUBSTR(activity->>'$.id', 9 + INSTR(SUBSTR(activity->>'$.id', 9), '/') + 22), '?') - 1 ELSE LENGTH(activity->>'$.id') END) WHEN activity->>'$.id' LIKE 'https://%' THEN activity->>'$.id' ELSE NULL END))`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO noutbox(activity, inserted, attempts, last, sent, sender) SELECT activity, inserted*1000000000, attempts, last, sent, sender FROM outbox`); err != nil {
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

	if _, err := tx.ExecContext(ctx, `CREATE UNIQUE INDEX outboxactivityidsender ON outbox(activity->>'$.id', sender)`); err != nil {
		return err
	}

	return nil
}
