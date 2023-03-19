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
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/data"
	"github.com/go-ap/activitypub"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
	"net/url"
	"path"
	"strings"
)

func Resolve(ctx context.Context, db *sql.DB, sender *data.Object, actorID string) (*activitypub.Actor, error) {
	if sender == nil {
		log.WithFields(log.Fields{"actor": actorID}).Debug("Resolving actor")
	} else {
		log.WithFields(log.Fields{"actor": actorID, "for": sender.ID}).Debug("Resolving actor")
	}

	actor := activitypub.Actor{}

	o, err := data.Objects.GetByID(actorID, db)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("Failed to fetch %s cache: %w", actorID, err)
	} else if err == nil {
		if err := json.Unmarshal([]byte(o.Object), &actor); err != nil {
			return nil, fmt.Errorf("Failed to unmarshal %s cache: %w", actorID, err)
		}
		log.WithField("actor", actorID).Debug("Resolved actor using cache")
		return &actor, nil
	}

	prefix := fmt.Sprintf("https://%s/user/", cfg.Domain)
	if strings.HasPrefix(actorID, prefix) {
		return nil, fmt.Errorf("Cannot resolve %s: no such local user", actorID)
	}

	u, err := url.Parse(actorID)
	if err != nil {
		return nil, fmt.Errorf("Failed to parse %s: %w", actorID, err)
	}

	name := path.Base(u.Path)

	finger := fmt.Sprintf("%s://%s/.well-known/webfinger?resource=acct:%s@%s", u.Scheme, u.Host, name, u.Host)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, finger, nil)
	if err != nil {
		return nil, fmt.Errorf("Failed to fetch %s: %w", finger, err)
	}

	resp, err := send(db, sender, &actor, req)
	if err != nil {
		return nil, fmt.Errorf("Failed to fetch %s: %w", finger, err)
	}
	defer resp.Body.Close()

	log.WithField("url", finger).Info("Decoding response")

	var j map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&j); err != nil {
		return nil, fmt.Errorf("Failed to decode %s response: %w", finger, err)
	}
	log.WithField("url", finger).Info("Done decoding response")

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

		if t, ok := link["type"].(string); !ok || t != "application/activity+json" {
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

	req, err = http.NewRequestWithContext(ctx, http.MethodGet, profile, nil)
	if err != nil {
		return nil, fmt.Errorf("Failed to send request to %s: %w", profile, err)
	}
	req.Header.Add("Accept", "application/activity+json")

	resp, err = send(db, sender, &actor, req)
	if err != nil {
		return nil, fmt.Errorf("Failed to fetch %s: %w", profile, err)
	}
	defer resp.Body.Close()

	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("Failed to fetch %s: %w", profile, err)
	}

	if err := json.Unmarshal(body, &actor); err != nil {
		return nil, fmt.Errorf("Failed to fetch %s: %w", profile, err)
	}

	if err := data.Objects.Insert(db, &data.Object{
		ID:     actorID,
		Type:   string(activitypub.PersonType),
		Object: string(body),
	}); err != nil {
		return nil, fmt.Errorf("Failed to cache %s: %w", actorID, err)
	}

	return &actor, nil
}
