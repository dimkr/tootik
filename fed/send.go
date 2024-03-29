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
	"database/sql"
	"errors"
	"fmt"
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/buildinfo"
	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/httpsig"
	"io"
	"log/slog"
	"net/http"
)

type sender struct {
	Domain string
	Config *cfg.Config
	client Client
}

var (
	userAgent     = "tootik/" + buildinfo.Version
	ErrLocalInbox = errors.New("local inbox")
)

func (s *sender) send(log *slog.Logger, key httpsig.Key, req *http.Request) (*http.Response, error) {
	urlString := req.URL.String()

	if req.URL.Scheme != "https" {
		return nil, fmt.Errorf("invalid scheme in %s: %s", urlString, req.URL.Scheme)
	}

	if req.URL.Host == "localhost" || req.URL.Host == "localhost.localdomain" || req.URL.Host == "127.0.0.1" || req.URL.Host == "::1" {
		return nil, fmt.Errorf("invalid host in %s: %s", urlString, req.URL.Host)
	}

	req.Header.Set("Content-Type", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)

	log.Debug("Sending request", "url", urlString)

	if err := httpsig.Sign(req, key); err != nil {
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
func (s *sender) post(ctx context.Context, log *slog.Logger, db *sql.DB, from *ap.Actor, key httpsig.Key, followersDigest, inbox string, body []byte) error {
	if inbox == "" {
		return fmt.Errorf("cannot send request to %s: empty URL", inbox)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, inbox, bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("failed to send request to %s: %w", inbox, err)
	}

	if req.URL.Host == s.Domain {
		return ErrLocalInbox
	}

	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)
	if followersDigest != "" {
		req.Header.Set("Collection-Synchronization", followersDigest)
	}

	resp, err := s.send(log, key, req)
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

func (s *sender) get(ctx context.Context, log *slog.Logger, db *sql.DB, key httpsig.Key, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to send request to %s: %w", url, err)
	}

	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Accept", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)

	return s.send(log, key, req)
}
