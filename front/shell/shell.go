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

// Package shell displays the frontend as an interactive shell.
package shell

import (
	"bytes"
	"context"
	"crypto/ed25519"
	"crypto/x509"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/url"
	"strconv"
	"strings"

	"github.com/dimkr/slopline"
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/front"
	"github.com/dimkr/tootik/front/text/gmi"
	"github.com/dimkr/tootik/gemtext"
	"github.com/dimkr/tootik/httpsig"
)

// Run runs a shell on behalf of a user.
func Run(ctx context.Context, handler front.Handler, user, domain string) error {
	u, err := url.Parse(fmt.Sprintf("gemini://%s/users", domain))
	if err != nil {
		return err
	}

	var actor ap.Actor
	var rsaPrivKeyDer, ed25519PrivKey []byte
	if err := handler.DB.QueryRowContext(
		ctx,
		`select json(actor), rsaprivkey, ed25519privkey from persons where actor->>'$.preferredUsername' = ? and ed25519privkey is not null`,
		user,
	).Scan(&actor, &rsaPrivKeyDer, &ed25519PrivKey); err != nil {
		panic(err)
	}

	rsaPrivKey, err := x509.ParsePKCS1PrivateKey(rsaPrivKeyDer)
	if err != nil {
		panic(err)
	}

	var buf bytes.Buffer

outer:
	for {
		if err := ctx.Err(); err != nil {
			return err
		}

		buf.Reset()

		w := gmi.Wrap(&buf)
		handler.Handle(
			&front.Request{
				Context: ctx,
				URL:     u,
				Log:     slog.Default(),
				User:    &actor,
				Keys: [2]httpsig.Key{
					{ID: actor.PublicKey.ID, PrivateKey: rsaPrivKey},
					{ID: actor.AssertionMethod[0].ID, PrivateKey: ed25519.NewKeyFromSeed(ed25519PrivKey)},
				},
			},
			w,
		)
		w.Flush()

		status, lines, links := gemtext.Parse(buf.String())

		slopline.SetHintsCallback(func(text string) (string, string, string) {
			if text == "" && len(links) > 0 {
				return fmt.Sprintf(" 1-%d", len(links)), "\033[90m", "\033[0m"
			} else if len(links) == 0 {
				return "", "", ""
			}

			if n, err := strconv.Atoi(text); err == nil && n > 0 {
				i := 0
				for _, line := range lines {
					if line.Type != gemtext.Link {
						continue
					}

					i++
					if i == n {
						return " " + line.Text, "\033[90m", "\033[0m"
					}
				}
			}

			return "", "", ""
		})

		if strings.HasPrefix(status, "30 ") {
			rel, _ := url.Parse(status[3 : buf.Len()-2])
			u = u.ResolveReference(rel)
			continue
		}

		if strings.HasPrefix(status, "10 ") {
			for {
				line, err := slopline.Line(fmt.Sprintf("\033[35m%s>\033[0m ", status[3:]))
				if errors.Is(err, io.EOF) {
					return nil
				} else if err != nil {
					return err
				}

				line = strings.TrimSpace(line)
				if line == "" {
					continue
				}

				if line != "" {
					u.RawQuery = line
					continue outer
				}
			}
		}

		if err := gemtext.Pager(ctx, lines, 80); err != nil {
			return err
		}

		prompt := domain
		for _, line := range lines {
			if line.Type == gemtext.Heading {
				prompt = line.Text
				break
			}
		}

		line, err := slopline.Line(fmt.Sprintf("\033[35m%s>\033[0m ", prompt))
		if errors.Is(err, io.EOF) {
			return nil
		} else if err != nil {
			return err
		}

		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if n, err := strconv.Atoi(line); err == nil && n > 0 && n <= len(links) {
			linkID := 1
			for _, line := range lines {
				if line.Type != gemtext.Link {
					continue
				}

				if linkID < n {
					linkID++
					continue
				}

				rel, err := url.Parse(line.URL)
				if err != nil {
					return err
				}

				u = u.ResolveReference(rel)
				break
			}
		} else {
			rel, err := url.Parse(line)
			if err != nil {
				fmt.Printf("Invalid URL or command: %s\n", line)
			}

			u = u.ResolveReference(rel)
		}
	}
}
