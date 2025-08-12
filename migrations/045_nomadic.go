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

	if _, err := tx.ExecContext(ctx, `DROP INDEX outboxhostinserted`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE outbox DROP COLUMN host`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE outbox ADD COLUMN host TEXT AS (CASE WHEN activity->>'$.id' LIKE 'ap://%' THEN substr(substr(activity->>'$.id', 6), 0, instr(substr(activity->>'$.id', 6), '/')) ELSE substr(substr(activity->>'$.id', 9), 0, instr(substr(activity->>'$.id', 9), '/')) END)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX outboxhostinserted ON outbox(host, inserted)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE UNIQUE INDEX personslocal ON persons(id) WHERE ed25519privkey IS NOT NULL`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE UNIQUE INDEX personsnomadic ON persons(id, actor->>'$.assertionMethod[0].id') WHERE ed25519privkey IS NOT NULL AND id LIKE 'ap://%'`); err != nil {
		return err
	}

	return nil
}
