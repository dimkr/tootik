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
	"encoding/json"
	"errors"
	"fmt"
	"hash/crc32"
	"io"
	"log/slog"
	"net"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/data"
	"github.com/dimkr/tootik/httpsig"
	"github.com/dimkr/tootik/lock"
)

// Resolver retrieves actor objects given their ID.
// Actors are cached, updated periodically and deleted if gone from the remote server.
type Resolver struct {
	sender
	BlockedDomains *BlockList
	db             *sql.DB
	locks          []lock.Lock
}

var (
	ErrActorGone        = errors.New("actor is gone")
	ErrNoLocalActor     = errors.New("no such local user")
	ErrActorNotCached   = errors.New("actor is not cached")
	ErrBlockedDomain    = errors.New("domain is blocked")
	ErrInvalidScheme    = errors.New("invalid scheme")
	ErrInvalidHost      = errors.New("invalid host")
	ErrInvalidID        = errors.New("invalid actor ID")
	ErrSuspendedActor   = errors.New("actor is suspended")
	ErrYoungActor       = errors.New("actor is too young")
	ErrNotInstanceActor = errors.New("not application actor")
)

// NewResolver returns a new [Resolver].
func NewResolver(blockedDomains *BlockList, domain string, cfg *cfg.Config, client Client, db *sql.DB) *Resolver {
	r := Resolver{
		sender: sender{
			Domain: domain,
			Config: cfg,
			client: client,
		},
		BlockedDomains: blockedDomains,
		db:             db,
		locks:          make([]lock.Lock, cfg.MaxResolverRequests),
	}
	for i := 0; i < len(r.locks); i++ {
		r.locks[i] = lock.New()
	}

	return &r
}

// ResolveID retrieves an actor object by its ID.
func (r *Resolver) ResolveID(ctx context.Context, key httpsig.Key, id string, flags ap.ResolverFlag) (*ap.Actor, error) {
	if id == "" {
		return nil, errors.New("empty ID")
	}

	u, err := url.Parse(id)
	if err != nil {
		return nil, fmt.Errorf("cannot resolve %s: %w", id, err)
	}

	if u.Scheme != "https" {
		return nil, ErrInvalidScheme
	}

	if actor, err := r.validate(func() (*ap.Actor, *ap.Actor, error) { return r.tryResolveID(ctx, key, u, id, flags) }); err != nil {
		return nil, err
	} else if actor.Suspended {
		return nil, ErrSuspendedActor
	} else if flags&ap.InstanceActor != 0 && actor.Type != ap.Application && actor.Type != ap.Service {
		return nil, ErrNotInstanceActor
	} else {
		return actor, nil
	}
}

// Resolve retrieves an actor object by host and name.
func (r *Resolver) Resolve(ctx context.Context, key httpsig.Key, host, name string, flags ap.ResolverFlag) (*ap.Actor, error) {
	if actor, err := r.validate(func() (*ap.Actor, *ap.Actor, error) { return r.tryResolve(ctx, key, host, name, flags) }); err != nil {
		return nil, err
	} else if actor.Suspended {
		return nil, ErrSuspendedActor
	} else if flags&ap.InstanceActor != 0 && actor.Type != ap.Application && actor.Type != ap.Service {
		return nil, ErrNotInstanceActor
	} else {
		return actor, nil
	}
}

func (r *Resolver) validate(try func() (*ap.Actor, *ap.Actor, error)) (*ap.Actor, error) {
	actor, cachedActor, err := try()
	if err != nil && cachedActor != nil && cachedActor.Published != (ap.Time{}) && time.Since(cachedActor.Published.Time) < r.Config.MinActorAge {
		slog.Warn("Failed to update cached actor", "id", cachedActor.ID, "error", err)
		return nil, ErrYoungActor
	} else if err != nil && cachedActor != nil {
		slog.Warn("Using old cache entry for actor", "id", cachedActor.ID, "error", err)
		return cachedActor, nil
	} else if actor == nil {
		return cachedActor, err
	} else if actor.Published != (ap.Time{}) && time.Since(actor.Published.Time) < r.Config.MinActorAge {
		return nil, ErrYoungActor
	}
	return actor, err
}

