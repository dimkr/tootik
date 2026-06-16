package migrations

import (
	"context"
	"database/sql"
)

func hashtags(ctx context.Context, domain string, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `DELETE FROM hashtags`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		INSERT INTO hashtags (note, hashtag)
		SELECT DISTINCT notes.id, CASE WHEN SUBSTR(value->>'$.name', 1, 1) = '#' THEN SUBSTR(value->>'$.name', 2) ELSE value->>'$.name' END COLLATE NOCASE
		FROM notes, JSON_EACH(notes.object->'$.tag')
		WHERE notes.deleted = 0 AND value->>'$.type' = 'Hashtag' AND value->>'$.name' IS NOT NULL AND value->>'$.name' != ''
	`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		CREATE TRIGGER noteshashtagsinserted AFTER INSERT ON notes
		BEGIN
			INSERT INTO hashtags (note, hashtag)
			SELECT DISTINCT new.id, CASE WHEN SUBSTR(value->>'$.name', 1, 1) = '#' THEN SUBSTR(value->>'$.name', 2) ELSE value->>'$.name' END COLLATE NOCASE
			FROM JSON_EACH(new.object->'$.tag')
			WHERE new.deleted = 0 AND value->>'$.type' = 'Hashtag' AND value->>'$.name' IS NOT NULL AND value->>'$.name' != '';
		END
	`); err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `
		CREATE TRIGGER noteshashtagsupdated AFTER UPDATE ON notes
		BEGIN
			DELETE FROM hashtags WHERE note = new.id AND hashtag NOT IN (
				SELECT CASE WHEN SUBSTR(value->>'$.name', 1, 1) = '#' THEN SUBSTR(value->>'$.name', 2) ELSE value->>'$.name' END COLLATE NOCASE
				FROM JSON_EACH(new.object->'$.tag')
				WHERE new.deleted = 0 AND value->>'$.type' = 'Hashtag' AND value->>'$.name' IS NOT NULL AND value->>'$.name' != ''
			);

			INSERT INTO hashtags (note, hashtag)
			SELECT candidates.note, candidates.hashtag FROM (
				SELECT DISTINCT new.id AS note, CASE WHEN SUBSTR(value->>'$.name', 1, 1) = '#' THEN SUBSTR(value->>'$.name', 2) ELSE value->>'$.name' END COLLATE NOCASE AS hashtag
				FROM JSON_EACH(new.object->'$.tag')
				WHERE new.deleted = 0 AND value->>'$.type' = 'Hashtag' AND value->>'$.name' IS NOT NULL AND value->>'$.name' != ''
			) candidates
			WHERE candidates.hashtag NOT IN (SELECT hashtag COLLATE NOCASE FROM hashtags WHERE hashtags.note = candidates.note);
		END
	`); err != nil {
		return err
	}

	_, err := tx.ExecContext(ctx, `
		CREATE TRIGGER noteshashtagsdeleted AFTER DELETE ON notes
		BEGIN
			DELETE FROM hashtags WHERE note = old.id;
		END
	`)
	return err
}
