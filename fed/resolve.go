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
	"github.com/dimkr/tootik/httpsig"
	"hash/crc32"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"path"
	"strings"
	"sync"
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
	sender
	BlockedDomains *BlockList
	locks          []sync.Mutex
}

var (
	ErrActorGone      = errors.New("actor is gone")
	ErrNoLocalActor   = errors.New("no such local user")
	ErrActorNotCached = errors.New("actor is not cached")
	ErrBlockedDomain  = errors.New("domain is blocked")
	ErrInvalidScheme  = errors.New("invalid scheme")
	ErrInvalidHost    = errors.New("invalid host")
	ErrInvalidID      = errors.New("invalid actor ID")
	ErrSuspendedActor = errors.New("actor is suspended")
	ErrYoungActor     = errors.New("actor is too young")
)

// NewResolver returns a new [Resolver].
func NewResolver(blockedDomains *BlockList, domain string, cfg *cfg.Config, client Client) *Resolver {
	r := Resolver{
		sender: sender{
			Domain: domain,
			Config: cfg,
			client: client,
		},
		BlockedDomains: blockedDomains,
		locks:          make([]sync.Mutex, cfg.MaxResolverRequests),
	}

	return &r
}

// ResolveID retrieves an actor object by its ID.
func (r *Resolver) ResolveID(ctx context.Context, log *slog.Logger, db *sql.DB, key httpsig.Key, id string, flags ap.ResolverFlag) (*ap.Actor, error) {
	u, err := url.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve %s: %w", id, err)
	}

	if u.Scheme != "https" {
		return nil, ErrInvalidScheme
	}

	var name string
	if flags&ap.InstanceActor == 0 {
		name = path.Base(u.Path)

		// strip the leading @ if URL follows the form https://a.b/@c
		if name[0] == '@' {
			name = name[1:]
		}
	} else {
		// in Mastodon, domain@domain leads to the "instance actor" (domain/actor) and it's discoverable through domain@domain
		name = u.Host
	}

	return r.Resolve(ctx, log, db, key, u.Host, name, flags)
}

// Resolve retrieves an actor object by host and name.
func (r *Resolver) Resolve(ctx context.Context, log *slog.Logger, db *sql.DB, key httpsig.Key, host, name string, flags ap.ResolverFlag) (*ap.Actor, error) {
	if actor, err := r.tryResolveOrCache(ctx, log, db, key, host, name, flags); err != nil {
		return nil, err
	} else if actor.Suspended {
		return nil, ErrSuspendedActor
	} else {
		return actor, nil
	}
}