func deleteActor(ctx context.Context, db *sql.DB, id string) {
	if _, err := db.ExecContext(ctx, `delete from notesfts where exists (select 1 from notes where notes.author = ? and notesfts.id = notes.id)`, id); err != nil {
		slog.Warn("Failed to delete notes by actor", "id", id, "error", err)
	}

	if _, err := db.ExecContext(ctx, `delete from shares where by = $1 or exists (select 1 from notes where notes.author = $1 and notes.id = shares.note)`, id); err != nil {
		slog.Warn("Failed to delete shares by actor", "id", id, "error", err)
	}

	if _, err := db.ExecContext(ctx, `delete from bookmarks where exists (select 1 from notes where notes.author = ? and notes.id = bookmarks.note)`, id); err != nil {
		slog.Warn("Failed to delete bookmarks by actor", "id", id, "error", err)
	}

	if _, err := db.ExecContext(ctx, `delete from feed where sharer->>'$.id' = ?`, id); err != nil {
		slog.Warn("Failed to delete shares by actor", "id", id, "error", err)
	}

	if _, err := db.ExecContext(ctx, `delete from feed where author->>'$.id' = ?`, id); err != nil {
		slog.Warn("Failed to delete shares by actor", "id", id, "error", err)
	}

	if _, err := db.ExecContext(ctx, `delete from notes where author = ?`, id); err != nil {
		slog.Warn("Failed to delete notes by actor", "id", id, "error", err)
	}

	if _, err := db.ExecContext(ctx, `delete from follows where follower = $1 or followed = $1`, id); err != nil {
		slog.Warn("Failed to delete follows for actor", "id", id, "error", err)
	}

	if _, err := db.ExecContext(ctx, `delete from persons where id = ?`, id); err != nil {
		slog.Warn("Failed to delete actor", "id", id, "error", err)
	}
}

func (r *Resolver) handleFetchFailure(ctx context.Context, fetched string, cachedActor *ap.Actor, sinceLastUpdate time.Duration, resp *http.Response, err error) (*ap.Actor, *ap.Actor, error) {
	if resp != nil && (resp.StatusCode == http.StatusGone || resp.StatusCode == http.StatusNotFound) {
		if cachedActor != nil {
			slog.Warn("Actor is gone, deleting associated objects", "id", cachedActor.ID)
			deleteActor(ctx, r.db, cachedActor.ID)
		}
		return nil, nil, fmt.Errorf("failed to fetch %s: %w", fetched, ErrActorGone)
	}

	var (
		urlError *url.Error
		opError  *net.OpError
		dnsError *net.DNSError
	)
	// if it's been a while since the last update and the server's domain is expired (NXDOMAIN), actor is gone
	if sinceLastUpdate > r.Config.MaxInstanceRecoveryTime && errors.As(err, &urlError) && errors.As(urlError.Err, &opError) && errors.As(opError.Err, &dnsError) && dnsError.IsNotFound {
		if cachedActor != nil {
			slog.Warn("Server is probably gone, deleting associated objects", "id", cachedActor.ID)
			deleteActor(ctx, r.db, cachedActor.ID)
		}
		return nil, nil, fmt.Errorf("failed to fetch %s: %w", fetched, err)
	}

	return nil, cachedActor, fmt.Errorf("failed to fetch %s: %w", fetched, err)
}

