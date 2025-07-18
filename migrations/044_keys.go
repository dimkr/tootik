package migrations

import (
	"context"
	"database/sql"
)

func keys(ctx context.Context, domain string, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `CREATE TABLE keys(id TEXT PRIMARY KEY, actor TEXT NOT NULL)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO keys(id, actor) SELECT persons.id, actor->>'$.publicKey.id' FROM persons WHERE actor->>'$.publicKey.id' IS NOT NULL AND host != ? AND actor->>'$.publicKey.id' LIKE 'https://' || host || '/%'`, domain); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO keys(id, actor) SELECT persons.id, actor->>'$.assertionMethod[0].id' FROM persons WHERE actor->>'$.assertionMethod[0].id' IS NOT NULL AND actor->>'$.assertionMethod[0].type' = 'Multikey' AND host != ? AND actor->>'$.assertionMethod[0].id' LIKE 'https://' || host || '/%'`, domain); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO keys(id, actor) SELECT persons.id, actor->>'$.assertionMethod[1].id' FROM persons WHERE actor->>'$.assertionMethod[1].id' IS NOT NULL AND actor->>'$.assertionMethod[1].type' = 'Multikey' AND host != ? AND actor->>'$.assertionMethod[1].id' LIKE 'https://' || host || '/%'`, domain); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO keys(id, actor) SELECT persons.id, actor->>'$.assertionMethod[2].id' FROM persons WHERE actor->>'$.assertionMethod[2].id' IS NOT NULL AND actor->>'$.assertionMethod[2].type' = 'Multikey' AND host != ? AND actor->>'$.assertionMethod[2].id' LIKE 'https://' || host || '/%'`, domain); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DROP INDEX personspublickeyid`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX keysactor ON keys(actor)`); err != nil {
		return err
	}

	return nil
}
