/*
Copyright 2023, 2024 Dima Krasner

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
	"encoding/json"
	"errors"
	"fmt"
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/data"
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

type webFingerResponse struct {
	Subject string `json:"subject"`
	Links   []struct {
		Rel  string `json:"rel"`
		Type string `json:"type"`
		Href string `json:"href"`
	} `json:"links"`
}

// Resolver retrieves actor objects given their ID.
// Actors are cached, updated periodically and deleted if gone from the remote server.
type Resolver struct {
	BlockedDomains *BlockList
	Domain         string
	Config         *cfg.Config
	locks          []*semaphore.Weighted
	client         Client
}

var (
	ErrActorGone      = errors.New("actor is gone")
	ErrNoLocalActor   = errors.New("no such local user")
	ErrActorNotCached = errors.New("actor is not cached")
	ErrBlockedDomain  = errors.New("domain is blocked")
	ErrInvalidScheme  = errors.New("invalid scheme")
	ErrInvalidHost    = errors.New("invalid host")
	ErrInvalidID      = errors.New("invalid actor ID")
)

// NewResolver returns a new [Resolver].
func NewResolver(blockedDomains *BlockList, domain string, cfg *cfg.Config, client Client) *Resolver {
	r := Resolver{
		BlockedDomains: blockedDomains,
		Domain:         domain,
		Config:         cfg,
		locks:          make([]*semaphore.Weighted, cfg.MaxResolverRequests),
		client:         client,
	}
	for i := 0; i < len(r.locks); i++ {
		r.locks[i] = semaphore.NewWeighted(1)
	}

	return &r
}

// ResolveID retrieves an actor object by its ID.
func (r *Resolver) ResolveID(ctx context.Context, log *slog.Logger, db *sql.DB, from *ap.Actor, id string, offline bool) (*ap.Actor, error) {
	u, err := url.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve %s: %w", id, err)
	}

	if u.Scheme != "https" {
		return nil, ErrInvalidScheme
	}

	name := path.Base(u.Path)

	// strip the leading @ if URL follows the form https://a.b/@c
	if name[0] == '@' {
		name = name[1:]
	}

	return r.Resolve(ctx, log, db, from, u.Host, name, offline)
}

// Resolve retrieves an actor object by host and name.
func (r *Resolver) Resolve(ctx context.Context, log *slog.Logger, db *sql.DB, from *ap.Actor, host, name string, offline bool) (*ap.Actor, error) {
	actor, cachedActor, err := r.resolve(ctx, log, db, from, host, name, offline)
	if err != nil && cachedActor != nil {
		log.Warn("Using old cache entry for actor", "host", host, "name", name, "error", err)
		return cachedActor, nil
	} else if actor == nil {
		return cachedActor, err
	}
	return actor, err
}

func deleteActor(ctx context.Context, log *slog.Logger, db *sql.DB, id string) {
	if _, err := db.ExecContext(ctx, `delete from notesfts where id in (select id from notes where author = ?)`, id); err != nil {
		log.Warn("Failed to delete notes by actor", "id", id, "error", err)
	}

	if _, err := db.ExecContext(ctx, `delete from shares where by = $1 or note in (select id from notes where author = $1)`, id); err != nil {
		log.Warn("Failed to delete shares by actor", "id", id, "error", err)
	}

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

func (r *Resolver) resolve(ctx context.Context, log *slog.Logger, db *sql.DB, from *ap.Actor, host, name string, offline bool) (*ap.Actor, *ap.Actor, error) {
	if from == nil {
		log.Debug("Resolving actor", "host", host, "name", name)
	} else {
		log.Debug("Resolving actor", "from", from.ID, "host", host, "name", name)
	}

	if r.BlockedDomains != nil && r.BlockedDomains.Contains(host) {
		return nil, nil, ErrBlockedDomain
	}

	if name == "" {
		return nil, nil, fmt.Errorf("cannot resolve %s%s: empty name", name, host)
	}

	isLocal := host == r.Domain

	if !isLocal && !offline {
		lock := r.locks[crc32.ChecksumIEEE([]byte(host+name))%uint32(len(r.locks))]
		if err := lock.Acquire(ctx, 1); err != nil {
			return nil, nil, err
		}
		defer lock.Release(1)
	}

	var tmp ap.Actor
	var cachedActor *ap.Actor
	update := false

	var updated int64
	var fetched sql.NullInt64
	var sinceLastUpdate time.Duration
	if err := db.QueryRowContext(ctx, `select actor, updated, fetched from persons where actor->>'preferredUsername' = $1 and host = $2`, name, host).Scan(&tmp, &updated, &fetched); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, nil, fmt.Errorf("failed to fetch %s%s cache: %w", name, host, err)
	} else if err == nil {
		cachedActor = &tmp
		sinceLastUpdate = time.Since(time.Unix(updated, 0))
		if !isLocal && !offline && sinceLastUpdate > r.Config.ResolverCacheTTL && (!fetched.Valid || time.Since(time.Unix(fetched.Int64, 0)) >= r.Config.ResolverRetryInterval) {
			log.Info("Updating old cache entry for actor", "id", cachedActor.ID)
			update = true
		} else {
			log.Debug("Resolved actor using cache", "id", cachedActor.ID)
			return nil, cachedActor, nil
		}
	}

	if isLocal {
		return nil, nil, fmt.Errorf("cannot resolve %s@%s: %w", name, host, ErrNoLocalActor)
	}

	if offline {
		return nil, nil, fmt.Errorf("cannot resolve %s@%s: %w", name, host, ErrActorNotCached)
	}

	if cachedActor != nil {
		if _, err := db.ExecContext(
			ctx,
			`UPDATE persons SET fetched = UNIXEPOCH() WHERE id = ?`,
			cachedActor.ID,
		); err != nil {
			return nil, cachedActor, fmt.Errorf("failed to update last fetch time for %s: %w", cachedActor.ID, err)
		}
	}

	finger := fmt.Sprintf("https://%s/.well-known/webfinger?resource=acct:%s@%s", host, name, host)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, finger, nil)
	if err != nil {
		return nil, cachedActor, fmt.Errorf("failed to fetch %s: %w", finger, err)
	}
	req.Header.Set("User-Agent", userAgent)

	resp, err := r.send(log, db, from, req)
	if err != nil {
		if resp != nil && (resp.StatusCode == http.StatusGone || resp.StatusCode == http.StatusNotFound) {
			if cachedActor != nil {
				log.Warn("Actor is gone, deleting associated objects", "id", cachedActor.ID)
				deleteActor(ctx, log, db, cachedActor.ID)
			}
			return nil, nil, fmt.Errorf("failed to fetch %s: %w", finger, ErrActorGone)
		}

		var (
			urlError *url.Error
			opError  *net.OpError
			dnsError *net.DNSError
		)
		// if it's been a while since the last update and the server's domain is expired (NXDOMAIN), actor is gone
		if sinceLastUpdate > r.Config.MaxInstanceRecoveryTime && errors.As(err, &urlError) && errors.As(urlError.Err, &opError) && errors.As(opError.Err, &dnsError) && dnsError.IsNotFound {
			if cachedActor != nil {
				log.Warn("Server is probably gone, deleting associated objects", "id", cachedActor.ID)
				deleteActor(ctx, log, db, cachedActor.ID)
			}
			return nil, nil, fmt.Errorf("failed to fetch %s: %w", finger, err)
		}

		return nil, cachedActor, fmt.Errorf("failed to fetch %s: %w", finger, err)
	}
	defer resp.Body.Close()

	var webFingerResponse webFingerResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, r.Config.MaxRequestBodySize)).Decode(&webFingerResponse); err != nil {
		return nil, cachedActor, fmt.Errorf("failed to decode %s response: %w", finger, err)
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
		return nil, cachedActor, fmt.Errorf("no profile link in %s response", finger)
	}

	req, err = http.NewRequestWithContext(ctx, http.MethodGet, profile, nil)
	if err != nil {
		return nil, cachedActor, fmt.Errorf("failed to send request to %s: %w", profile, err)
	}

	if req.URL.Host != host && !strings.HasSuffix(req.URL.Host, "."+host) {
		return nil, nil, fmt.Errorf("actor link host is %s: %w", profile, ErrInvalidHost)
	}

	if !data.IsIDValid(req.URL) {
		return nil, nil, fmt.Errorf("cannot resolve %s: %w", profile, ErrInvalidID)
	}

	req.Header.Set("User-Agent", userAgent)
	req.Header.Add("Accept", "application/activity+json")

	resp, err = r.send(log, db, from, req)
	if err != nil {
		return nil, cachedActor, fmt.Errorf("failed to fetch %s: %w", profile, err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(io.LimitReader(resp.Body, r.Config.MaxRequestBodySize))
	if err != nil {
		return nil, cachedActor, fmt.Errorf("failed to fetch %s: %w", profile, err)
	}

	var actor ap.Actor
	if err := json.Unmarshal(body, &actor); err != nil {
		return nil, cachedActor, fmt.Errorf("failed to unmarshal %s: %w", profile, err)
	}

	if actor.ID != profile {
		return nil, cachedActor, fmt.Errorf("%s does not match %s", actor.ID, profile)
	}

	if update {
		if _, err := db.ExecContext(
			ctx,
			`UPDATE persons SET actor = ?, updated = UNIXEPOCH() WHERE id = ?`,
			string(body),
			actor.ID,
		); err != nil {
			return nil, cachedActor, fmt.Errorf("failed to cache %s: %w", actor.ID, err)
		}
	} else if _, err := db.ExecContext(
		ctx,
		`INSERT INTO persons(id, actor, fetched) VALUES(?,?,UNIXEPOCH())`,
		actor.ID,
		string(body),
	); err != nil {
		return nil, cachedActor, fmt.Errorf("failed to cache %s: %w", actor.ID, err)
	}

	return &actor, cachedActor, nil
}
