/*
Copyright 2023 - 2025 Dima Krasner

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

package fed

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand/v2"
	"net/http"
	"time"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/buildinfo"
	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/httpsig"
)

type sender struct {
	Domain string
	Config *cfg.Config
	client Client
	DB     *sql.DB
}

var userAgent = "tootik/" + buildinfo.Version

func (s *sender) send(keys [2]httpsig.Key, req *http.Request) (*http.Response, error) {
	urlString := req.URL.String()

	if req.URL.Scheme != "https" {
		return nil, fmt.Errorf("invalid scheme in %s: %s", urlString, req.URL.Scheme)
	}

	if req.URL.Host == "localhost" || req.URL.Host == "localhost.localdomain" || req.URL.Host == "127.0.0.1" || req.URL.Host == "::1" {
		return nil, fmt.Errorf("invalid host in %s: %s", urlString, req.URL.Host)
	}

	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Content-Type", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)

	slog.Debug("Sending request", "url", urlString)

	capabilities := ap.CavageDraftSignatures

	if err := s.DB.QueryRowContext(req.Context(), `select capabilities from servers where host = ?`, req.URL.Host).Scan(&capabilities); errors.Is(err, sql.ErrNoRows) {
		slog.Debug("Server capabilities are unknown", "url", urlString)
	} else if err != nil {
		return nil, fmt.Errorf("failed to query server capabilities for %s: %w", req.URL.Host, err)
	}

	if capabilities&ap.RFC9421Ed25519Signatures == 0 && req.Method == http.MethodPost && rand.Float32() > s.Config.Ed25519Threshold {
		slog.Debug("Randomly enabling RFC9421 with Ed25519", "server", req.URL.Host)
		capabilities = ap.RFC9421Ed25519Signatures
	} else if capabilities&ap.RFC9421RSASignatures == 0 && req.Method == http.MethodPost && rand.Float32() > s.Config.RFC9421Threshold {
		slog.Debug("Randomly enabling RFC9421 with RSA", "server", req.URL.Host)
		capabilities = ap.RFC9421RSASignatures
	}

	if capabilities&ap.RFC9421Ed25519Signatures > 0 {
		slog.Debug("Signing request using RFC9421 with Ed25519", "method", req.Method, "url", urlString, "key", keys[1].ID)

		if err := httpsig.SignRFC9421(req, keys[1], time.Now(), time.Time{}, httpsig.RFC9421DigestSHA256, "ed25519", nil); err != nil {
			return nil, fmt.Errorf("failed to sign request for %s: %w", urlString, err)
		}
	} else if capabilities&ap.RFC9421RSASignatures > 0 {
		slog.Debug("Signing request using RFC9421 with RSA", "method", req.Method, "url", urlString, "key", keys[0].ID)

		if err := httpsig.SignRFC9421(req, keys[0], time.Now(), time.Time{}, httpsig.RFC9421DigestSHA256, "rsa-v1_5-sha256", nil); err != nil {
			return nil, fmt.Errorf("failed to sign request for %s: %w", urlString, err)
		}
	} else if err := httpsig.Sign(req, keys[0], time.Now()); err != nil {
		slog.Debug("Signing request using draft-cavage-http-signatures", "method", req.Method, "url", urlString, "key", keys[0].ID)

		return nil, fmt.Errorf("failed to sign request for %s: %w", urlString, err)
	}

	resp, err := s.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to send request to %s: %w", urlString, err)
	}

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusAccepted {
		defer resp.Body.Close()

		if resp.ContentLength > s.Config.MaxResponseBodySize {
			return resp, fmt.Errorf("failed to send request to %s: %d", urlString, resp.StatusCode)
		}

		body, err := io.ReadAll(io.LimitReader(resp.Body, s.Config.MaxResponseBodySize))
		if err != nil {
			return resp, fmt.Errorf("failed to send request to %s: %d, %w", urlString, resp.StatusCode, err)
		}
		return resp, fmt.Errorf("failed to send request to %s: %d, %s", urlString, resp.StatusCode, string(body))
	}

	if _, err = s.DB.ExecContext(
		req.Context(),
		`INSERT INTO servers (host, capabilities) VALUES ($1, $2) ON CONFLICT(host) DO UPDATE SET capabilities = capabilities | $2, updated = UNIXEPOCH()`,
		req.URL.Host,
		capabilities,
	); err != nil {
		slog.Warn("Failed to record server capabilities", "server", req.URL.Host, "error", err)
	}

	return resp, nil
}

func (s *sender) Get(ctx context.Context, keys [2]httpsig.Key, url string) (*http.Response, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to send request to %s: %w", url, err)
	}

	req.Header.Set("Accept", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)

	return s.send(keys, req)
}
