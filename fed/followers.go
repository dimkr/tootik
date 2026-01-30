/*
Copyright 2024 - 2026 Dima Krasner

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
	"crypto/ed25519"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/danger"
	"github.com/dimkr/tootik/data"
	"github.com/dimkr/tootik/httpsig"
)

type partialFollowers map[string]map[string]string

type Syncer struct {
	Domain   string
	Config   *cfg.Config
	DB       *sql.DB
	Resolver *Resolver
	Keys     [2]httpsig.Key
	Inbox    ap.Inbox
}

type followersDigest struct {
	Followed string
	URL      string
	Digest   string
	Inbox    ap.Inbox
}

var collectionSynchronizationHeaderRegex = regexp.MustCompile(`\b([^"=]+)="([^"]+)"`)

func fetchFollowers(ctx context.Context, db *sql.DB, followed, host string) (ap.Audience, error) {
	var followers ap.Audience

	rows, err := db.QueryContext(ctx, `SELECT follower FROM follows WHERE followed = ? AND follower LIKE 'https://' || ? || '/%' AND accepted = 1`, followed, host)
	if err != nil {
		return followers, err
	}
	defer rows.Close()

	if err := data.ScanRows(
		rows,
		func(follower string) bool {
			followers.Add(follower)
			return true
		},
		func(err error) bool {
			return false
		},
	); err != nil {
		return followers, err
	}

	return followers, nil
}

func digestFollowers(ctx context.Context, db *sql.DB, followed, host string) (string, error) {
	rows, err := db.QueryContext(ctx, `SELECT follower FROM follows WHERE followed = ? AND follower LIKE 'https://' || ? || '/%' AND accepted = 1`, followed, host)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var digest [sha256.Size]byte

	if err := data.ScanRows(
		rows,
		func(follower string) bool {
			hash := sha256.Sum256(danger.Bytes(follower))
			for i := range sha256.Size {
				digest[i] ^= hash[i]
			}
			return true
		},
		func(err error) bool {
			return false
		},
	); err != nil {
		return "", err
	}

	return fmt.Sprintf("%x", digest), nil
}

func (f partialFollowers) Digest(ctx context.Context, db *sql.DB, domain string, actor *ap.Actor, host string) (string, error) {
	byActor, ok := f[actor.ID]
	if ok {
		if header, ok := byActor[host]; ok && header != "" {
			return header, nil
		}
	} else {
		byActor = map[string]string{}
		f[actor.ID] = byActor
	}

	digest, err := digestFollowers(ctx, db, actor.ID, host)
	if err != nil {
		return "", err
	}

	var header string
	if m := ap.GatewayURLRegex.FindStringSubmatch(actor.ID); m != nil {
		header = fmt.Sprintf(`collectionId="%s", url="https://%s/.well-known/apgateway/did:key:%s/actor/followers_synchronization", digest="%s"`, actor.Followers, domain, m[1], digest)
	} else {
		header = fmt.Sprintf(`collectionId="%s", url="https://%s/followers_synchronization/%s", digest="%s"`, actor.Followers, domain, actor.PreferredUsername, digest)
	}
	byActor[host] = header
	return header, nil
}

func (l *Listener) handleFollowers(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("username")

	_, sender, err := l.verifyRequest(r, nil, ap.InstanceActor, l.AppActorKeys)
	if err != nil {
		slog.Warn("Failed to verify followers request", "error", err)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	u, err := url.Parse(sender.ID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	rows, err := l.DB.QueryContext(r.Context(), `SELECT follower FROM follows WHERE followed = 'https://' || ? || '/user/' || ? AND follower LIKE 'https://' || ? || '/%' AND accepted = 1`, l.Domain, name, u.Host)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}
	defer rows.Close()

	var items ap.Audience

	if err := data.ScanRows(
		rows,
		func(follower string) bool {
			items.Add(follower)
			return true
		},
		func(err error) bool {
			return false
		},
	); err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	collection, err := json.Marshal(ap.Collection{
		Context:      "https://www.w3.org/ns/activitystreams",
		ID:           fmt.Sprintf("https://%s/followers/%s?domain=%s", l.Domain, name, u.Host),
		Type:         ap.OrderedCollection,
		OrderedItems: items,
	})
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	slog.Info("Received followers request", "sender", sender.ID, "username", name, "host", u.Host, "count", len(items.OrderedMap))

	w.Header().Set("Content-Type", `application/activity+json; charset=utf-8`)
	w.Write(collection)
}

func (l *Listener) saveFollowersDigest(ctx context.Context, sender *ap.Actor, header string) error {
	var collection, digest, partial string
	for _, m := range collectionSynchronizationHeaderRegex.FindAllStringSubmatch(header, 3) {
		switch m[1] {
		case "collectionId":
			collection = m[2]
		case "digest":
			digest = m[2]
		case "url":
			partial = m[2]
		}
	}

	if collection == "" || digest == "" || partial == "" {
		return errors.New("at least one required attribute is empty")
	}

	if collection != sender.Followers {
		return fmt.Errorf("collection is not sender's followers: %s != %s", collection, sender.Followers)
	}

	if !strings.HasPrefix(sender.Followers, "https://") {
		return errors.New("invalid followers collection")
	}

	if len(digest) != sha256.Size*2 {
		return errors.New("invalid digest length")
	}

	u, err := url.Parse(sender.ID)
	if err != nil {
		return fmt.Errorf("invalid actor ID: %w", err)
	}
	host := u.Host

	u, err = url.Parse(partial)
	if err != nil {
		return fmt.Errorf("invalid partial followers collection: %w", err)
	}
	if u.Host != host {
		return errors.New("partial collection host does not match actor")
	}

	if _, err := l.DB.ExecContext(
		ctx,
		`INSERT INTO follows_sync(actor, url, digest) VALUES($1, $2, $3) ON CONFLICT(actor) DO UPDATE SET url = $2, digest = $3, changed = CASE WHEN digest = $3 THEN changed ELSE UNIXEPOCH() END`,
		sender.ID,
		partial,
		digest,
	); err != nil {
		return err
	}

	return nil
}

func (d *followersDigest) Sync(ctx context.Context, domain string, cfg *cfg.Config, db *sql.DB, resolver *Resolver, keys [2]httpsig.Key) error {
	if digest, err := digestFollowers(ctx, db, d.Followed, domain); err != nil {
		return err
	} else if digest == d.Digest {
		slog.Debug("Follower collections are synchronized", "followed", d.Followed)
		return nil
	}

	local, err := fetchFollowers(ctx, db, d.Followed, domain)
	if err != nil {
		return err
	}

	slog.Info("Synchronizing followers", "followed", d.Followed)

	resp, err := resolver.Get(ctx, keys, d.URL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.ContentLength > cfg.MaxResponseBodySize {
		return errors.New("response is too big")
	}

	var body []byte
	if resp.ContentLength >= 0 {
		body = make([]byte, resp.ContentLength)
		_, err = io.ReadFull(resp.Body, body)
	} else {
		body, err = io.ReadAll(io.LimitReader(resp.Body, cfg.MaxResponseBodySize))
	}
	if err != nil {
		return err
	}

	var remote struct {
		OrderedItems ap.Audience `json:"orderedItems"`
	}
	if err := json.Unmarshal(body, &remote); err != nil {
		return err
	}

	for follower := range local.Keys() {
		if remote.OrderedItems.Contains(follower) {
			continue
		}

		slog.Info("Found unknown local follow", "followed", d.Followed, "follower", follower)

		if _, err := db.ExecContext(
			ctx,
			`UPDATE follows SET accepted = NULL WHERE follower = ? AND followed = ?`,
			follower,
			d.Followed,
		); err != nil {
			slog.Warn("Failed to remove local follow", "followed", d.Followed, "follower", follower, "error", err)
		}
	}

	prefix := fmt.Sprintf("https://%s/", domain)

	for follower := range remote.OrderedItems.Keys() {
		if local.Contains(follower) {
			continue
		}

		if !strings.HasPrefix(follower, prefix) {
			continue
		}

		slog.Info("Found unknown remote follow", "followed", d.Followed, "follower", follower)

		var actor ap.Actor
		var ed25519PrivKey []byte
		if err := db.QueryRowContext(ctx, `SELECT JSON(persons.actor), persons.ed25519privkey FROM persons WHERE id = ? AND persons.ed25519privkey IS NOT NULL`, follower).Scan(&actor, &ed25519PrivKey); errors.Is(err, sql.ErrNoRows) {
			slog.Info("Follower does not exist", "followed", d.Followed, "follower", follower)
			continue
		} else if err != nil {
			slog.Warn("Failed to fetch actor of unknown remote follow", "followed", d.Followed, "follower", follower, "error", err)
			continue
		}

		var followID string
		if err := db.QueryRowContext(ctx, `SELECT id FROM follows WHERE follower = ? AND followed = ?`, follower, d.Followed).Scan(&followID); err != nil && errors.Is(err, sql.ErrNoRows) {
			followID, err = d.Inbox.NewID(actor.ID, "follow")
			if err != nil {
				slog.Warn("Failed to generate fake follow ID", "followed", d.Followed, "follower", follower, "error", err)
				continue
			}
			slog.Warn("Using fake follow ID to remove unknown remote follow", "followed", d.Followed, "follower", follower, "id", followID)
		} else if err != nil {
			slog.Warn("Failed to fetch follow ID of unknown remote follow", "followed", d.Followed, "follower", follower, "error", err)
			continue
		}

		if err := d.Inbox.Unfollow(ctx, &actor, httpsig.Key{ID: actor.AssertionMethod[0].ID, PrivateKey: ed25519.NewKeyFromSeed(ed25519PrivKey)}, d.Followed, followID); err != nil {
			slog.Warn("Failed to remove remote follow", "followed", d.Followed, "follower", follower, "error", err)
		}
	}

	return nil
}

func (s *Syncer) ProcessBatch(ctx context.Context) (int, error) {
	if _, err := s.DB.ExecContext(ctx, `DELETE FROM follows_sync WHERE NOT EXISTS (SELECT 1 FROM persons WHERE persons.id = follows_sync.actor)`); err != nil {
		return 0, err
	}

	rows, err := s.DB.QueryContext(
		ctx,
		`SELECT actor, url, digest FROM follows_sync WHERE changed <= $1 ORDER BY changed LIMIT $2`,
		time.Now().Add(-s.Config.FollowersSyncInterval).Unix(),
		s.Config.FollowersSyncBatchSize,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch followers to sync: %w", err)
	}

	jobs := make([]followersDigest, 0, s.Config.FollowersSyncBatchSize)

	for rows.Next() {
		job := followersDigest{
			Inbox: s.Inbox,
		}
		if err := rows.Scan(&job.Followed, &job.URL, &job.Digest); err != nil {
			slog.Error("Failed to scan digest", "error", err)
			continue
		}
		jobs = append(jobs, job)
	}
	rows.Close()

	for _, job := range jobs {
		if err := job.Sync(ctx, s.Domain, s.Config, s.DB, s.Resolver, s.Keys); err != nil {
			slog.Warn("Failed to sync followers", "actor", job.Followed, "error", err)
		}

		if _, err := s.DB.ExecContext(ctx, `UPDATE follows_sync SET changed = UNIXEPOCH() WHERE actor = ?`, job.Followed); err != nil {
			return 0, fmt.Errorf("failed to update last sync time for %s: %w", job.Followed, err)
		}
	}

	return len(jobs), nil
}

func (s *Syncer) Run(ctx context.Context) error {
	for {
		if n, err := s.ProcessBatch(ctx); err != nil {
			return err
		} else if n == 0 {
			break
		}
	}

	return nil
}
