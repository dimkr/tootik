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

	if _, err := tx.ExecContext(ctx, `UPDATE notes SET pulse = inserted`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE notes
		SET nreplies = rc.count, pulse = MAX(notes.pulse, rc.pulse)
		FROM (
			SELECT object->>'$.inReplyTo' AS parent, COUNT(*) AS count, MAX(inserted) AS pulse
			FROM notes
			WHERE parent IS NOT NULL
			GROUP BY parent
		) rc
		WHERE notes.id = rc.parent
	`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE notes
		SET nshares = sc.count, pulse = MAX(notes.pulse, sc.pulse)
		FROM (
			SELECT shares.note, COUNT(*) AS count, MAX(shares.inserted) AS pulse
			FROM shares
			JOIN notes ON shares.note = notes.id
			WHERE shares.by IS NOT notes.object->>'$.audience'
			GROUP BY shares.note
		) sc
		WHERE notes.id = sc.note
	`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE notes
		SET nquotes = qc.count, pulse = MAX(notes.pulse, qc.pulse)
		FROM (
			SELECT object->>'$.quote' AS quote, COUNT(*) AS count, MAX(inserted) AS pulse
			FROM notes
			WHERE quote IS NOT NULL
			GROUP BY quote
		) qc
		WHERE notes.id = qc.quote
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
			WHERE id = NEW.note AND NEW.by IS NOT object->>'$.audience';
		END
	`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		CREATE TRIGGER nshares_delete AFTER DELETE ON shares
		BEGIN
			UPDATE notes
			SET nshares = MAX(0, nshares - 1)
			WHERE id = OLD.note AND OLD.by IS NOT object->>'$.audience';
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
				nshares = (SELECT COUNT(*) FROM shares WHERE note = NEW.id AND shares.by IS NOT NEW.object->>'$.audience'),
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

	if _, err := tx.ExecContext(ctx, `CREATE TABLE nfeed(follower TEXT NOT NULL, note TEXT NOT NULL, author TEXT NOT NULL, sharer TEXT, mention INTEGER NOT NULL DEFAULT 0, inserted INTEGER NOT NULL)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO nfeed(follower, note, author, sharer, mention, inserted)
		SELECT
			follower,
			note->>'$.id',
			author->>'$.id',
			sharer->>'$.id',
			exists (select 1 from json_each(note->'$.to') where value = follower) or exists (select 1 from json_each(note->'$.cc') where value = follower),
			inserted
		FROM feed
	`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `DROP TABLE feed`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `ALTER TABLE nfeed RENAME TO feed`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX feedfollowerinserted ON feed(follower, inserted)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX feedinserted ON feed(inserted)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX feednote ON feed(note)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX feedauthorid ON feed(author)`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `CREATE INDEX feedshareid ON feed(sharer) WHERE sharer IS NOT NULL`); err != nil {
		return err
	}

	return nil
}
