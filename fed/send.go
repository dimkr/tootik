/*
Copyright 2023 Dima Krasner

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless ruired by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package fed

import (
	"bytes"
	"context"
	"crypto/sha256"
	"crypto/x509"
	"database/sql"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/cfg"
	log "github.com/dimkr/tootik/slogru"
	"github.com/go-fed/httpsig"
	"io"
	"io/ioutil"
	"net/http"
	"time"
)

func send(db *sql.DB, from *ap.Actor, resolver *Resolver, r *http.Request) (*http.Response, error) {
	urlString := r.URL.String()

	if r.URL.Scheme != "https" {
		return nil, fmt.Errorf("Invalid scheme in %s: %s", urlString, r.URL.Scheme)
	}

	if r.URL.Host == "localhost" || r.URL.Host == "localhost.localdomain" || r.URL.Host == "127.0.0.1" || r.URL.Host == "::1" {
		return nil, fmt.Errorf("Invalid host in %s: %s", urlString, r.URL.Host)
	}

	if from == nil {
		var err error
		from, err = resolver.Resolve(r.Context(), db, nil, fmt.Sprintf("https://%s/user/nobody", cfg.Domain))
		if err != nil {
			return nil, fmt.Errorf("Cannot resolve nobody user to send request: %w", err)
		}
	}

	resolver.Log.WithFields(log.Fields{"url": urlString, "from": from.ID}).Info("Sending request")

	r.Header.Set("Content-Type", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)

	if from != nil {
		var publicKeyID, privateKeyPemString string
		if err := db.QueryRowContext(r.Context(), `select actor->>'publicKey.id', actor->>'privateKey' from persons where id = ?`, from.ID).Scan(&publicKeyID, &privateKeyPemString); err != nil {
			return nil, fmt.Errorf("Failed to fetch key for from %s: %w", from.ID, err)
		}

		signer, _, err := httpsig.NewSigner(
			[]httpsig.Algorithm{httpsig.RSA_SHA256},
			httpsig.DigestSha256,
			[]string{httpsig.RequestTarget, "host", "date", "digest"},
			httpsig.Signature,
			int64(time.Hour*12/time.Second),
		)
		if err != nil {
			return nil, fmt.Errorf("Failed to sign body for %s: %w", urlString, err)
		}

		var body []byte
		var hash [sha256.Size]byte

		if r.Body == nil {
			hash = sha256.Sum256(nil)
		} else {
			body, err = io.ReadAll(r.Body)
			if err != nil {
				return nil, fmt.Errorf("Failed to read body for %s: %w", urlString, err)
			}

			r.Body = io.NopCloser(bytes.NewReader(body))
			hash = sha256.Sum256(body)
		}

		privateKeyPem, _ := pem.Decode([]byte(privateKeyPemString))

		privateKey, err := x509.ParsePKCS1PrivateKey(privateKeyPem.Bytes)
		if err != nil {
			return nil, fmt.Errorf("Failed to sign body for %s: %w", urlString, err)
		}

		r.Header.Add("Digest", "SHA-256="+base64.StdEncoding.EncodeToString(hash[:]))
		r.Header.Add("Date", time.Now().UTC().Format(http.TimeFormat))
		r.Header.Add("Host", r.URL.Host)

		if err := signer.SignRequest(privateKey, publicKeyID, r, nil); err != nil {
			return nil, fmt.Errorf("Failed to sign request for %s: %w", urlString, err)
		}
	}

	client := http.Client{}

	resp, err := client.Do(r)
	if err != nil {
		return nil, fmt.Errorf("Failed to send request to %s: %w", urlString, err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		body, err := ioutil.ReadAll(resp.Body)
		resp.Body.Close()
		if err != nil {
			return resp, fmt.Errorf("Failed to send request to %s: %d, %w", urlString, resp.StatusCode, err)
		}
		return resp, fmt.Errorf("Failed to send request to %s: %d, %s", urlString, resp.StatusCode, string(body))
	}

	return resp, nil
}

func Send(ctx context.Context, db *sql.DB, from *ap.Actor, resolver *Resolver, to *ap.Actor, body []byte) error {
	if to.Inbox == "" {
		return fmt.Errorf("Cannot send request to %s: no inbox link", to.ID)
	}

	r, err := http.NewRequestWithContext(ctx, http.MethodPost, to.Inbox, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("Failed to send request to %s: %w", to.ID, err)
	}

	if r.URL.Host == cfg.Domain {
		resolver.Log.WithFields(log.Fields{"to": to.ID, "from": from.ID}).Info("Skipping request")
		return nil
	}

	r.Header.Set("Accept", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)

	resp, err := send(db, from, resolver, r)
	if err != nil {
		return fmt.Errorf("Failed to send request to %s: %w", to.ID, err)
	}
	defer resp.Body.Close()

	respBody, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("Failed to send request to %s: %w", to.ID, err)
	}

	resolver.Log.WithFields(log.Fields{"to": to.ID, "body": string(respBody)}).Info("Successfully sent message")
	return nil
}