func (r *Resolver) tryResolveOrCache(ctx context.Context, log *slog.Logger, db *sql.DB, key httpsig.Key, host, name string, flags ap.ResolverFlag) (*ap.Actor, error) {
	actor, cachedActor, err := r.tryResolve(ctx, log, db, key, host, name, flags)
	if err != nil && cachedActor != nil && cachedActor.Published != nil && time.Since(cachedActor.Published.Time) < r.Config.MinActorAge {
		log.Warn("Failed to update cached actor", "host", host, "name", name, "error", err)
		return nil, ErrYoungActor
	} else if err != nil && cachedActor != nil {
		log.Warn("Using old cache entry for actor", "host", host, "name", name, "error", err)
		return cachedActor, nil
	} else if actor == nil {
		return cachedActor, err
	} else if actor.Published != nil && time.Since(actor.Published.Time) < r.Config.MinActorAge {
		return nil, ErrYoungActor
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

func (r *Resolver) tryResolve(ctx context.Context, log *slog.Logger, db *sql.DB, key httpsig.Key, host, name string, flags ap.ResolverFlag) (*ap.Actor, *ap.Actor, error) {
	log.Debug("Resolving actor", "host", host, "name", name)

	if r.BlockedDomains != nil && r.BlockedDomains.Contains(host) {
		return nil, nil, ErrBlockedDomain
	}

	if name == "" {
		return nil, nil, fmt.Errorf("cannot resolve %s%s: empty name", name, host)
	}

	isLocal := host == r.Domain

	if !isLocal && flags&ap.Offline == 0 {
		locked := make(chan struct{}, 1)
		unlock := make(chan struct{}, 1)
		unlocked := make(chan struct{}, 1)

		lock := &r.locks[crc32.ChecksumIEEE([]byte(host+name))%uint32(len(r.locks))]
		go func() {
			lock.Lock()
			locked <- struct{}{}
			<-unlock
			lock.Unlock()
			unlocked <- struct{}{}
		}()

		defer func() {
			unlock <- struct{}{}
			<-unlocked
		}()

		select {
		case <-ctx.Done():
			return nil, nil, ctx.Err()

		case <-locked:
		}
	}

	var tmp ap.Actor
	var cachedActor *ap.Actor

	var updated, inserted int64
	var fetched sql.NullInt64
	var sinceLastUpdate time.Duration
	if err := db.QueryRowContext(ctx, `select actor, updated, fetched, inserted from persons where actor->>'$.preferredUsername' = $1 and host = $2`, name, host).Scan(&tmp, &updated, &fetched, &inserted); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, nil, fmt.Errorf("failed to fetch %s%s cache: %w", name, host, err)
	} else if err == nil {
		cachedActor = &tmp

		// fall back to insertion time if we don't have registration time
		if cachedActor.Published == nil {
			cachedActor.Published = &ap.Time{Time: time.Unix(inserted, 0)}
		}

		sinceLastUpdate = time.Since(time.Unix(updated, 0))
		if !isLocal && flags&ap.Offline == 0 && sinceLastUpdate > r.Config.ResolverCacheTTL && (!fetched.Valid || time.Since(time.Unix(fetched.Int64, 0)) >= r.Config.ResolverRetryInterval) {
			log.Info("Updating old cache entry for actor", "id", cachedActor.ID)
		} else {
			log.Debug("Resolved actor using cache", "id", cachedActor.ID)
			return nil, cachedActor, nil
		}
	}

	if isLocal {
		return nil, nil, fmt.Errorf("cannot resolve %s@%s: %w", name, host, ErrNoLocalActor)
	}

	if flags&ap.Offline != 0 {
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

	resp, err := r.send(log, key, req)
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

	if cachedActor != nil && profile != cachedActor.ID {
		return nil, cachedActor, fmt.Errorf("%s does not match %s", profile, cachedActor.ID)
	}

	req, err = http.NewRequestWithContext(ctx, http.MethodGet, profile, nil)
	if err != nil {
		return nil, cachedActor, fmt.Errorf("failed to send request to %s: %w", profile, err)
	}

	if req.URL.Host != host && !strings.HasSuffix(req.URL.Host, "."+host) {
		return nil, nil, fmt.Errorf("actor link host is %s: %w", req.URL.Host, ErrInvalidHost)
	}

	if !data.IsIDValid(req.URL) {
		return nil, nil, fmt.Errorf("cannot resolve %s: %w", profile, ErrInvalidID)
	}

	req.Header.Set("User-Agent", userAgent)
	req.Header.Add("Accept", "application/activity+json")

	resp, err = r.send(log, key, req)
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

	if _, err := db.ExecContext(
		ctx,
		`INSERT INTO persons(id, actor, fetched) VALUES($1, $2, UNIXEPOCH()) ON CONFLICT(id) DO UPDATE SET actor = $2, updated = UNIXEPOCH()`,
		actor.ID,
		string(body),
	); err != nil {
		return nil, cachedActor, fmt.Errorf("failed to cache %s: %w", actor.ID, err)
	}

	if actor.Published == nil && cachedActor != nil && cachedActor.Published != nil {
		actor.Published = cachedActor.Published
	} else if actor.Published == nil && (cachedActor == nil || cachedActor.Published == nil) {
		actor.Published = &ap.Time{Time: time.Now()}
	}

	return &actor, cachedActor, nil
}
