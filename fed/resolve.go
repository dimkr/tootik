/*
Copyright 2023 Dima Krasner

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
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/cfg"
	"golang.org/x/sync/semaphore"
	"hash/crc32"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

const (
	resolverCacheTTL        = time.Hour * 24 * 3
	resolverMaxIdleConns    = 128
	maxInstanceRecoveryTime = time.Hour * 24 * 30
	resolverIdleConnTimeout = time.Minute
)

type webFingerResponse struct {
	Subject string `json:"subject"`
	Links   []struct {
		Rel  string `json:"rel"`
		Type string `json:"type"`
		Href string `json:"href"`
	} `json:"links"`
}

type Resolver struct {
	Client         http.Client
	BlockedDomains *BlockList
	locks          []*semaphore.Weighted
}

var (
	ErrActorGone      = errors.New("actor is gone")
	ErrActorNotCached = errors.New("actor is not cached")
	ErrBlockedDomain  = errors.New("domain is blocked")
	ErrInvalidScheme  = errors.New("invalid scheme")
	ErrRedirect       = errors.New("redirects are forbidden")
)

func NewResolver(blockedDomains *BlockList) *Resolver {
	transport := http.Transport{
		MaxIdleConns:    resolverMaxIdleConns,
		IdleConnTimeout: resolverIdleConnTimeout,
	}
	r := Resolver{
		Client: http.Client{
			Transport: &transport,
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				return ErrRedirect
			},
		},
		BlockedDomains: blockedDomains,
		locks:          make([]*semaphore.Weighted, cfg.MaxResolverRequests),
	}
	for i := 0; i < len(r.locks); i++ {
		r.locks[i] = semaphore.NewWeighted(1)
	}

	return &r
}

func (r *Resolver) Resolve(ctx context.Context, log *slog.Logger, db *sql.DB, from *ap.Actor, to string, offline bool) (*ap.Actor, error) {
	u, err := url.Parse(to)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve %s: %w", to, err)
	}

	if u.Scheme != "https" {
		return nil, ErrInvalidScheme
	}

	if r.BlockedDomains != nil {
		if blocked := r.BlockedDomains.Contains(u.Host); blocked {
			return nil, ErrBlockedDomain
		}
	}

	u.Fragment = ""

	return r.resolve(ctx, log, db, from, u.String(), u, offline)
}

func deleteActor(ctx context.Context, log *slog.Logger, db *sql.DB, id string) {
	if _, err := db.ExecContext(ctx, `delete from notes where author = ?`, id); err != nil {
		log.Warn("Failed to delete notes by actor", "id", id, "error", err)
	}

	if _, err := db.ExecContext(ctx, `delete from follows where follower = $1 or followed = $1`, id); err != nil {
		log.Warn("Failed to delete follows for actor", "id", id, "error", err)
	}

	if _, err := db.ExecContext(ctx, `delete from persons where id = ?`, id); err != nil {
		log.Warn("Failed to delete actor", "id", id, "error", err)
	}
}

func (r *Resolver) resolve(ctx context.Context, log *slog.Logger, db *sql.DB, from *ap.Actor, to string, u *url.URL, offline bool) (*ap.Actor, error) {
	if from == nil {
		log.Debug("Resolving actor", "to", to)
	} else {
		log.Debug("Resolving actor", "from", from.ID, "to", to)
	}

	isLocal := strings.HasPrefix(to, fmt.Sprintf("https://%s/", cfg.Domain))

	if !isLocal && !offline {
		lock := r.locks[crc32.ChecksumIEEE([]byte(to))%uint32(len(r.locks))]
		if err := lock.Acquire(ctx, 1); err != nil {
			return nil, err
		}
		defer lock.Release(1)
	}

	actor := ap.Actor{}
	update := false

	var actorString string
	var updated int64
	var sinceLastUpdate time.Duration
	if err := db.QueryRowContext(ctx, `select actor, updated from persons where id = ?`, to).Scan(&actorString, &updated); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("failed to fetch %s cache: %w", to, err)
	} else if err == nil {
		sinceLastUpdate = time.Since(time.Unix(updated, 0))
		if !isLocal && !offline && sinceLastUpdate > resolverCacheTTL {
			log.Info("Updating old cache entry for actor", "to", to)
			update = true
		} else {
			if err := json.Unmarshal([]byte(actorString), &actor); err != nil {
				return nil, fmt.Errorf("failed to unmarshal %s cache: %w", to, err)
			}
			log.Debug("Resolved actor using cache", "to", to)
			return &actor, nil
		}
	}

	if isLocal {
		return nil, fmt.Errorf("cannot resolve %s: no such local user", to)
	}

	if offline {
		return nil, fmt.Errorf("cannot resolve %s: %w", to, ErrActorNotCached)
	}

	name := path.Base(u.Path)

	// strip the leading @ if URL follows the form https://a.b/@c
	if name[0] == '@' {
		name = name[1:]
	}

	if name == "" {
		return nil, fmt.Errorf("cannot resolve %s: empty name", to)
	}

	finger := fmt.Sprintf("%s://%s/.well-known/webfinger?resource=acct:%s@%s", u.Scheme, u.Host, name, u.Host)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, finger, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch %s: %w", finger, err)
	}

	resp, err := send(log, db, from, r, req)
	if err != nil {
		if resp != nil && (resp.StatusCode == http.StatusGone || resp.StatusCode == http.StatusNotFound) {
			log.Warn("Actor is gone, deleting associated objects", "to", to)
			deleteActor(ctx, log, db, to)
			return nil, fmt.Errorf("failed to fetch %s: %w", finger, ErrActorGone)
		}

		var (
			urlError *url.Error
			opError  *net.OpError
			dnsError *net.DNSError
		)
		// if it's been a while since the last update and the server's domain is expired (NXDOMAIN), actor is gone
		if sinceLastUpdate > maxInstanceRecoveryTime && errors.As(err, &urlError) && errors.As(urlError.Err, &opError) && errors.As(opError.Err, &dnsError) && dnsError.IsNotFound {
			log.Warn("Server is probably gone, deleting associated objects", "to", to)
			deleteActor(ctx, log, db, to)
		}

		return nil, fmt.Errorf("failed to fetch %s: %w", finger, err)
	}
	defer resp.Body.Close()

	var webFingerResponse webFingerResponse
	if err := json.NewDecoder(resp.Body).Decode(&webFingerResponse); err != nil {
		return nil, fmt.Errorf("failed to decode %s response: %w", finger, err)
	}

	profile := ""

	for _, link := range webFingerResponse.Links {
		if link.Rel != "self" {
			continue
		}

		if link.Type != "application/activity+json" && link.Type != `application/ld+json; profile="https://www.w3.org/ns/activitystreams"` {
			continue
		}

		if link.Href != "" {
			profile = link.Href
			break
		}
	}

	if profile == "" {
		return nil, fmt.Errorf("no profile link in %s response", finger)
	}

	if profile != to {
		log.Info("Replacing actor ID", "before", to, "after", profile)
		to = profile

		if err := db.QueryRowContext(ctx, `select actor, updated from persons where id = ?`, to).Scan(&actorString, &updated); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("failed to fetch %s cache: %w", to, err)
		} else if err == nil {
			if !isLocal && time.Since(time.Unix(updated, 0)) > resolverCacheTTL {
				log.Info("Updating old cache entry for actor", "to", to)
				update = true
			} else {
				if err := json.Unmarshal([]byte(actorString), &actor); err != nil {
					return nil, fmt.Errorf("failed to unmarshal %s cache: %w", to, err)
				}
				log.Debug("Resolved actor using cache", "to", to)
				return &actor, nil
			}
		}
	}

	req, err = http.NewRequestWithContext(ctx, http.MethodGet, profile, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to send request to %s: %w", profile, err)
	}
	req.Header.Add("Accept", "application/activity+json")

	resp, err = send(log, db, from, r, req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch %s: %w", profile, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, maxBodySize))
	if err != nil {
		return nil, fmt.Errorf("failed to fetch %s: %w", profile, err)
	}

	if err := json.Unmarshal(body, &actor); err != nil {
		return nil, fmt.Errorf("failed to unmarshal %s: %w", profile, err)
	}

	resolvedID := actor.ID
	if resolvedID == "" {
		return nil, fmt.Errorf("failed to unmarshal %s: empty ID", profile)
	}
	if resolvedID != to {
		log.Info("Replacing actor ID", "before", to, "after", resolvedID)
	}

	if update {
		if _, err := db.ExecContext(
			ctx,
			`UPDATE persons SET actor = ?, updated = UNIXEPOCH() WHERE id = ?`,
			string(body),
			resolvedID,
		); err != nil {
			return nil, fmt.Errorf("failed to cache %s: %w", resolvedID, err)
		}
	} else if _, err := db.ExecContext(
		ctx,
		`INSERT INTO persons(id, hash, actor) VALUES(?,?,?)`,
		resolvedID,
		fmt.Sprintf("%x", sha256.Sum256([]byte(resolvedID))),
		string(body),
	); err != nil {
		return nil, fmt.Errorf("failed to cache %s: %w", resolvedID, err)
	}

	return &actor, nil
}
