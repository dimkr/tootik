package migrations

import (
	"context"
	"database/sql"
)

func counters(ctx context.Context, domain string, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `ALTER TABLE notes ADD COLUMN replies_count INTEGER DEFAULT 0`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE notes ADD COLUMN quotes_count INTEGER DEFAULT 0`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE notes SET
		replies_count = (SELECT COUNT(*) FROM notes r WHERE r.object->>'$.inReplyTo' = notes.id),
		quotes_count = (SELECT COUNT(*) FROM notes q WHERE q.object->>'$.quote' = notes.id)
	`); err != nil {
		return err
	}

	// Trigger for new replies
	if _, err := tx.ExecContext(ctx, `
		CREATE TRIGGER notes_replies_count_insert AFTER INSERT ON notes WHEN NEW.object->>'$.inReplyTo' IS NOT NULL
		BEGIN
			UPDATE notes SET replies_count = replies_count + 1 WHERE id = NEW.object->>'$.inReplyTo';
		END
	`); err != nil {
		return err
	}

	// Trigger for deleted replies
	if _, err := tx.ExecContext(ctx, `
		CREATE TRIGGER notes_replies_count_delete AFTER DELETE ON notes WHEN OLD.object->>'$.inReplyTo' IS NOT NULL
		BEGIN
			UPDATE notes SET replies_count = MAX(0, replies_count - 1) WHERE id = OLD.object->>'$.inReplyTo';
		END
	`); err != nil {
		return err
	}

	// Trigger for new quotes
	if _, err := tx.ExecContext(ctx, `
		CREATE TRIGGER notes_quotes_count_insert AFTER INSERT ON notes WHEN NEW.object->>'$.quote' IS NOT NULL
		BEGIN
			UPDATE notes SET quotes_count = quotes_count + 1 WHERE id = NEW.object->>'$.quote';
		END
	`); err != nil {
		return err
	}

	// Trigger for deleted quotes
	if _, err := tx.ExecContext(ctx, `
		CREATE TRIGGER notes_quotes_count_delete AFTER DELETE ON notes WHEN OLD.object->>'$.quote' IS NOT NULL
		BEGIN
			UPDATE notes SET quotes_count = MAX(0, quotes_count - 1) WHERE id = OLD.object->>'$.quote';
		END
	`); err != nil {
		return err
	}

	// Trigger for new notes to handle replies/quotes that arrived before the parent
	if _, err := tx.ExecContext(ctx, `
		CREATE TRIGGER notes_initial_counts AFTER INSERT ON notes
		BEGIN
			UPDATE notes SET
				replies_count = (SELECT COUNT(*) FROM notes WHERE object->>'$.inReplyTo' = NEW.id),
				quotes_count = (SELECT COUNT(*) FROM notes WHERE object->>'$.quote' = NEW.id)
			WHERE id = NEW.id;
		END
	`); err != nil {
		return err
	}

	return nil
}
