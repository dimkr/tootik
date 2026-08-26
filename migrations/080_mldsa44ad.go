package migrations

import (
	"context"
	"crypto/ed25519"
	"database/sql"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/httpsig"
	"github.com/dimkr/tootik/proof"
)

func mldsa44ad(ctx context.Context, domain string, tx *sql.Tx) error {
	var actor ap.Actor
	var ed25519Seed []byte
	if err := tx.QueryRowContext(
		ctx,
		`SELECT JSON(actor), ed25519seed FROM persons WHERE ed25519seed IS NOT NULL AND actor->>'$.preferredUsername' = 'actor'`,
	).Scan(&actor, &ed25519Seed); err != nil {
		return err
	}

	actor.Implements = append(actor.Implements, ap.Implement{
		Name: "RFC-9421 signatures using the ml-dsa-44 algorithm",
		Href: "https://c2sp.org/httpsig-pq@v1.0.0#ml-dsa-44",
	})

	var err error
	actor.Proof, err = proof.Create(
		httpsig.Key{
			ID:         actor.AssertionMethod[0].ID,
			PrivateKey: ed25519.NewKeyFromSeed(ed25519Seed),
		},
		&actor,
	)
	if err != nil {
		return err
	}

	if _, err := tx.ExecContext(
		ctx,
		`UPDATE persons SET actor = JSONB(?) WHERE id = ?`,
		&actor,
		actor.ID,
	); err != nil {
		return err
	}

	return nil
}
