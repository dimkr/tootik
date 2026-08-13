package migrations

import (
	"context"
	"database/sql"
	"strings"

	"github.com/cloudflare/circl/sign/mldsa/mldsa44"
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/data"
)

func mldsa44seed(ctx context.Context, domain string, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `DROP INDEX notescid`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE notes DROP COLUMN cid`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE notes ADD COLUMN cid TEXT NOT NULL AS (CASE WHEN id LIKE 'https://%' AND (id LIKE '%/.well-known/apgateway/did:key:z6Mk%' OR id LIKE '%/.well-known/apgateway/did:key:ukC%') THEN 'ap://' || SUBSTR(id, 9 + INSTR(SUBSTR(id, 9), '/') + 22, CASE WHEN INSTR(SUBSTR(id, 9 + INSTR(SUBSTR(id, 9), '/') + 22), '?') > 0 THEN INSTR(SUBSTR(id, 9 + INSTR(SUBSTR(id, 9), '/') + 22), '?') - 1 ELSE LENGTH(id) END) WHEN id LIKE 'https://%' THEN id ELSE NULL END)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE UNIQUE INDEX notescid ON notes(cid)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DROP INDEX personscid`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DROP INDEX personscidlocal`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE persons DROP COLUMN cid`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE persons ADD COLUMN cid TEXT NOT NULL AS (CASE WHEN id LIKE 'https://%' AND (id LIKE '%/.well-known/apgateway/did:key:z6Mk%' OR id LIKE '%/.well-known/apgateway/did:key:ukC%') THEN 'ap://' || SUBSTR(id, 9 + INSTR(SUBSTR(id, 9), '/') + 22, CASE WHEN INSTR(SUBSTR(id, 9 + INSTR(SUBSTR(id, 9), '/') + 22), '?') > 0 THEN INSTR(SUBSTR(id, 9 + INSTR(SUBSTR(id, 9), '/') + 22), '?') - 1 ELSE LENGTH(id) END) WHEN id LIKE 'https://%' THEN id ELSE NULL END)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX personscid ON persons(cid)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE UNIQUE INDEX personscidlocal ON persons(cid) WHERE ed25519privkey IS NOT NULL`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DROP INDEX outboxhostinserted`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DROP INDEX outboxcidsender`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE outbox DROP COLUMN host`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE outbox ADD COLUMN host TEXT AS (substr(substr(activity->>'$.id', 9), 0, instr(substr(activity->>'$.id', 9), '/')))`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE outbox DROP COLUMN cid`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE outbox ADD COLUMN cid TEXT NOT NULL AS (CASE WHEN activity->>'$.id' LIKE 'https://%' AND (activity->>'$.id' LIKE '%/.well-known/apgateway/did:key:z6Mk%' OR activity->>'$.id' LIKE '%/.well-known/apgateway/did:key:ukC%') THEN 'ap://' || SUBSTR(activity->>'$.id', 9 + INSTR(SUBSTR(activity->>'$.id', 9), '/') + 22, CASE WHEN INSTR(SUBSTR(activity->>'$.id', 9 + INSTR(SUBSTR(activity->>'$.id', 9), '/') + 22), '?') > 0 THEN INSTR(SUBSTR(activity->>'$.id', 9 + INSTR(SUBSTR(activity->>'$.id', 9), '/') + 22), '?') - 1 ELSE LENGTH(activity->>'$.id') END) WHEN activity->>'$.id' LIKE 'https://%' THEN activity->>'$.id' ELSE NULL END)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX outboxhostinserted ON outbox(host, inserted)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX outboxcidsender ON outbox(cid, sender)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE persons ADD COLUMN mldsa44seed TEXT`); err != nil {
		return err
	}

	rows, err := tx.QueryContext(ctx, `SELECT id, JSON(actor) FROM persons WHERE ed25519privkey IS NOT NULL`, domain)
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

		if _, err := tx.ExecContext(ctx, `UPDATE persons SET actor = JSONB(?), mldsa44seed = ? WHERE id = ?`, &actor, mldsa44Priv.Seed(), id); err != nil {
			return err
		}
	}

	return rows.Err()
}
