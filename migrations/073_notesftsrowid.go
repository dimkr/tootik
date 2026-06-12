package migrations

import (
	"context"
	"database/sql"
)

func notesftsrowid(ctx context.Context, domain string, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `CREATE VIRTUAL TABLE nnotesfts USING fts5(content, tokenize = "unicode61 tokenchars '#@'")`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `INSERT INTO nnotesfts(rowid, content) SELECT notes.rowid, notesfts.content FROM notesfts JOIN notes ON notes.id = notesfts.id`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DROP TABLE notesfts`); err != nil {
		return err
	}

	_, err := tx.ExecContext(ctx, `ALTER TABLE nnotesfts RENAME TO notesfts`)
	return err
}
