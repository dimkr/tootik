package migrations

import (
	"context"
	"database/sql"
	"fmt"
)

func cutQuerySQL(s string) string {
	return fmt.Sprintf(`(CASE WHEN INSTR(%[1]s, '?') > 0 THEN SUBSTR(%[1]s, 1, INSTR(%[1]s, '?') - 1) ELSE %[1]s END)`, s)
}

func dropQuerySQL(s string) string {
	return fmt.Sprintf(
		`(CASE WHEN INSTR(%[1]s, '#') > 0 THEN %[2]s || SUBSTR(%[1]s, INSTR(%[1]s, '#')) ELSE %[3]s END)`,
		s,
		cutQuerySQL(fmt.Sprintf(`SUBSTR(%[1]s, 1, INSTR(%[1]s, '#') - 1)`, s)),
		cutQuerySQL(s),
	)
}

func canonicalPortableSQL(authority string) string {
	return fmt.Sprintf(
		`(CASE WHEN SUBSTR(%[1]s, 1, 8) = 'did:key:' THEN 'ap://' || %[2]s WHEN LOWER(SUBSTR(%[1]s, 1, 12)) = 'did%%3akey%%3a' THEN 'ap://' || %[3]s END)`,
		authority,
		dropQuerySQL(authority),
		dropQuerySQL(fmt.Sprintf(`('did:key:' || SUBSTR(%s, 13))`, authority)),
	)
}

// canonicalSQL returns a SQL expression that evaluates to [ap.Canonical] of id, for every id accepted by [ap.ParseID].
func canonicalSQL(id string) string {
	slash := fmt.Sprintf(`(INSTR(SUBSTR(%[1]s, 9), '/') + 8)`, id)
	key := fmt.Sprintf(`SUBSTR(%s, %s + 31)`, id, slash)

	return fmt.Sprintf(
		`(CASE WHEN SUBSTR(%[1]s, 1, 8) = 'https://' AND SUBSTR(%[1]s, %[2]s, 31) = '/.well-known/apgateway/did:key:' AND (SUBSTR(%[3]s, 1, 4) = 'z6Mk' OR SUBSTR(%[3]s, 1, 3) = 'ukC') AND INSTR(SUBSTR(%[1]s, %[2]s + 1), '/.well-known/apgateway/did:') = 0 THEN 'ap://' || %[4]s WHEN SUBSTR(%[1]s, 1, 5) = 'ap://' THEN COALESCE(%[5]s, %[1]s) WHEN SUBSTR(%[1]s, 1, 10) = 'ap+ef61://' THEN COALESCE(%[6]s, %[1]s) ELSE %[1]s END)`,
		id,
		slash,
		key,
		dropQuerySQL(fmt.Sprintf(`SUBSTR(%s, %s + 23)`, id, slash)),
		canonicalPortableSQL(fmt.Sprintf(`SUBSTR(%s, 6)`, id)),
		canonicalPortableSQL(fmt.Sprintf(`SUBSTR(%s, 11)`, id)),
	)
}

