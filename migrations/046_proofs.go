package migrations

import (
	"context"
	"database/sql"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/data"
	"github.com/dimkr/tootik/httpsig"
	"github.com/dimkr/tootik/proof"
)

func proofs(ctx context.Context, domain string, tx *sql.Tx) error {
	if rows, err := tx.QueryContext(ctx, `SELECT JSON(actor), ed25519privkey FROM persons WHERE ed25519privkey IS NOT NULL`); err != nil {
		return err
	} else {
		defer rows.Close()

		for rows.Next() {
			var actor ap.Actor
			var ed25519PrivKeyMultibase string
			if err := rows.Scan(&actor, &ed25519PrivKeyMultibase); err != nil {
				return err
			}

			ed25519PrivKey, err := data.DecodeEd25519PrivateKey(ed25519PrivKeyMultibase)
			if err != nil {
				return err
			}

			actor.Proof, err = proof.Create(httpsig.Key{ID: actor.AssertionMethod[0].ID, PrivateKey: ed25519PrivKey}, &actor)
			if err != nil {
				return err
			}

			if _, err := tx.ExecContext(ctx, `UPDATE persons SET actor = JSONB(?) WHERE id = ?`, &actor, actor.ID); err != nil {
				return err
			}
		}

		if err := rows.Err(); err != nil {
			return err
		}
	}

	if rows, err := tx.QueryContext(ctx, `SELECT JSON(notes.object), JSON(persons.actor), persons.ed25519privkey FROM notes JOIN persons ON persons.id = notes.author WHERE persons.ed25519privkey IS NOT NULL`); err != nil {
		return err
	} else {
		defer rows.Close()

		for rows.Next() {
			var note ap.Object
			var actor ap.Actor
			var ed25519PrivKeyMultibase string
			if err := rows.Scan(&note, &actor, &ed25519PrivKeyMultibase); err != nil {
				return err
			}

			ed25519PrivKey, err := data.DecodeEd25519PrivateKey(ed25519PrivKeyMultibase)
			if err != nil {
				return err
			}

			note.Proof, err = proof.Create(httpsig.Key{ID: actor.AssertionMethod[0].ID, PrivateKey: ed25519PrivKey}, &note)
			if err != nil {
				return err
			}

			if _, err := tx.ExecContext(ctx, `UPDATE notes SET object = JSONB(?) WHERE id = ?`, &note, note.ID); err != nil {
				return err
			}
		}

		if err := rows.Err(); err != nil {
			return err
		}
	}

	if rows, err := tx.QueryContext(ctx, `SELECT JSON(outbox.activity), JSON(persons.actor), persons.ed25519privkey FROM outbox JOIN persons ON persons.id = outbox.activity->>'$.actor' WHERE persons.ed25519privkey IS NOT NULL`); err != nil {
		return err
	} else {
		defer rows.Close()

		for rows.Next() {
			var activity ap.Activity
			var actor ap.Actor
			var ed25519PrivKeyMultibase string
			if err := rows.Scan(&activity, &actor, &ed25519PrivKeyMultibase); err != nil {
				return err
			}

			ed25519PrivKey, err := data.DecodeEd25519PrivateKey(ed25519PrivKeyMultibase)
			if err != nil {
				return err
			}

			activity.Proof, err = proof.Create(httpsig.Key{ID: actor.AssertionMethod[0].ID, PrivateKey: ed25519PrivKey}, &activity)
			if err != nil {
				return err
			}

			if _, err := tx.ExecContext(ctx, `UPDATE outbox SET activity = JSONB(?) WHERE activity->>'$.id' = ?`, &activity, activity.ID); err != nil {
				return err
			}
		}

		if err := rows.Err(); err != nil {
			return err
		}
	}

	return nil
}
