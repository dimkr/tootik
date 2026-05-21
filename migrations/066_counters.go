package migrations

import (
	"context"
	"database/sql"
)

func counters(ctx context.Context, domain string, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `ALTER TABLE notes ADD COLUMN nreplies INTEGER DEFAULT 0`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE notes ADD COLUMN nquotes INTEGER DEFAULT 0`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE notes ADD COLUMN nshares INTEGER DEFAULT 0`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE notes ADD COLUMN pulse INTEGER DEFAULT 0`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE notes SET
			nreplies = (SELECT COUNT(*) FROM notes r WHERE r.object->>'$.inReplyTo' = notes.id),
			nquotes = (SELECT COUNT(*) FROM notes quotes WHERE quotes.object->>'$.quote' = notes.id),
			nshares = (SELECT COUNT(*) FROM shares WHERE shares.note = notes.id),
			pulse = COALESCE(
				(SELECT MAX(v) FROM (
					SELECT MAX(replies.inserted) as v FROM notes replies WHERE replies.object->>'$.inReplyTo' = notes.id
					UNION ALL
					SELECT MAX(quotes.inserted) as v FROM notes quotes WHERE quotes.object->>'$.quote' = notes.id
				)),
				notes.inserted
			)
	`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		CREATE TRIGGER nreplies_insert AFTER INSERT ON notes
		WHEN NEW.object->>'$.inReplyTo' IS NOT NULL
		BEGIN
			UPDATE notes
			SET nreplies = nreplies + 1
			WHERE id = NEW.object->>'$.inReplyTo';

			UPDATE notes
			SET pulse = MAX(pulse, NEW.inserted)
			WHERE id IN (
				WITH RECURSIVE thread(id, depth) AS (
					SELECT NEW.object->>'$.inReplyTo', 1
					UNION ALL
					SELECT n.object->>'$.inReplyTo', t.depth + 1
					FROM notes n
					JOIN thread t ON n.id = t.id
					WHERE n.object->>'$.inReplyTo' IS NOT NULL AND t.depth <= 5
				)
				SELECT id FROM thread WHERE id IS NOT NULL
			);
		END
	`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		CREATE TRIGGER nreplies_delete AFTER DELETE ON notes
		WHEN OLD.object->>'$.inReplyTo' IS NOT NULL
		BEGIN
			UPDATE notes
			SET nreplies = MAX(0, nreplies - 1)
			WHERE id = OLD.object->>'$.inReplyTo';
		END
	`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		CREATE TRIGGER nquotes_insert AFTER INSERT ON notes
		WHEN NEW.object->>'$.quote' IS NOT NULL
		BEGIN
			UPDATE notes
			SET nquotes = nquotes + 1, pulse = MAX(pulse, NEW.inserted)
			WHERE id = NEW.object->>'$.quote';
		END
	`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		CREATE TRIGGER nquotes_delete AFTER DELETE ON notes
		WHEN OLD.object->>'$.quote' IS NOT NULL
		BEGIN
			UPDATE notes
			SET nquotes = MAX(0, nquotes - 1)
			WHERE id = OLD.object->>'$.quote';
		END
	`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		CREATE TRIGGER nshares_insert AFTER INSERT ON shares
		BEGIN
			UPDATE notes
			SET nshares = nshares + 1
			WHERE id = NEW.note;
		END
	`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		CREATE TRIGGER nshares_delete AFTER DELETE ON shares
		BEGIN
			UPDATE notes
			SET nshares = MAX(0, nshares - 1)
			WHERE id = OLD.note;
		END
	`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		CREATE TRIGGER notes_insert AFTER INSERT ON notes
		BEGIN
			UPDATE notes SET
				nreplies = (SELECT COUNT(*) FROM notes WHERE object->>'$.inReplyTo' = NEW.id),
				nquotes = (SELECT COUNT(*) FROM notes WHERE object->>'$.quote' = NEW.id),
				nshares = (SELECT COUNT(*) FROM shares WHERE note = NEW.id),
				pulse = COALESCE(
					(SELECT MAX(v) FROM (
						SELECT MAX(replies.inserted) as v FROM notes replies WHERE replies.object->>'$.inReplyTo' = NEW.id
						UNION ALL
						SELECT MAX(quotes.inserted) as v FROM notes quotes WHERE quotes.object->>'$.quote' = NEW.id
					)),
					NEW.inserted
				)
			WHERE id = NEW.id;
		END
	`); err != nil {
		return err
	}

	return nil
}
