package migrations

import (
	"context"
	"database/sql"
)

func slug(ctx context.Context, domain string, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `ALTER TABLE persons ADD COLUMN slug TEXT`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `UPDATE persons SET slug = id`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE persons ALTER COLUMN slug SET NOT NULL`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE notes ADD COLUMN slug TEXT`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `UPDATE notes SET slug = id`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE notes ALTER COLUMN slug SET NOT NULL`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE VIRTUAL TABLE nnotesfts USING fts5(slug, content, tokenize = "unicode61 tokenchars '#@'")`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO nnotesfts(slug, content) SELECT notes.slug, notesfts.content FROM notesfts JOIN notes ON notes.id = notesfts.rowid`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DROP TABLE notesfts`); err != nil {
		return err
	}

	_, err := tx.ExecContext(ctx, `ALTER TABLE nnotesfts RENAME TO notesfts`)
	return err
}
