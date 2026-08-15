package migrations

import (
	"context"
	"database/sql"
)

func mldsa44cid(ctx context.Context, domain string, tx *sql.Tx) error {
	for _, stmt := range []string{
		`DROP INDEX notescid`,
		`ALTER TABLE notes DROP COLUMN cid`,
		`ALTER TABLE notes ADD COLUMN cid TEXT NOT NULL AS (CASE WHEN id LIKE 'https://%' AND (id LIKE '%/.well-known/apgateway/did:key:z6Mk%' OR id LIKE '%/.well-known/apgateway/did:key:ukC%') THEN 'ap://' || SUBSTR(id, 9 + INSTR(SUBSTR(id, 9), '/') + 22, CASE WHEN INSTR(SUBSTR(id, 9 + INSTR(SUBSTR(id, 9), '/') + 22), '?') > 0 THEN INSTR(SUBSTR(id, 9 + INSTR(SUBSTR(id, 9), '/') + 22), '?') - 1 ELSE LENGTH(id) END) WHEN id LIKE 'https://%' THEN id ELSE NULL END)`,
		`CREATE UNIQUE INDEX notescid ON notes(cid)`,
		`DROP INDEX personscid`,
		`DROP INDEX personscidlocal`,
		`ALTER TABLE persons DROP COLUMN cid`,
		`ALTER TABLE persons ADD COLUMN cid TEXT NOT NULL AS (CASE WHEN id LIKE 'https://%' AND (id LIKE '%/.well-known/apgateway/did:key:z6Mk%' OR id LIKE '%/.well-known/apgateway/did:key:ukC%') THEN 'ap://' || SUBSTR(id, 9 + INSTR(SUBSTR(id, 9), '/') + 22, CASE WHEN INSTR(SUBSTR(id, 9 + INSTR(SUBSTR(id, 9), '/') + 22), '?') > 0 THEN INSTR(SUBSTR(id, 9 + INSTR(SUBSTR(id, 9), '/') + 22), '?') - 1 ELSE LENGTH(id) END) WHEN id LIKE 'https://%' THEN id ELSE NULL END)`,
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

	return nil
}