func (r *Resolver) tryResolve(ctx context.Context, key httpsig.Key, host, name string, flags ap.ResolverFlag) (*ap.Actor, *ap.Actor, error) {
	slog.Debug("Resolving actor", "host", host, "name", name)

	if r.BlockedDomains != nil && r.BlockedDomains.Contains(host) {
		return nil, nil, ErrBlockedDomain
	}

	if name == "" {
		return nil, nil, fmt.Errorf("cannot resolve %s%s: empty name", name, host)
	}

	isLocal := host == r.Domain

	var lockID uint32
	if !isLocal && flags&ap.Offline == 0 {
		lockID = crc32.ChecksumIEEE([]byte(host+name)) % uint32(len(r.locks))
		lock := r.locks[lockID]
		if err := lock.Lock(ctx); err != nil {
			return nil, nil, err
		}
		defer lock.Unlock()
	}

	var tmp ap.Actor
	var cachedActor *ap.Actor

	var updated, inserted int64
	var fetched sql.NullInt64
	var sinceLastUpdate time.Duration
	var err error
	if flags&ap.GroupActor == 0 {
		err = r.db.QueryRowContext(ctx, `select json(actor), updated, fetched, inserted from persons where actor->>'$.preferredUsername' = $1 and host = $2 order by actor->>'$.type' = 'Group' limit 1`, name, host).Scan(&tmp, &updated, &fetched, &inserted)
	} else {
		err = r.db.QueryRowContext(ctx, `select json(actor), updated, fetched, inserted from persons where actor->>'$.preferredUsername' = $1 and host = $2 and actor->>'$.type' = 'Group'`, name, host).Scan(&tmp, &updated, &fetched, &inserted)
	}
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, nil, fmt.Errorf("failed to fetch %s%s cache: %w", name, host, err)
	} else if err == nil {
		cachedActor = &tmp

		// fall back to insertion time if we don't have registration time
		if cachedActor.Published == (ap.Time{}) {
			cachedActor.Published = ap.Time{Time: time.Unix(inserted, 0)}
		}

		sinceLastUpdate = time.Since(time.Unix(updated, 0))
		if !isLocal && flags&ap.Offline == 0 && sinceLastUpdate >= r.Config.ResolverCacheTTL && (!fetched.Valid || time.Since(time.Unix(fetched.Int64, 0)) >= r.Config.ResolverRetryInterval) {
			slog.Info("Updating old cache entry for actor", "id", cachedActor.ID)
		} else {
			slog.Debug("Resolved actor using cache", "id", cachedActor.ID)
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
		altLockID := crc32.ChecksumIEEE([]byte(cachedActor.ID)) % uint32(len(r.locks))
		if altLockID != lockID {
			lock := r.locks[altLockID]
			if err := lock.Lock(ctx); err != nil {
				return nil, nil, err
			}
			defer lock.Unlock()
		}

		if _, err := r.db.ExecContext(
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
	req.Header.Add("Accept", "application/json")

	resp, err := r.send(key, req)
	if err != nil {
		return r.handleFetchFailure(ctx, finger, cachedActor, sinceLastUpdate, resp, err)
	}
	defer resp.Body.Close()

	if resp.ContentLength > r.Config.MaxResponseBodySize {
		return nil, cachedActor, fmt.Errorf("failed to decode %s response: response is too big", finger)
	}

	var webFingerResponse webFingerResponse
	if err := json.NewDecoder(io.LimitReader(resp.Body, r.Config.MaxResponseBodySize)).Decode(&webFingerResponse); err != nil {
		return nil, cachedActor, fmt.Errorf("failed to decode %s response: %w", finger, err)
	}

	// assumption: there can be only one actor with the same name, per type
	actors := data.OrderedMap[ap.ActorType, string]{}

	for _, link := range webFingerResponse.Links {
		if link.Rel != "self" {
			continue
		}

		if link.Type != "application/activity+json" && link.Type != `application/ld+json; profile="https://www.w3.org/ns/activitystreams"` {
			continue
		}

		if link.Href != "" {
			actors.Store(link.Properties.Type, link.Href)
			break
		}
	}

	for actorType, id := range actors.All() {
		// look for a Group actor if the same name belongs to multiple actors, otherwise a non-Group one
		if len(actors) > 1 && ((flags&ap.GroupActor == 0 && actorType == ap.Group) || (flags&ap.GroupActor > 0 && actorType != ap.Group)) {
			continue
		}

		if cachedActor != nil && id != cachedActor.ID {
			return nil, cachedActor, fmt.Errorf("%s does not match %s", id, cachedActor.ID)
		}

		return r.fetchActor(ctx, key, host, id, cachedActor, sinceLastUpdate)
	}

	return nil, cachedActor, fmt.Errorf("no profile link in %s response", finger)
}

func (r *Resolver) tryResolveID(ctx context.Context, key httpsig.Key, u *url.URL, id string, flags ap.ResolverFlag) (*ap.Actor, *ap.Actor, error) {
	slog.Debug("Resolving actor", "id", id)

	if r.BlockedDomains != nil && r.BlockedDomains.Contains(u.Host) {
		return nil, nil, ErrBlockedDomain
	}

	isLocal := u.Host == r.Domain

	var lockID uint32
	if !isLocal && flags&ap.Offline == 0 {
		lockID = crc32.ChecksumIEEE([]byte(id)) % uint32(len(r.locks))
		lock := r.locks[lockID]
		if err := lock.Lock(ctx); err != nil {
			return nil, nil, err
		}
		defer lock.Unlock()
	}

	var tmp ap.Actor
	var cachedActor *ap.Actor

	var updated, inserted int64
	var fetched sql.NullInt64
	var sinceLastUpdate time.Duration
	if err := r.db.QueryRowContext(ctx, `select json(actor), updated, fetched, inserted from persons where id = $1 or actor->>'$.publicKey.id' = $1 or actor->>'$.assertionMethod[0].id' = $1`, id).Scan(&tmp, &updated, &fetched, &inserted); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, nil, fmt.Errorf("failed to fetch %s cache: %w", id, err)
	} else if err == nil {
		cachedActor = &tmp

		// fall back to insertion time if we don't have registration time
		if cachedActor.Published == (ap.Time{}) {
			cachedActor.Published = ap.Time{Time: time.Unix(inserted, 0)}
		}

		sinceLastUpdate = time.Since(time.Unix(updated, 0))
		if !isLocal && flags&ap.Offline == 0 && sinceLastUpdate > r.Config.ResolverCacheTTL && (!fetched.Valid || time.Since(time.Unix(fetched.Int64, 0)) >= r.Config.ResolverRetryInterval) {
			slog.Info("Updating old cache entry for actor", "id", cachedActor.ID)
		} else {
			slog.Debug("Resolved actor using cache", "id", cachedActor.ID)
			return nil, cachedActor, nil
		}
	}

	if isLocal {
		return nil, nil, fmt.Errorf("cannot resolve %s: %w", id, ErrNoLocalActor)
	}

	if flags&ap.Offline != 0 {
		return nil, nil, fmt.Errorf("cannot resolve %s: %w", id, ErrActorNotCached)
	}

	if cachedActor != nil {
		if cachedActor.ID != id {
			altLockID := crc32.ChecksumIEEE([]byte(cachedActor.ID)) % uint32(len(r.locks))
			if altLockID != lockID {
				lock := r.locks[altLockID]
				if err := lock.Lock(ctx); err != nil {
					return nil, nil, err
				}
				defer lock.Unlock()
			}
		}

		if _, err := r.db.ExecContext(
			ctx,
			`UPDATE persons SET fetched = UNIXEPOCH() WHERE id = ?`,
			cachedActor.ID,
		); err != nil {
			return nil, cachedActor, fmt.Errorf("failed to update last fetch time for %s: %w", cachedActor.ID, err)
		}

		return r.fetchActor(ctx, key, u.Host, cachedActor.ID, cachedActor, sinceLastUpdate)
	}

	return r.fetchActor(ctx, key, u.Host, id, nil, sinceLastUpdate)
}

func (r *Resolver) fetchActor(ctx context.Context, key httpsig.Key, host, profile string, cachedActor *ap.Actor, sinceLastUpdate time.Duration) (*ap.Actor, *ap.Actor, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, profile, nil)
	if err != nil {
		return nil, cachedActor, fmt.Errorf("failed to send request to %s: %w", profile, err)
	}

	if req.URL.Host != host && !strings.HasSuffix(req.URL.Host, "."+host) {
		return nil, cachedActor, fmt.Errorf("actor link host is %s: %w", req.URL.Host, ErrInvalidHost)
	}

	if !data.IsIDValid(req.URL) {
		return nil, cachedActor, fmt.Errorf("cannot resolve %s: %w", profile, ErrInvalidID)
	}

	req.Header.Set("User-Agent", userAgent)
	req.Header.Add("Accept", "application/activity+json")

	resp, err := r.send(key, req)
	if err != nil {
		return r.handleFetchFailure(ctx, profile, cachedActor, sinceLastUpdate, resp, err)
	}
	defer resp.Body.Close()

	if resp.ContentLength > r.Config.MaxResponseBodySize {
		return nil, cachedActor, fmt.Errorf("failed to fetch %s: response is too big", profile)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, r.Config.MaxResponseBodySize))
	if err != nil {
		return nil, cachedActor, fmt.Errorf("failed to fetch %s: %w", profile, err)
	}

	var actor ap.Actor
	if err := json.Unmarshal(body, &actor); err != nil {
		return nil, cachedActor, fmt.Errorf("failed to unmarshal %s: %w", profile, err)
	}

	if !(actor.ID == profile || actor.PublicKey.ID == profile || (len(actor.AssertionMethod) > 0 && actor.AssertionMethod[0].ID == profile)) {
		return nil, cachedActor, fmt.Errorf("%s does not match %s", actor.ID, profile)
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, cachedActor, fmt.Errorf("failed to cache %s: %w", actor.ID, err)
	}

	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO persons(id, actor, fetched) VALUES ($1, JSONB($2), UNIXEPOCH()) ON CONFLICT(id) DO UPDATE SET actor = JSONB($2), updated = UNIXEPOCH()`,
		actor.ID,
		string(body),
	); err != nil {
		return nil, cachedActor, fmt.Errorf("failed to cache %s: %w", actor.ID, err)
	}

	if _, err := tx.ExecContext(
		ctx,
		`UPDATE feed SET author = JSONB(?) WHERE author->>'$.id' = ?`,
		string(body),
		actor.ID,
	); err != nil {
		return nil, cachedActor, fmt.Errorf("failed to cache %s: %w", actor.ID, err)
	}

	if _, err := tx.ExecContext(
		ctx,
		`UPDATE feed SET sharer = JSONB(?) WHERE sharer->>'$.id' = ?`,
		string(body),
		actor.ID,
	); err != nil {
		return nil, cachedActor, fmt.Errorf("failed to cache %s: %w", actor.ID, err)
	}

	if err := tx.Commit(); err != nil {
		return nil, cachedActor, fmt.Errorf("failed to cache %s: %w", actor.ID, err)
	}

	if actor.Published == (ap.Time{}) && cachedActor != nil && cachedActor.Published != (ap.Time{}) {
		actor.Published = cachedActor.Published
	} else if actor.Published == (ap.Time{}) && (cachedActor == nil || cachedActor.Published == (ap.Time{})) {
		actor.Published = ap.Time{Time: time.Now()}
	}

	return &actor, cachedActor, nil
}
