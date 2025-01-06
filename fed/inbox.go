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
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/dimkr/tootik/ap"
	"io"
	"log/slog"
	"net/http"
	"net/url"
)

func (l *Listener) isForwardedActivity(activity *ap.Activity, sender *ap.Actor) (bool, error) {
	activityUrl, err := url.Parse(activity.ID)
	if err != nil {
		return false, err
	}

	actorUrl, err := url.Parse(activity.Actor)
	if err != nil {
		return false, err
	}

	senderUrl, err := url.Parse(sender.ID)
	if err != nil {
		return false, err
	}

	if activityUrl.Host == l.Domain {
		return false, errors.New("invalid activity host")
	}

	if actorUrl.Host == l.Domain {
		return false, errors.New("invalid actor host")
	}

	if senderUrl.Host == l.Domain {
		return false, errors.New("invalid sender host")
	}

	return activityUrl.Host != senderUrl.Host, nil
}

func (l *Listener) fetchObject(ctx context.Context, id string) (bool, []byte, error) {
	resp, err := l.Resolver.Get(ctx, l.ActorKey, id)
	if err != nil {
		if resp != nil && (resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusGone) {
			return false, nil, err
		}
		return true, nil, err
	}
	defer resp.Body.Close()

	if resp.ContentLength > l.Config.MaxRequestBodySize {
		return true, nil, fmt.Errorf("object is too big: %d", resp.ContentLength)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, l.Config.MaxRequestBodySize))
	if err != nil {
		return true, nil, err
	}

	return true, body, nil
}

func (l *Listener) handleInbox(w http.ResponseWriter, r *http.Request) {
	receiver := r.PathValue("username")

	var registered int
	if err := l.DB.QueryRowContext(r.Context(), `select exists (select 1 from persons where actor->>'$.preferredUsername' = ? and host = ?)`, receiver, l.Domain).Scan(&registered); err != nil {
		slog.Warn("Failed to check if receiving user exists", "receiver", receiver, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	} else if registered == 0 {
		slog.Debug("Receiving user does not exist", "receiver", receiver)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	if r.ContentLength > l.Config.MaxRequestBodySize {
		slog.Warn("Ignoring big request", "size", r.ContentLength)
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		return
	}

	rawActivity, err := io.ReadAll(io.LimitReader(r.Body, l.Config.MaxRequestBodySize))
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var activity ap.Activity
	if err := json.Unmarshal(rawActivity, &activity); err != nil {
		slog.Warn("Failed to unmarshal activity", "body", string(rawActivity), "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	r.Body = io.NopCloser(bytes.NewReader(rawActivity))

	// if actor is deleted, ignore this activity if we don't know this actor
	var flags ap.ResolverFlag
	if activity.Type == ap.Delete {
		flags |= ap.Offline
	}

	sender, err := l.verify(r, rawActivity, flags)
	if err != nil {
		if errors.Is(err, ErrActorGone) {
			w.WriteHeader(http.StatusOK)
			return
		}
		if errors.Is(err, ErrActorNotCached) {
			slog.Debug("Ignoring Delete activity for unknown actor", "error", err)
			w.WriteHeader(http.StatusOK)
			return
		}
		if errors.Is(err, ErrBlockedDomain) {
			slog.Debug("Failed to verify activity", "activity", activity.ID, "type", activity.Type, "error", err)
		} else {
			slog.Warn("Failed to verify activity", "activity", activity.ID, "type", activity.Type, "error", err)
		}
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	/*
		if an activity wasn't sent by an actor on the same server, we must fetch it instead of trusting the sender to
		pass the activity as-is
	*/
	forwarded, err := l.isForwardedActivity(&activity, sender)
	if err != nil {
		slog.Warn("Failed to determine whether or not activity is forwarded", "activity", activity.ID, "sender", sender.ID, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var queued any = string(rawActivity)

	if forwarded {
		id := activity.ID
		if activity.Type == ap.Delete {
			switch o := activity.Object.(type) {
			case *ap.Object:
				id = o.ID
			case string:
				id = o
			default:
				slog.Warn("Ignoring invalid forwarded Delete activity", "activity", activity.ID, "sender", sender.ID)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
		}

		slog.Info("Fetching forwarded object", "activity", activity.ID, "id", id, "sender", sender.ID)

		if exists, fetched, err := l.fetchObject(r.Context(), id); !exists && activity.Type == ap.Delete {
			queued = &ap.Activity{
				Type:   ap.Delete,
				Object: id,
			}
		} else if err == nil && exists && activity.Type == ap.Delete {
			slog.Warn("Ignoring forwarded Delete activity for existing object", "activity", activity.ID, "id", id, "sender", sender.ID)
			w.WriteHeader(http.StatusBadRequest)
			return
		} else if err != nil {
			slog.Warn("Failed to fetch forwarded object", "activity", activity.ID, "id", id, "sender", sender.ID, "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		} else if activity.Type == ap.Update {
			queued = string(fetched)

			var tmp ap.Activity
			if err := json.Unmarshal([]byte(fetched), &tmp); err != nil {
				var post ap.Object
				if err := json.Unmarshal([]byte(fetched), &post); err != nil {
					slog.Warn("Ignoring invalid forwarded Update activity", "activity", activity.ID, "sender", sender.ID, "error", err)
					w.WriteHeader(http.StatusBadRequest)
					return
				}

				slog.Info("Wrapping forwarded Update activity", "activity", activity.ID, "sender", sender.ID)
				queued = &ap.Activity{
					Type:   ap.Update,
					Object: &post,
				}
			}
		} else {
			queued = string(fetched)
		}
	}

	if _, err = l.DB.ExecContext(
		r.Context(),
		`INSERT OR IGNORE INTO inbox (sender, activity, raw) VALUES(?,?,?)`,
		sender.ID,
		queued,
		string(rawActivity),
	); err != nil {
		slog.Error("Failed to insert activity", "sender", sender.ID, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	followersSync := r.Header.Get("Collection-Synchronization")
	if followersSync != "" {
		if err := l.saveFollowersDigest(r.Context(), sender, followersSync); err != nil {
			slog.Warn("Failed to save followers sync header", "sender", sender.ID, "header", followersSync, "error", err)
		}
	}

	w.WriteHeader(http.StatusOK)
}
