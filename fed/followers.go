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
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

type followers map[string]string

type orderedCollection struct {
	Context      string      `json:"@context"`
	ID           string      `json:"id"`
	Type         string      `json:"type"`
	OrderedItems ap.Audience `json:"orderedItems"`
}

type Syncer struct {
	Domain   string
	Config   *cfg.Config
	Log      *slog.Logger
	DB       *sql.DB
	Resolver *Resolver
	Actor    *ap.Actor
}

type syncJob struct {
	ActorID string
	URL     string
	Digest  string
}

var followersSyncRegex = regexp.MustCompile(`\b([^"=]+)="([^"]+)"`)

func fetchFollowers(ctx context.Context, db *sql.DB, actorID, host string) (*ap.Audience, error) {
	rows, err := db.QueryContext(ctx, `select follower from follows where followed = ? and follower like 'https://' || ? || '/' || '%' and accepted = 1`, actorID, host)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var followers ap.Audience
	for rows.Next() {
		var follower string
		if err := rows.Scan(&follower); err != nil {
			return nil, err
		}
		followers.Add(follower)
	}

	return &followers, nil
}

func digestFollowers(followers *ap.Audience) string {
	var digest [sha256.Size]byte

	followers.Range(func(follower string, _ struct{}) bool {
		hash := sha256.Sum256([]byte(follower))
		for i := range sha256.Size {
			digest[i] ^= hash[i]
		}
		return true
	})

	return fmt.Sprintf("%x", digest)
}

func (f followers) Digest(ctx context.Context, db *sql.DB, domain string, actor *ap.Actor, req *http.Request) error {
	if header, ok := f[req.URL.Host]; ok && header != "" {
		req.Header.Set("Collection-Synchronization", header)
		return nil
	}

	followers, err := fetchFollowers(ctx, db, actor.ID, req.URL.Host)
	if err != nil {
		return err
	}

	header := fmt.Sprintf(`collectionId="%s", url="https://%s/followers_synchronization/%s", digest="%s"`, actor.Followers, domain, actor.PreferredUsername, digestFollowers(followers))
	f[req.URL.Host] = header
	req.Header.Set("Collection-Synchronization", header)
	return nil
}

func (l *Listener) handleFollowers(w http.ResponseWriter, r *http.Request) {
	name := r.PathValue("username")

	sender, err := verify(r.Context(), l.Domain, l.Log, r, l.DB, l.Resolver, l.Actor, true)
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

	rows, err := l.DB.QueryContext(r.Context(), `select follower from follows where followed = 'https://' || ? || '/user/' || ? and follower like 'https://' || ? || '/' || '%'`, l.Domain, name, u.Host)
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

	collection, err := json.Marshal(orderedCollection{
		Context:      "https://www.w3.org/ns/activitystreams",
		ID:           fmt.Sprintf("https://%s/followers/%s?domain=%s", l.Domain, name, u.Host),
		Type:         "OrderedCollection",
		OrderedItems: items,
	})
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	l.Log.Info("Received followers request", "username", name, "host", u.Host, "response", collection)

	// TODO: remove
	w.WriteHeader(http.StatusInternalServerError)
	return

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

	if collection == "" && digest == "" && partial == "" {
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

	u, err := url.Parse(sender.Followers)
	if err != nil {
		return fmt.Errorf("invalid followers collection %s: %w", sender.Followers, err)
	}
	host := u.Host

	u, err = url.Parse(partial)
	if err != nil {
		return fmt.Errorf("invalid partial followers collection: %w", err)
	}
	if u.Host != host {
		return errors.New("partial collection does not match collection host")
	}

	if _, err := l.DB.ExecContext(
		ctx,
		`INSERT INTO follows_sync(actor, url, digest) VALUES($1, $2, $3) ON CONFLICT(actor) DO UPDATE SET url = $2, digest = $3, updated = UNIXEPOCH()`,
		sender.ID,
		partial,
		digest,
	); err != nil {
		return err
	}

	return nil
}

func (j *syncJob) Run(ctx context.Context, domain string, cfg *cfg.Config, log *slog.Logger, db *sql.DB, resolver *Resolver, from *ap.Actor) error {
	if _, err := db.ExecContext(
		ctx,
		`UPDATE follows_sync SET fetched = UNIXEPOCH() WHERE actor = ?`,
		j.ActorID,
	); err != nil {
		return fmt.Errorf("failed to update last fetch time for %s: %w", j.URL, err)
	}

	local, err := fetchFollowers(ctx, db, j.ActorID, domain)
	if err != nil {
		return err
	}

	if digestFollowers(local) == j.Digest {
		return nil
	}

	var key key
	resp, err := resolver.get(ctx, log, db, from, &key, j.URL)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	var remote orderedCollection
	if err := json.NewDecoder(io.LimitReader(resp.Body, cfg.MaxRequestBodySize)).Decode(&remote); err != nil {
		return err
	}

	local.Range(func(follower string, _ struct{}) bool {
		if !remote.OrderedItems.Contains(follower) {
			if _, err := db.ExecContext(
				ctx,
				`DELETE FROM follows WHERE follower = ? AND followed = ?`,
				follower,
				j.ActorID,
			); err != nil {
				log.Info("Got followers collection", "actor", j.ActorID, "followeres", remote.OrderedItems, "error", err)
			}
		}
		return true
	})

	log.Info("Got followers collection", "actor", j.ActorID, "followeres", remote.OrderedItems)

	return nil
}

func (s *Syncer) processBatch(ctx context.Context) (int, error) {
	rows, err := s.DB.QueryContext(
		ctx,
		`
			select actor, url, digest from follows_sync where fetched < $1 order by fetched limit $2
		`,
		time.Now().Add(-s.Config.FollowersSyncRetryInterval).Unix(),
		s.Config.FollowersSyncBatchSize,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch followers to sync: %w", err)
	}

	jobs := make([]syncJob, 0, s.Config.FollowersSyncBatchSize)

	for rows.Next() {
		var job syncJob
		if err := rows.Scan(&job.ActorID, &job.URL, &job.Digest); err != nil {
			s.Log.Error("Failed to scan digest", "error", err)
			continue
		}
		jobs = append(jobs, job)
	}
	rows.Close()

	succeeded := 0
	for _, job := range jobs {
		if err := job.Run(ctx, s.Domain, s.Config, s.Log, s.DB, s.Resolver, s.Actor); err != nil {
			s.Log.Warn("Failed to sync followers", "actor", job.ActorID, "error", err)
		} else {
			succeeded += 1
		}
	}

	return succeeded, nil
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