func canonicalids(ctx context.Context, domain string, tx *sql.Tx) error {
	activityID := `activity->>'$.id'`
	activityCID := canonicalSQL(activityID)

	for _, stmt := range []string{
		`DROP INDEX outboxcidsender`,
		`DROP INDEX outboxhostinserted`,
		`DROP INDEX outboxactor`,
		`ALTER TABLE outbox DROP COLUMN cid`,
		`ALTER TABLE outbox DROP COLUMN host`,
		fmt.Sprintf(`ALTER TABLE outbox ADD COLUMN cid TEXT NOT NULL AS (%s)`, activityCID),
		fmt.Sprintf(`ALTER TABLE outbox ADD COLUMN host TEXT AS (CASE WHEN SUBSTR(%[1]s, 1, 8) = 'https://' THEN SUBSTR(SUBSTR(%[1]s, 9), 0, INSTR(SUBSTR(%[1]s, 9), '/')) WHEN SUBSTR(%[2]s, 1, 5) = 'ap://' THEN SUBSTR(%[2]s, 6, INSTR(SUBSTR(%[2]s, 6), '/') - 1) END)`, activityID, activityCID),
		fmt.Sprintf(`ALTER TABLE outbox ADD COLUMN actorcid TEXT AS (%s)`, canonicalSQL(`COALESCE(activity->>'$.actor.id', activity->>'$.actor')`)),
		`CREATE INDEX outboxcidsender ON outbox(cid, sender)`,
		`CREATE INDEX outboxhostinserted ON outbox(host, inserted)`,
		`CREATE INDEX outboxactorcidinserted ON outbox(actorcid, inserted)`,

		`DROP TRIGGER nreplies_insert`,
		`DROP TRIGGER nreplies_delete`,
		`DROP TRIGGER nquotes_insert`,
		`DROP TRIGGER nquotes_delete`,
		`DROP TRIGGER notes_insert`,
		`DROP TRIGGER nshares_insert`,
		`DROP TRIGGER nshares_delete`,
		`DROP INDEX notesinreplytoinserted`,
		`DROP INDEX notesquote`,
		`DROP INDEX notesaudience`,
		`DROP INDEX localnotescontext`,
		fmt.Sprintf(`ALTER TABLE notes ADD COLUMN inreplytocid TEXT AS (%s)`, canonicalSQL(`object->>'$.inReplyTo'`)),
		fmt.Sprintf(`ALTER TABLE notes ADD COLUMN quotecid TEXT AS (%s)`, canonicalSQL(`object->>'$.quote'`)),
		fmt.Sprintf(`ALTER TABLE notes ADD COLUMN audiencecid TEXT AS (%s)`, canonicalSQL(`object->>'$.audience'`)),
		fmt.Sprintf(`ALTER TABLE notes ADD COLUMN contextcid TEXT AS (%s)`, canonicalSQL(`object->>'$.context'`)),
		`CREATE INDEX notesinreplytocidinserted ON notes(inreplytocid, inserted) WHERE inreplytocid IS NOT NULL`,
		`CREATE INDEX notesquotecid ON notes(quotecid) WHERE quotecid IS NOT NULL`,
		`CREATE INDEX notesaudiencecid ON notes(audiencecid)`,
		`CREATE INDEX notescontextcid ON notes(contextcid) WHERE contextcid IS NOT NULL`,
		`CREATE TRIGGER nreplies_insert AFTER INSERT ON notes
		WHEN NEW.inreplytocid IS NOT NULL
		BEGIN
			UPDATE notes
			SET nreplies = nreplies + 1
			WHERE cid = NEW.inreplytocid;

			UPDATE notes
			SET pulse = MAX(pulse, NEW.inserted)
			WHERE cid IN (
				WITH RECURSIVE thread(cid, depth) AS (
					SELECT NEW.inreplytocid, 1
					UNION ALL
					SELECT n.inreplytocid, t.depth + 1
					FROM notes n
					JOIN thread t ON n.cid = t.cid
					WHERE n.inreplytocid IS NOT NULL AND t.depth <= 5
				)
				SELECT cid FROM thread WHERE cid IS NOT NULL
			);
		END`,
		`CREATE TRIGGER nreplies_delete AFTER DELETE ON notes
		WHEN OLD.inreplytocid IS NOT NULL
		BEGIN
			UPDATE notes
			SET nreplies = MAX(0, nreplies - 1)
			WHERE cid = OLD.inreplytocid;
		END`,
		`CREATE TRIGGER nquotes_insert AFTER INSERT ON notes
		WHEN NEW.quotecid IS NOT NULL
		BEGIN
			UPDATE notes
			SET nquotes = nquotes + 1, pulse = MAX(pulse, NEW.inserted)
			WHERE cid = NEW.quotecid;
		END`,
		`CREATE TRIGGER nquotes_delete AFTER DELETE ON notes
		WHEN OLD.quotecid IS NOT NULL
		BEGIN
			UPDATE notes
			SET nquotes = MAX(0, nquotes - 1)
			WHERE cid = OLD.quotecid;
		END`,
		fmt.Sprintf(`CREATE TRIGGER notes_insert AFTER INSERT ON notes
		BEGIN
			UPDATE notes SET
				nreplies = (SELECT COUNT(*) FROM notes WHERE inreplytocid = NEW.cid),
				nquotes = (SELECT COUNT(*) FROM notes WHERE quotecid = NEW.cid),
				nshares = (SELECT COUNT(*) FROM shares WHERE note = NEW.id AND %s IS NOT NEW.audiencecid),
				pulse = COALESCE(
					(SELECT MAX(v) FROM (
						SELECT MAX(replies.inserted) as v FROM notes replies WHERE replies.inreplytocid = NEW.cid
						UNION ALL
						SELECT MAX(quotes.inserted) as v FROM notes quotes WHERE quotes.quotecid = NEW.cid
					)),
					NEW.inserted
				)
			WHERE id = NEW.id;
		END`, canonicalSQL(`shares.by`)),
		fmt.Sprintf(`CREATE TRIGGER nshares_insert AFTER INSERT ON shares
		BEGIN
			UPDATE notes
			SET nshares = nshares + 1
			WHERE id = NEW.note AND %s IS NOT audiencecid;
		END`, canonicalSQL(`NEW.by`)),
		fmt.Sprintf(`CREATE TRIGGER nshares_delete AFTER DELETE ON shares
		BEGIN
			UPDATE notes
			SET nshares = MAX(0, nshares - 1)
			WHERE id = OLD.note AND %s IS NOT audiencecid;
		END`, canonicalSQL(`OLD.by`)),

		fmt.Sprintf(`UPDATE OR IGNORE deliveries SET activity = %[1]s WHERE %[1]s != activity`, canonicalSQL(`activity`)),
		fmt.Sprintf(`DELETE FROM deliveries WHERE %s != activity`, canonicalSQL(`activity`)),
		fmt.Sprintf(`UPDATE OR IGNORE icons SET cid = %[1]s WHERE %[1]s != cid`, canonicalSQL(`cid`)),
		fmt.Sprintf(`DELETE FROM icons WHERE %s != cid`, canonicalSQL(`cid`)),
	} {
		if _, err := tx.ExecContext(ctx, stmt); err != nil {
			return err
		}
	}

	return nil
}
