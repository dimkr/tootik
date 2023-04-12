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
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"strings"
	"time"
)

const resolverCacheTTL = time.Hour * 24 * 3

var goneError = errors.New("Actor is gone")

type Resolver struct {
	Log *log.Logger
}

func (r *Resolver) Resolve(ctx context.Context, db *sql.DB, from *ap.Actor, to string) (*ap.Actor, error) {
	u, err := url.Parse(to)
	if err != nil {
		return nil, fmt.Errorf("Cannot resolve %s: %w", to, err)
	}
	u.Fragment = ""

	return r.resolve(ctx, db, from, u.String(), u)
}

func (r *Resolver) deleteActor(ctx context.Context, db *sql.DB, id string) {
	if _, err := db.ExecContext(ctx, `delete from notes where author = ?`, id); err != nil {
		r.Log.WithField("id", id).WithError(err).Warn("Failed to delete notes by actor")
	}

	if _, err := db.ExecContext(ctx, `delete from follows where follower = $1 or followed = $1`, id); err != nil {
		r.Log.WithField("id", id).WithError(err).Warn("Failed to delete follows for actor")
	}

	if _, err := db.ExecContext(ctx, `delete from persons where id = ?`, id); err != nil {
		r.Log.WithField("id", id).WithError(err).Warn("Failed to delete actor")
	}
}

func (r *Resolver) resolve(ctx context.Context, db *sql.DB, from *ap.Actor, to string, u *url.URL) (*ap.Actor, error) {
	if from == nil {
		r.Log.WithField("to", to).Debug("Resolving actor")
	} else {
		r.Log.WithFields(log.Fields{"from": from.ID, "to": to}).Debug("Resolving actor")
	}

	isLocal := strings.HasPrefix(to, fmt.Sprintf("https://%s/", cfg.Domain))

	actor := ap.Actor{}
	update := false

	var actorString string
	var updated int64
	if err := db.QueryRowContext(ctx, `select actor, updated from persons where id = ?`, to).Scan(&actorString, &updated); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("Failed to fetch %s cache: %w", to, err)
	} else if err == nil {
		if !isLocal && time.Now().Sub(time.Unix(updated, 0)) > resolverCacheTTL {
			r.Log.WithField("to", to).Info("Updating old cache entry for actor")
			update = true
		} else {
			if err := json.Unmarshal([]byte(actorString), &actor); err != nil {
				return nil, fmt.Errorf("Failed to unmarshal %s cache: %w", to, err)
			}
			r.Log.WithField("to", to).Debug("Resolved actor using cache")
			return &actor, nil
		}
	}

	if isLocal {
		return nil, fmt.Errorf("Cannot resolve %s: no such local user", to)
	}

	name := path.Base(u.Path)

	finger := fmt.Sprintf("%s://%s/.well-known/webfinger?resource=acct:%s@%s", u.Scheme, u.Host, name, u.Host)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, finger, nil)
	if err != nil {
		return nil, fmt.Errorf("Failed to fetch %s: %w", finger, err)
	}

	resp, err := send(db, from, r, req)
	if err != nil {
		if resp != nil && (resp.StatusCode == http.StatusGone || resp.StatusCode == http.StatusNotFound) {
			r.Log.WithField("to", to).Warn("Actor is gone, deleting associated objects")
			r.deleteActor(ctx, db, to)
			return nil, fmt.Errorf("Failed to fetch %s: %w", finger, goneError)
		}

		return nil, fmt.Errorf("Failed to fetch %s: %w", finger, err)
	}
	defer resp.Body.Close()

	var j map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&j); err != nil {
		return nil, fmt.Errorf("Failed to decode %s response: %w", finger, err)
	}

	arr, ok := j["links"].([]any)
	if !ok {
		return nil, fmt.Errorf("No links in %s response", finger)
	}

	profile := ""

	for _, elem := range arr {
		link, ok := elem.(map[string]any)
		if !ok {
			continue
		}

		if rel, ok := link["rel"].(string); !ok || rel != "self" {
			continue
		}

		if t, ok := link["type"].(string); !ok || (t != "application/activity+json" && t != `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`) {
			continue
		}

		href, ok := link["href"].(string)
		if !ok || href == "" {
			continue
		}

		profile = href
		break
	}

	if profile == "" {
		return nil, fmt.Errorf("No profile link in %s response", finger)
	}

	if profile != to {
		r.Log.WithFields(log.Fields{"before": to, "after": profile}).Info("Replacing actor ID")
		to = profile

		if err := db.QueryRowContext(ctx, `select actor, updated from persons where id = ?`, to).Scan(&actorString, &updated); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("Failed to fetch %s cache: %w", to, err)
		} else if err == nil {
			if !isLocal && time.Now().Sub(time.Unix(updated, 0)) > resolverCacheTTL {
				r.Log.WithField("to", to).Info("Updating old cache entry for actor")
				update = true
			} else {
				if err := json.Unmarshal([]byte(actorString), &actor); err != nil {
					return nil, fmt.Errorf("Failed to unmarshal %s cache: %w", to, err)
				}
				r.Log.WithField("to", to).Debug("Resolved actor using cache")
				return &actor, nil
			}
		}
	}

	req, err = http.NewRequestWithContext(ctx, http.MethodGet, profile, nil)
	if err != nil {
		return nil, fmt.Errorf("Failed to send request to %s: %w", profile, err)
	}
	req.Header.Add("Accept", "application/activity+json")

	resp, err = send(db, from, r, req)
	if err != nil {
		return nil, fmt.Errorf("Failed to fetch %s: %w", profile, err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Failed to fetch %s: %w", profile, err)
	}

	if err := json.Unmarshal(body, &actor); err != nil {
		return nil, fmt.Errorf("Failed to unmarshal %s: %w", profile, err)
	}

	resolvedID := actor.ID
	if resolvedID == "" {
		return nil, fmt.Errorf("Failed to unmarshal %s: empty ID", profile)
	}
	if resolvedID != to {
		r.Log.WithFields(log.Fields{"before": to, "after": resolvedID}).Info("Replacing actor ID")
	}

	if update {
		if _, err := db.ExecContext(
			ctx,
			`UPDATE persons SET actor = ?, updated = UNIXEPOCH() WHERE id = ?`,
			string(body),
			resolvedID,
		); err != nil {
			return nil, fmt.Errorf("Failed to cache %s: %w", resolvedID, err)
		}
	} else if _, err := db.ExecContext(
		ctx,
		`INSERT INTO persons(id, hash, actor) VALUES(?,?,?)`,
		resolvedID,
		fmt.Sprintf("%x", sha256.Sum256([]byte(resolvedID))),
		string(body),
	); err != nil {
		return nil, fmt.Errorf("Failed to cache %s: %w", resolvedID, err)
	}

	return &actor, nil
}
