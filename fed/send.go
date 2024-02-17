/*
Copyright 2023, 2024 Dima Krasner

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
	"database/sql"
	"encoding/base64"
	"fmt"
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/buildinfo"
	"github.com/dimkr/tootik/cfg"
	"github.com/go-fed/httpsig"
	"io"
	"log/slog"
	"net/http"
	"time"
)

var userAgent = "tootik/" + buildinfo.Version

type sender struct {
	Domain string
	Config *cfg.Config
	client Client
}

func (s *sender) send(log *slog.Logger, db *sql.DB, from *ap.Actor, key *key, req *http.Request) (*http.Response, error) {
	urlString := req.URL.String()

	if req.URL.Scheme != "https" {
		return nil, fmt.Errorf("invalid scheme in %s: %s", urlString, req.URL.Scheme)
	}

	if req.URL.Host == "localhost" || req.URL.Host == "localhost.localdomain" || req.URL.Host == "127.0.0.1" || req.URL.Host == "::1" {
		return nil, fmt.Errorf("invalid host in %s: %s", urlString, req.URL.Host)
	}

	req.Header.Set("Content-Type", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)

	log.Debug("Sending request", "url", urlString)

	signer, _, err := httpsig.NewSigner(
		[]httpsig.Algorithm{httpsig.RSA_SHA256},
		httpsig.DigestSha256,
		[]string{httpsig.RequestTarget, "host", "date", "digest"},
		httpsig.Signature,
		int64(time.Hour*12/time.Second),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to sign body for %s: %w", urlString, err)
	}

	var body []byte
	var hash [sha256.Size]byte

	if req.Body == nil {
		hash = sha256.Sum256(nil)
	} else {
		body, err = io.ReadAll(req.Body)
		if err != nil {
			return nil, fmt.Errorf("failed to read body for %s: %w", urlString, err)
		}

		req.Body = io.NopCloser(bytes.NewReader(body))
		hash = sha256.Sum256(body)
	}

	req.Header.Add("Digest", "SHA-256="+base64.StdEncoding.EncodeToString(hash[:]))
	req.Header.Add("Date", time.Now().UTC().Format(http.TimeFormat))
	req.Header.Add("Host", req.URL.Host)

	publicKeyID, privateKey, err := key.Load(req.Context(), db, from)
	if err != nil {
		return nil, err
	}

	if err := signer.SignRequest(privateKey, publicKeyID, req, nil); err != nil {
		return nil, fmt.Errorf("failed to sign request for %s: %w", urlString, err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request to %s: %w", urlString, err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		body, err := io.ReadAll(io.LimitReader(resp.Body, s.Config.MaxRequestBodySize))
		resp.Body.Close()
		if err != nil {
			return resp, fmt.Errorf("failed to send request to %s: %d, %w", urlString, resp.StatusCode, err)
		}
		return resp, fmt.Errorf("failed to send request to %s: %d, %s", urlString, resp.StatusCode, string(body))
	}

	return resp, nil
}

// post sends a signed request to actor's inbox.
func (s *sender) post(ctx context.Context, log *slog.Logger, db *sql.DB, from *ap.Actor, key *key, followers partialFollowers, inbox string, body []byte) error {
	if inbox == "" {
		return fmt.Errorf("cannot send request to %s: empty URL", inbox)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, inbox, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to send request to %s: %w", inbox, err)
	}

	if req.URL.Host == s.Domain {
		log.Info("Skipping request", "inbox", inbox, "from", from.ID)
		return nil
	}

	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)
	if followers != nil {
		if err := followers.Digest(ctx, db, s.Domain, from, req); err != nil {
			log.Warn("Failed to add Collection-Synchronization header", "from", from.ID, "inbox", inbox, "error", err)
		}
	}

	resp, err := s.send(log, db, from, key, req)
	if err != nil {
		return fmt.Errorf("failed to send request to %s: %w", inbox, err)
	}
	defer resp.Body.Close()

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("failed to send request to %s: %w", inbox, err)
	}

	log.Info("Successfully sent message", "inbox", inbox, "body", string(respBody))
	return nil
}
