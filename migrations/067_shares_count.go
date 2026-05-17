package migrations

import (
	"context"
	"database/sql"
)

func shares_count(ctx context.Context, domain string, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `ALTER TABLE notes ADD COLUMN shares_count INTEGER DEFAULT 0`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE notes SET
		shares_count = (SELECT COUNT(*) FROM shares WHERE shares.note = notes.id)
	`); err != nil {
		return err
	}

	// Trigger for new shares
	if _, err := tx.ExecContext(ctx, `
		CREATE TRIGGER shares_count_insert AFTER INSERT ON shares
		BEGIN
			UPDATE notes SET shares_count = shares_count + 1 WHERE id = NEW.note;
		END
	`); err != nil {
		return err
	}

	// Trigger for deleted shares
	if _, err := tx.ExecContext(ctx, `
		CREATE TRIGGER shares_count_delete AFTER DELETE ON shares
		BEGIN
			UPDATE notes SET shares_count = MAX(0, shares_count - 1) WHERE id = OLD.note;
		END
	`); err != nil {
		return err
	}

	// Update the initial counts trigger from migration 066
	if _, err := tx.ExecContext(ctx, `DROP TRIGGER notes_initial_counts`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		CREATE TRIGGER notes_initial_counts AFTER INSERT ON notes
		BEGIN
			UPDATE notes SET
				replies_count = (SELECT COUNT(*) FROM notes WHERE object->>'$.inReplyTo' = NEW.id),
				quotes_count = (SELECT COUNT(*) FROM notes WHERE object->>'$.quote' = NEW.id),
				shares_count = (SELECT COUNT(*) FROM shares WHERE note = NEW.id)
			WHERE id = NEW.id;
		END
	`); err != nil {
		return err
	}

	return nil
}
