/*
Copyright 2024 Dima Krasner

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
	"github.com/dimkr/tootik/httpsig"
	"github.com/dimkr/tootik/outbox"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
)

type partialFollowers struct {
	sync.Mutex
	cache map[string]string
}

type Syncer struct {
	Domain   string
	Config   *cfg.Config
	Log      *slog.Logger
	DB       *sql.DB
	Resolver *Resolver
	Key      httpsig.Key
}

type followersDigest struct {
	Followed string
	URL      string
	Digest   string
}

var followersSyncRegex = regexp.MustCompile(`\b([^"=]+)="([^"]+)"`)

func fetchFollowers(ctx context.Context, db *sql.DB, followed, host string) (ap.Audience, error) {
	var followers ap.Audience

	rows, err := db.QueryContext(ctx, `SELECT follower FROM follows WHERE followed = ? AND follower LIKE 'https://' || ? || '/' || '%' AND accepted = 1`, followed, host)
	if err != nil {
		return followers, err
	}
	defer rows.Close()

	for rows.Next() {
		var follower string
		if err := rows.Scan(&follower); err != nil {
			return followers, err
		}
		followers.Add(follower)
	}

	return followers, nil
}

func digestFollowers(ctx context.Context, db *sql.DB, followed, host string) (string, error) {
	rows, err := db.QueryContext(ctx, `SELECT follower FROM follows WHERE followed = ? AND follower LIKE 'https://' || ? || '/' || '%' AND accepted = 1`, followed, host)
	if err != nil {
		return "", err
	}
	defer rows.Close()

	var digest [sha256.Size]byte
	for rows.Next() {
		var follower string
		if err := rows.Scan(&follower); err != nil {
			return "", err
		}
		hash := sha256.Sum256([]byte(follower))
		for i := range sha256.Size {
			digest[i] ^= hash[i]
		}
	}

	return fmt.Sprintf("%x", digest), nil
}

func (f *partialFollowers) Digest(ctx context.Context, db *sql.DB, domain string, actor *ap.Actor, req *http.Request) error {
	f.Lock()
	defer f.Unlock()

	if header, ok := f.cache[req.URL.Host]; ok && header != "" {
		req.Header.Set("Collection-Synchronization", header)
		return nil
	}

	digest, err := digestFollowers(ctx, db, actor.ID, req.URL.Host)
	if err != nil {
		return err
	}

	header := fmt.Sprintf(`collectionId="%s", url="https://%s/followers_synchronization/%s", digest="%s"`, actor.Followers, domain, actor.PreferredUsername, digest)
	f.cache[req.URL.Host] = header
	req.Header.Set("Collection-Synchronization", header)
	return nil
}

func (l *Listener) handleFollowers(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("username")

	sender, err := verify(r.Context(), l.Domain, l.Config, l.Log, r, nil, l.DB, l.Resolver, l.ActorKey, ap.InstanceActor)
	if err != nil {
		l.Log.Warn("Failed to verify followers request", "error", err)
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	u, err := url.Parse(sender.ID)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	rows, err := l.DB.QueryContext(r.Context(), `SELECT follower FROM follows WHERE followed = 'https://' || ? || '/user/' || ? AND follower LIKE 'https://' || ? || '/' || '%'`, l.Domain, name, u.Host)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var items ap.Audience

	for rows.Next() {
		var follower string
		if err := rows.Scan(&follower); err != nil {
			rows.Close()
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		items.Add(follower)
	}
	rows.Close()

	collection, err := json.Marshal(map[string]any{
		"@context":     "https://www.w3.org/ns/activitystreams",
		"id":           fmt.Sprintf("https://%s/followers/%s?domain=%s", l.Domain, name, u.Host),
		"type":         "OrderedCollection",
		"orderedItems": items,
	})
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	l.Log.Info("Received followers request", "sender", sender.ID, "username", name, "host", u.Host, "count", len(items.OrderedMap))

	w.Header().Set("Content-Type", `application/activity+json; charset=utf-8`)
	w.Write([]byte(collection))
}

func (l *Listener) saveFollowersDigest(ctx context.Context, sender *ap.Actor, header string) error {
	var collection, digest, partial string
	for _, m := range followersSyncRegex.FindAllStringSubmatch(header, 3) {
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
		return errors.New("collection is not sender's followers")
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

func (d *followersDigest) Sync(ctx context.Context, domain string, cfg *cfg.Config, log *slog.Logger, db *sql.DB, resolver *Resolver, key httpsig.Key) error {
	if digest, err := digestFollowers(ctx, db, d.Followed, domain); err != nil {
		return err
	} else if err == nil && digest == d.Digest {
		log.Debug("Follower collections are synchronized", "followed", d.Followed)
		return nil
	}

	local, err := fetchFollowers(ctx, db, d.Followed, domain)
	if err != nil {
		return err
	}

	log.Info("Synchronizing followers", "followed", d.Followed)

	resp, err := resolver.get(ctx, log, db, key, d.URL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var remote struct {
		OrderedItems ap.Audience `json:"orderedItems"`
	}
	if err := json.NewDecoder(io.LimitReader(resp.Body, cfg.MaxRequestBodySize)).Decode(&remote); err != nil {
		return err
	}

	local.Range(func(follower string, _ struct{}) bool {
		if remote.OrderedItems.Contains(follower) {
			return true
		}

		log.Info("Found unknown local follow", "followed", d.Followed, "follower", follower)

		if _, err := db.ExecContext(
			ctx,
			`UPDATE follows SET accepted = 0 WHERE follower = ? AND followed = ?`,
			follower,
			d.Followed,
		); err != nil {
			log.Warn("Failed to remove local follow", "followed", d.Followed, "follower", follower, "error", err)
		}

		return true
	})

	prefix := fmt.Sprintf("https://%s/", domain)

	remote.OrderedItems.Range(func(follower string, _ struct{}) bool {
		if local.Contains(follower) {
			return true
		}

		if !strings.HasPrefix(follower, prefix) {
			return true
		}

		log.Info("Found unknown remote follow", "followed", d.Followed, "follower", follower)

		var exists int
		if err := db.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM persons WHERE id = ?)`, follower).Scan(&exists); err != nil {
			log.Warn("Failed to check if follower exists", "followed", d.Followed, "follower", follower, "error", err)
			return true
		} else if exists == 0 {
			log.Info("Follower does not exist", "followed", d.Followed, "follower", follower)
			return true
		}

		var followID string
		if err := db.QueryRowContext(ctx, `SELECT id FROM follows WHERE follower = ? AND followed = ?`, follower, d.Followed).Scan(&followID); err != nil && errors.Is(err, sql.ErrNoRows) {
			followID = fmt.Sprintf("https://%s/follow/%x", domain, sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%d", follower, d.Followed, time.Now().UnixNano()))))
			log.Warn("Using fake follow ID to remove unknown remote follow", "followed", d.Followed, "follower", follower, "id", followID)
		} else if err != nil {
			log.Warn("Failed to fetch follow ID of unknown remote follow", "followed", d.Followed, "follower", follower, "error", err)
			return true
		}

		if err := outbox.Unfollow(ctx, domain, log, db, follower, d.Followed, followID); err != nil {
			log.Warn("Failed to remove remote follow", "followed", d.Followed, "follower", follower, "error", err)
		}

		return true
	})

	return nil
}

func (s *Syncer) processBatch(ctx context.Context) (int, error) {
	if _, err := s.DB.ExecContext(ctx, `DELETE FROM follows_sync WHERE NOT EXISTS (SELECT 1 FROM persons WHERE persons.id = follows_sync.actor)`); err != nil {
		return 0, err
	}

	rows, err := s.DB.QueryContext(
		ctx,
		`SELECT actor, url, digest FROM follows_sync WHERE changed < $1 ORDER BY changed LIMIT $2`,
		time.Now().Add(-s.Config.FollowersSyncInterval).Unix(),
		s.Config.FollowersSyncBatchSize,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch followers to sync: %w", err)
	}

	jobs := make([]followersDigest, 0, s.Config.FollowersSyncBatchSize)

	for rows.Next() {
		var job followersDigest
		if err := rows.Scan(&job.Followed, &job.URL, &job.Digest); err != nil {
			s.Log.Error("Failed to scan digest", "error", err)
			continue
		}
		jobs = append(jobs, job)
	}
	rows.Close()

	for _, job := range jobs {
		if err := job.Sync(ctx, s.Domain, s.Config, s.Log, s.DB, s.Resolver, s.Key); err != nil {
			s.Log.Warn("Failed to sync followers", "actor", job.Followed, "error", err)
		}

		if _, err := s.DB.ExecContext(ctx, `UPDATE follows_sync SET changed = UNIXEPOCH() WHERE actor = ?`, job.Followed); err != nil {
			return 0, fmt.Errorf("failed to update last sync time for %s: %w", job.Followed, err)
		}
	}

	return len(jobs), nil
}

func (s *Syncer) Run(ctx context.Context) error {
	for {
		if n, err := s.processBatch(ctx); err != nil {
			return err
		} else if n == 0 {
			break
		}
	}

	return nil
}
