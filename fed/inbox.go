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
	"io"
	"log/slog"
	"net/http"
	"net/url"

	"github.com/dimkr/tootik/ap"
)

var unsupportedActivityTypes = map[ap.ActivityType]struct{}{
	ap.Like:       {},
	ap.Dislike:    {},
	ap.EmojiReact: {},
	ap.Add:        {},
	ap.Remove:     {},
	ap.Move:       {},
}

func (l *Listener) getActivityOrigin(activity *ap.Activity, sender *ap.Actor) (string, bool, error) {
	if activity.ID == "" {
		return "", false, errors.New("unspecified activity ID")
	}

	activityUrl, err := url.Parse(activity.ID)
	if err != nil {
		return "", false, err
	}

	if sender.ID == "" {
		return "", false, errors.New("unspecified sender ID")
	}

	senderUrl, err := url.Parse(sender.ID)
	if err != nil {
		return "", false, err
	}

	return activityUrl.Host, activityUrl.Host != senderUrl.Host, nil
}

func (l *Listener) validateActivity(activity *ap.Activity, origin string, depth uint) error {
	if depth == ap.MaxActivityDepth {
		return errors.New("activity is too nested")
	}

	if origin == l.Domain {
		return errors.New("invalid origin")
	}

	slog.Debug("Validating activity origin", "activity", activity, "origin", origin, "depth", depth)

	if activity.ID == "" {
		return errors.New("unspecified activity ID")
	}

	activityUrl, err := url.Parse(activity.ID)
	if err != nil {
		return err
	}

	if activityUrl.Host != origin {
		return fmt.Errorf("invalid activity host: %s", activityUrl.Host)
	}

	if activity.Actor == "" {
		return errors.New("unspecified actor")
	}

	actorUrl, err := url.Parse(activity.Actor)
	if err != nil {
		return err
	}

	if actorUrl.Host != origin {
		return fmt.Errorf("invalid actor host: %s", actorUrl.Host)
	}

	switch activity.Type {
	case ap.Delete:
		// $origin can only delete objects that belong to $origin
		switch v := activity.Object.(type) {
		case *ap.Object:
			if objectUrl, err := url.Parse(v.ID); err != nil {
				return err
			} else if objectUrl.Host != origin {
				return fmt.Errorf("invalid object host: %s", objectUrl.Host)
			}

		case string:
			if objectUrl, err := url.Parse(v); err != nil {
				return err
			} else if objectUrl.Host != origin {
				return fmt.Errorf("invalid object host: %s", objectUrl.Host)
			}

		default:
			return fmt.Errorf("invalid object: %T", v)
		}

	case ap.Follow:
		if inner, ok := activity.Object.(string); ok {
			if innerUrl, err := url.Parse(inner); err != nil {
				return err
				// actors from $origin can only follow ours
			} else if innerUrl.Host != l.Domain {
				return fmt.Errorf("invalid object host: %s", innerUrl.Host)
			}
		} else {
			return fmt.Errorf("invalid object: %T", activity.Object)
		}

	case ap.Accept, ap.Reject:
		// $origin can only accept or reject Follow activities that belong to us
		switch v := activity.Object.(type) {
		case *ap.Activity:
			if v.Type != ap.Follow {
				return fmt.Errorf("invalid object type: %s", v.Type)
			}

			if activityUrl, err := url.Parse(v.ID); err != nil {
				return err
			} else if activityUrl.Host != l.Domain {
				return fmt.Errorf("invalid object host: %s", activityUrl.Host)
			}

		case string:
			if activityUrl, err := url.Parse(v); err != nil {
				return err
			} else if activityUrl.Host != l.Domain {
				return fmt.Errorf("invalid object host: %s", activityUrl.Host)
			}

		default:
			return fmt.Errorf("invalid object: %T", v)
		}

	case ap.Undo:
		if inner, ok := activity.Object.(*ap.Activity); ok {
			if inner.Type != ap.Announce && inner.Type != ap.Follow {
				return fmt.Errorf("invalid inner activity: %w: %s", ap.ErrUnsupportedActivity, inner.Type)
			}

			// $origin can only undo actions performed by actors from $origin
			if err := l.validateActivity(inner, origin, depth+1); err != nil {
				return err
			}
		} else {
			return fmt.Errorf("invalid object: %T", activity.Object)
		}

	case ap.Create, ap.Update:
		// $origin can only create objects that belong to $origin
		if obj, ok := activity.Object.(*ap.Object); ok {
			if objectUrl, err := url.Parse(obj.ID); err != nil {
				return err
			} else if objectUrl.Host != origin {
				return fmt.Errorf("invalid object host: %s", objectUrl.Host)
			} else if obj.AttributedTo != "" && obj.AttributedTo != activity.Actor {
				authorUrl, err := url.Parse(obj.AttributedTo)
				if err != nil {
					return err
				}

				if authorUrl.Host != origin {
					return fmt.Errorf("invalid author host: %s", authorUrl.Host)
				}
			}
		} else if s, ok := activity.Object.(string); ok {
			if innerUrl, err := url.Parse(s); err != nil {
				return err
			} else if innerUrl.Host != origin {
				return fmt.Errorf("invalid object host: %s", innerUrl.Host)
			}
		} else {
			return fmt.Errorf("invalid object: %T", obj)
		}

	case ap.Announce:
		// we always unwrap nested Announce, validate the inner activity and don't allow nesting
		if _, ok := activity.Object.(*ap.Activity); ok {
			return errors.New("announce must not be nested")
		} else if s, ok := activity.Object.(string); !ok {
			return fmt.Errorf("invalid object: %T", activity.Object)
		} else if s == "" {
			return errors.New("empty ID")
		} else if _, err := url.Parse(s); err != nil {
			return err
		}

	default:
		return fmt.Errorf("%w: %s", ap.ErrUnsupportedActivity, activity.Type)
	}

	return nil
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

	sig, sender, err := l.verify(r, rawActivity, flags)
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
			slog.Debug("Failed to verify activity", "activity", &activity, "error", err)
		} else {
			slog.Warn("Failed to verify activity", "activity", &activity, "error", err)
		}
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	/*
		we have 4 activities:
		1. the one we received, in its JSON form (rawActivity): we need it in case we're going to forward it
		2. the one we received, parsed (activity)
		3. the activity or object we fetched, if the activity wasn't sent to us by its origin (see later)
		4. the activity we want to queue for processing (queued)

		(if we fetch 3, we process 3, otherwise we process 2, but we always send 1 when we forward)
	*/

	queued := &activity

	/*
		if this is chain of Announce activities, unwrap: if the outermost Announce and the innermost activity belong to
		different servers, we need to fetch the latter from its origin; in other words, the Announce that wraps an
		activity shouldn't change the validation flow because it's not the Announce that needs to be validated
	*/
	for queued.Type == ap.Announce {
		if inner, ok := queued.Object.(*ap.Activity); ok {
			queued = inner
		} else if o, ok := queued.Object.(*ap.Object); ok {
			slog.Debug("Wrapping object with Update activity", "activity", &activity, "sender", sender.ID, "object", o.ID)

			// hack for Lemmy: wrap a Page inside Announce with Update
			queued = &ap.Activity{
				ID:     o.ID,
				Type:   ap.Update,
				Actor:  o.AttributedTo,
				Object: o,
			}

			break
		} else {
			break
		}
	}

	if _, ok := unsupportedActivityTypes[queued.Type]; ok {
		slog.Debug("Ignoring unsupported activity", "activity", &activity, "sender", sender.ID)
		w.WriteHeader(http.StatusOK)
		return
	}

	/*
		if an activity wasn't sent by an actor on the same server, we must fetch the activity from its origin instead
		of trusting the sender to pass it as-is
	*/
	origin, forwarded, err := l.getActivityOrigin(queued, sender)
	if err != nil {
		slog.Warn("Failed to determine whether or not activity is forwarded", "activity", &activity, "sender", sender.ID, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	/* if we don't support this activity or it's invalid, we don't want to fetch it (we validate again later) */
	if err := l.validateActivity(queued, origin, 0); errors.Is(err, ap.ErrUnsupportedActivity) {
		slog.Debug("Activity is unsupported", "activity", &activity, "sender", sender.ID, "error", err)
		w.WriteHeader(http.StatusOK)
		return
	} else if err != nil {
		slog.Warn("Activity is invalid", "activity", &activity, "sender", sender.ID, "error", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	} else if forwarded {
		// if this is a forwarded Delete, we ask the origin if the deleted object is indeed deleted
		id := queued.ID
		if queued.Type == ap.Delete {
			switch o := queued.Object.(type) {
			case *ap.Object:
				id = o.ID
			case string:
				id = o
			default:
				slog.Warn("Ignoring invalid forwarded Delete activity", "activity", &activity, "sender", sender.ID)
				w.WriteHeader(http.StatusBadRequest)
				return
			}
		}

		slog.Info("Fetching forwarded object", "activity", &activity, "id", id, "sender", sender.ID)

		if exists, fetched, err := l.fetchObject(r.Context(), id); !exists && queued.Type == ap.Delete {
			queued = &ap.Activity{
				ID:     queued.ID,
				Type:   ap.Delete,
				Actor:  queued.Actor,
				Object: id,
			}
		} else if err == nil && exists && queued.Type == ap.Delete {
			var parsed ap.Object
			if err := json.Unmarshal([]byte(fetched), &parsed); err != nil {
				slog.Warn("Ignoring invalid forwarded Delete activity", "activity", &activity, "sender", sender.ID, "error", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			} else if parsed.Type != ap.Tombstone {
				slog.Warn("Ignoring forwarded Delete activity for existing object", "activity", &activity, "id", id, "sender", sender.ID)
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			// hack for Mastodon: a deleted Note is replaced with a Tombstone
			slog.Debug("Wrapping Tombstone with Delete", "activity", &activity, "sender", sender.ID)
			queued = &ap.Activity{
				ID:     queued.ID,
				Type:   ap.Delete,
				Actor:  queued.Actor,
				Object: &parsed,
			}
		} else if err != nil {
			slog.Warn("Failed to fetch forwarded object", "activity", &activity, "id", id, "sender", sender.ID, "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		} else if queued.Type == ap.Update {
			var parsed ap.Activity
			if err := json.Unmarshal([]byte(fetched), &parsed); err != nil {
				// hack for Mastodon: we get the updated Note when we fetch an Update activity
				var post ap.Object
				if err := json.Unmarshal([]byte(fetched), &post); err != nil {
					slog.Warn("Ignoring invalid forwarded Update activity", "activity", &activity, "sender", sender.ID, "error", err)
					w.WriteHeader(http.StatusBadRequest)
					return
				}

				slog.Debug("Wrapping forwarded Update activity", "activity", &activity, "sender", sender.ID)
				queued = &ap.Activity{
					ID:     queued.ID,
					Type:   ap.Update,
					Actor:  queued.Actor,
					Object: &post,
				}
			} else {
				queued = &parsed
			}
		} else {
			var parsed ap.Activity
			if err := json.Unmarshal([]byte(fetched), &parsed); err != nil {
				slog.Warn("Ignoring invalid forwarded activity", "activity", &activity, "sender", sender.ID, "error", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			queued = &parsed
		}

		// we must validate the original activity because the forwarded one can be valid while the original isn't
		if err := l.validateActivity(queued, origin, 0); errors.Is(err, ap.ErrUnsupportedActivity) {
			slog.Debug("Activity is unsupported", "activity", &activity, "sender", sender.ID, "error", err)
			w.WriteHeader(http.StatusOK)
			return
		} else if err != nil {
			slog.Warn("Activity is invalid", "activity", &activity, "sender", sender.ID, "error", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	}

	if _, err = l.DB.ExecContext(
		r.Context(),
		`INSERT OR IGNORE INTO inbox (sender, activity, raw) VALUES (?, JSONB(?), ?)`,
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

	var capabilities ap.Capability
	switch sig.Alg {
	case "ed25519":
		capabilities = ap.RFC9421Signatures | ap.RFC9421ED25519Signatures
	case "rsa-v1_5-sha256":
		capabilities = ap.RFC9421Signatures
	}

	if _, err = l.DB.ExecContext(
		r.Context(),
		`INSERT INTO servers (host, capabilities) VALUES ($1, $2) ON CONFLICT(host) DO UPDATE SET capabilities = capabilities | $2`,
		origin,
		capabilities,
	); err != nil {
		slog.Error("Failed to record server capabilities", "servers", origin, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusOK)
}
