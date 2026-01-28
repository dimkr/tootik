/*
Copyright 2023 - 2026 Dima Krasner

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
	"crypto/ed25519"
	"crypto/x509"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/danger"
	"github.com/dimkr/tootik/data"
	"github.com/dimkr/tootik/httpsig"
	"github.com/dimkr/tootik/proof"
)

var unsupportedActivityTypes = map[ap.ActivityType]struct{}{
	ap.Like:       {},
	ap.Dislike:    {},
	ap.EmojiReact: {},
	ap.Add:        {},
	ap.Remove:     {},
	ap.Move:       {},
}

func (l *Listener) getActivityOrigin(activity *ap.Activity, sender *ap.Actor) (string, string, string, error) {
	if activity.ID == "" {
		return "", "", "", errors.New("unspecified activity ID")
	}

	activityOrigin, err := ap.Origin(activity.ID)
	if err != nil {
		return "", "", "", err
	}

	if sender.ID == "" {
		return "", "", "", errors.New("unspecified sender ID")
	}

	senderOrigin, senderHost, err := ap.Origins(sender.ID)
	if err != nil {
		return "", "", "", err
	}

	return activityOrigin, senderOrigin, senderHost, nil
}

func (l *Listener) fetchObject(ctx context.Context, id string, keys [2]httpsig.Key) (bool, []byte, error) {
	resp, err := l.Resolver.Get(ctx, keys, id)
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

	var body []byte
	if resp.ContentLength >= 0 {
		body = make([]byte, resp.ContentLength)
		_, err = io.ReadFull(resp.Body, body)
	} else {
		body, err = io.ReadAll(io.LimitReader(resp.Body, l.Config.MaxRequestBodySize))
	}
	if err != nil {
		return true, nil, err
	}

	if !ap.IsPortable(id) {
		return true, body, nil
	}

	var withProof struct {
		Proof ap.Proof `json:"proof"`
	}
	if err := json.Unmarshal(body, &withProof); err != nil {
		return true, nil, err
	}

	m := ap.KeyRegex.FindStringSubmatch(withProof.Proof.VerificationMethod)
	if m == nil {
		return true, nil, fmt.Errorf("%s does not contain a public key", withProof.Proof.VerificationMethod)
	}

	origin, err := ap.Origin(id)
	if err != nil {
		return true, nil, fmt.Errorf("failed to get origin of %s: %w", id, err)
	}

	if suffix, ok := strings.CutPrefix(origin, "did:key:"); !ok || suffix != m[1] {
		return true, nil, fmt.Errorf("key %s does not belong to %s", m[1], origin)
	}

	publicKey, err := data.DecodeEd25519PublicKey(m[1])
	if err != nil {
		return true, nil, fmt.Errorf("failed to verify proof using %s: %w", withProof.Proof.VerificationMethod, err)
	}

	if err := proof.Verify(publicKey, withProof.Proof, body); err != nil {
		return true, nil, err
	}

	return true, body, nil
}

func (l *Listener) handleSharedInbox(w http.ResponseWriter, r *http.Request) {
	l.doHandleInbox(w, r, l.AppActorKeys)
}

func (l *Listener) handleInbox(w http.ResponseWriter, r *http.Request) {
	receiver := r.PathValue("username")

	var actor ap.Actor
	var rsaPrivKeyDer, ed25519PrivKey []byte
	if err := l.DB.QueryRowContext(r.Context(), `select json(actor), rsaprivkey, ed25519privkey from persons where actor->>'$.preferredUsername' = ? and ed25519privkey is not null`, receiver).Scan(&actor, &rsaPrivKeyDer, &ed25519PrivKey); errors.Is(err, sql.ErrNoRows) {
		slog.Debug("Receiving user does not exist", "receiver", receiver)
		w.WriteHeader(http.StatusNotFound)
		return
	} else if err != nil {
		slog.Warn("Failed to check if receiving user exists", "receiver", receiver, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	rsaPrivKey, err := x509.ParsePKCS1PrivateKey(rsaPrivKeyDer)
	if err != nil {
		slog.Warn("Failed to parse RSA private key", "receiver", receiver, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	l.doHandleInbox(w, r, [2]httpsig.Key{
		{ID: actor.PublicKey.ID, PrivateKey: rsaPrivKey},
		{ID: actor.AssertionMethod[0].ID, PrivateKey: ed25519.NewKeyFromSeed(ed25519PrivKey)},
	})
}

func (l *Listener) doHandleInbox(w http.ResponseWriter, r *http.Request, keys [2]httpsig.Key) {
	if r.ContentLength > l.Config.MaxRequestBodySize {
		slog.Warn("Ignoring big request", "size", r.ContentLength)
		w.WriteHeader(http.StatusRequestEntityTooLarge)
		return
	}

	var rawActivity []byte
	var err error
	if r.ContentLength >= 0 {
		rawActivity = make([]byte, r.ContentLength)
		_, err = io.ReadFull(r.Body, rawActivity)
	} else {
		rawActivity, err = io.ReadAll(io.LimitReader(r.Body, l.Config.MaxRequestBodySize))
	}
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	var activity ap.Activity
	if err := json.Unmarshal(rawActivity, &activity); err != nil {
		slog.Warn("Failed to unmarshal activity", "body", danger.String(rawActivity), "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	r.Body = io.NopCloser(bytes.NewReader(rawActivity))

	// if actor is deleted, ignore this activity if we don't know this actor
	var flags ap.ResolverFlag
	if activity.Type == ap.Delete {
		flags |= ap.Offline
	}

	var sender *ap.Actor
	var sig *httpsig.Signature
	if activity.Proof != (ap.Proof{}) {
		// if activity has an integrity proof, pretend it was sent by its actor even if forwarded by another
		sender, err = l.verifyProof(r.Context(), activity.Proof, &activity, rawActivity, flags, keys)
		if err != nil {
			slog.Warn("Failed to verify integrity proof", "activity", &activity, "proof", &activity.Proof, "error", err)
			w.WriteHeader(http.StatusUnauthorized)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
			return
		}
	} else if ap.IsPortable(activity.ID) {
		slog.Warn("Portable activity has no integrity proof", "activity", &activity)
		w.WriteHeader(http.StatusUnauthorized)
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]any{"error": "integrity proof is required"})
		return
	} else {
		sig, sender, err = l.verifyRequest(r, rawActivity, flags, keys)
		if err != nil {
			if errors.Is(err, ErrActorGone) {
				w.WriteHeader(http.StatusAccepted)
				return
			}

			if errors.Is(err, ErrActorNotCached) {
				slog.Debug("Ignoring Delete activity for unknown actor", "error", err)
				w.WriteHeader(http.StatusAccepted)
				return
			}

			if errors.Is(err, ErrBlockedDomain) {
				slog.Debug("Failed to verify activity", "activity", &activity, "error", err)
			} else {
				slog.Warn("Failed to verify activity", "activity", &activity, "error", err)
			}

			w.WriteHeader(http.StatusUnauthorized)
			w.Header().Set("Content-Type", "application/json")
			json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})

			return
		}
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
		w.WriteHeader(http.StatusAccepted)
		return
	}

	/*
		if an activity wasn't sent by an actor on the same server, we must fetch the activity from its origin instead
		of trusting the sender to pass it as-is
	*/
	origin, senderOrigin, senderHost, err := l.getActivityOrigin(queued, sender)
	if err != nil {
		slog.Warn("Failed to determine whether or not activity is forwarded", "activity", &activity, "sender", sender.ID, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if l.BlockList != nil && l.BlockList.Contains(origin) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]any{"error": "blocked"})
		return
	}

	forwarded := origin != senderOrigin

	/* if we don't support this activity or it's invalid, we don't want to fetch it (we validate again later) */
	if err := ap.ValidateOrigin(l.Domain, queued, origin); errors.Is(err, ap.ErrUnsupportedActivity) {
		slog.Debug("Activity is unsupported", "activity", &activity, "sender", sender.ID, "error", err)
		w.WriteHeader(http.StatusAccepted)
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

		if exists, fetched, err := l.fetchObject(r.Context(), id, keys); !exists && queued.Type == ap.Delete {
			queued = &ap.Activity{
				ID:     queued.ID,
				Type:   ap.Delete,
				Actor:  queued.Actor,
				Object: id,
			}
		} else if err == nil && exists && queued.Type == ap.Delete {
			var parsed ap.Object
			if err := json.Unmarshal(fetched, &parsed); err != nil {
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
			if err := json.Unmarshal(fetched, &parsed); err != nil {
				// hack for Mastodon: we get the updated Note when we fetch an Update activity
				var post ap.Object
				if err := json.Unmarshal(fetched, &post); err != nil {
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
			if err := json.Unmarshal(fetched, &parsed); err != nil {
				slog.Warn("Ignoring invalid forwarded activity", "activity", &activity, "sender", sender.ID, "error", err)
				w.WriteHeader(http.StatusBadRequest)
				return
			}

			queued = &parsed
		}

		// we must validate the original activity because the forwarded one can be valid while the original isn't
		if err := ap.ValidateOrigin(l.Domain, queued, origin); errors.Is(err, ap.ErrUnsupportedActivity) {
			slog.Debug("Activity is unsupported", "activity", &activity, "sender", sender.ID, "error", err)
			w.WriteHeader(http.StatusAccepted)
			return
		} else if err != nil {
			slog.Warn("Activity is invalid", "activity", &activity, "sender", sender.ID, "error", err)
			w.WriteHeader(http.StatusBadRequest)
			return
		}
	}

	if _, err = l.DB.ExecContext(
		r.Context(),
		`INSERT OR IGNORE INTO inbox (path, sender, activity, raw) VALUES (?, ?, JSONB(?), ?)`,
		r.URL.Path,
		sender.ID,
		queued,
		danger.String(rawActivity),
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
	if sig != nil {
		switch sig.Alg {
		case "rsa-sha256", "hs2019":
			capabilities = ap.CavageDraftSignatures
		case "rsa-v1_5-sha256":
			capabilities = ap.RFC9421RSASignatures
		case "ed25519":
			capabilities = ap.RFC9421Ed25519Signatures
		}
	}

	if capabilities > 0 {
		if _, err = l.DB.ExecContext(
			r.Context(),
			`INSERT INTO servers (host, capabilities) VALUES ($1, $2) ON CONFLICT(host) DO UPDATE SET capabilities = capabilities | $2, updated = UNIXEPOCH()`,
			senderHost,
			capabilities,
		); err != nil {
			slog.Error("Failed to record server capabilities", "server", senderOrigin, "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
	}

	w.WriteHeader(http.StatusAccepted)
}
