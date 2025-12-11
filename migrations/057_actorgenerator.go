package migrations

import (
	"context"
	"crypto/ed25519"
	"database/sql"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/httpsig"
	"github.com/dimkr/tootik/proof"
)

func actorgenerator(ctx context.Context, domain string, tx *sql.Tx) error {
	var actor ap.Actor
	var ed25519PrivKey []byte
	if err := tx.QueryRowContext(
		ctx,
		`SELECT JSON(actor), ed25519privkey FROM persons WHERE ed25519privkey IS NOT NULL AND actor->>'$.preferredUsername' = 'actor'`,
	).Scan(&actor, &ed25519PrivKey); err != nil {
		return err
	}

	actor.Generator.Type = ap.Application
	actor.Generator.Implements = []ap.Implement{
		{
			Name: "RFC-9421: HTTP Message Signatures",
			Href: "https://datatracker.ietf.org/doc/html/rfc9421",
		},
		{
			Name: "RFC-9421 signatures using the Ed25519 algorithm",
			Href: "https://datatracker.ietf.org/doc/html/rfc9421#name-eddsa-using-curve-edwards25",
		},
	}

	var err error
	actor.Proof, err = proof.Create(httpsig.Key{ID: actor.AssertionMethod[0].ID, PrivateKey: ed25519.NewKeyFromSeed(ed25519PrivKey)}, &actor)
	if err != nil {
		return err
	}

	if _, err := tx.ExecContext(ctx, `UPDATE persons SET actor = JSONB(?) WHERE id = ?`, &actor, actor.ID); err != nil {
		return err
	}

	return nil
}
