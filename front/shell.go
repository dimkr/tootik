/*
Copyright 2026 Dima Krasner

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package front

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"fmt"
	"log/slog"
	"net/url"

	"github.com/cloudflare/circl/sign/mldsa/mldsa44"
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/front/text/gmi"
	"github.com/dimkr/tootik/httpsig"
	"github.com/dimkr/tootik/shell"
)

// Shell runs an interactive shell on behalf of a user.
func (h *Handler) Shell(ctx context.Context, user, domain string) error {
	u, err := url.Parse(fmt.Sprintf("gemini://%s/users", domain))
	if err != nil {
		return err
	}

	var actor ap.Actor
	var rsaPrivKeyDer, ed25519PrivKey, mldsa44Seed []byte
	if err := h.DB.QueryRowContext(
		ctx,
		`select json(actor), rsaprivkey, ed25519privkey, mldsa44seed from persons where actor->>'$.preferredUsername' = ? and ed25519privkey is not null`,
		user,
	).Scan(&actor, &rsaPrivKeyDer, &ed25519PrivKey, &mldsa44Seed); err != nil {
		panic(err)
	}

	rsaPrivKey, err := x509.ParsePKCS1PrivateKey(rsaPrivKeyDer)
	if err != nil {
		panic(err)
	}

	_, mldsa44Priv := mldsa44.NewKeyFromSeed((*[mldsa44.SeedSize]byte)(mldsa44Seed))

	var buf bytes.Buffer

	return shell.Run(ctx, domain, u, func(ctx context.Context, u *url.URL) (*url.URL, string, error) {
		buf.Reset()

		w := gmi.Wrap(&buf)
		h.Handle(
			&Request{
				Context: ctx,
				URL:     u,
				Log:     slog.Default(),
				User:    &actor,
				Keys: [3]httpsig.Key{
					{ID: actor.PublicKey.ID, PrivateKey: rsaPrivKey},
					{ID: actor.AssertionMethod[0].ID, PrivateKey: ed25519.NewKeyFromSeed(ed25519PrivKey)},
					{ID: actor.AssertionMethod[1].ID, PrivateKey: mldsa44Priv},
				},
			},
			w,
		)
		w.Flush()

		return u, buf.String(), nil
	})
}
