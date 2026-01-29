/*
Copyright 2025, 2026 Dima Krasner

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
	"crypto/ed25519"
	"crypto/x509"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strconv"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/danger"
	"github.com/dimkr/tootik/data"
	"github.com/dimkr/tootik/httpsig"
	"github.com/dimkr/tootik/icon"
	"github.com/dimkr/tootik/proof"
)

var apGatewayPathRegex = regexp.MustCompile(`\/.well-known\/apgateway\/(did:key:z6Mk[a-km-zA-HJ-NP-Z1-9]+)(\/.+)`)

func (l *Listener) handleApGatewayInboxPost(w http.ResponseWriter, r *http.Request, did string) {
	var actor ap.Actor
	var rsaPrivKeyDer, ed25519PrivKey []byte
	if err := l.DB.QueryRowContext(r.Context(), `select json(actor), rsaprivkey, ed25519privkey from persons where cid = 'ap://' || ? || '/actor' and ed25519privkey is not null`, did).Scan(&actor, &rsaPrivKeyDer, &ed25519PrivKey); errors.Is(err, sql.ErrNoRows) {
		slog.Debug("Receiving user does not exist", "did", did)
		w.WriteHeader(http.StatusNotFound)
		return
	} else if err != nil {
		slog.Warn("Failed to check if receiving user exists", "did", did, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	rsaPrivKey, err := x509.ParsePKCS1PrivateKey(rsaPrivKeyDer)
	if err != nil {
		slog.Warn("Failed to parse RSA private key", "actor", actor.ID, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	l.doHandleInbox(w, r, [2]httpsig.Key{
		{ID: actor.PublicKey.ID, PrivateKey: rsaPrivKey},
		{ID: actor.AssertionMethod[0].ID, PrivateKey: ed25519.NewKeyFromSeed(ed25519PrivKey)},
	})
}

func (l *Listener) handleApGatewayOutboxPost(w http.ResponseWriter, r *http.Request, did string) {
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
	if err := json.Unmarshal(rawActivity, &activity); errors.Is(err, ap.ErrInvalidActivity) || errors.Is(err, ap.ErrUnsupportedActivity) {
		slog.Warn("Failed to unmarshal activity", "body", danger.String(rawActivity), "error", err)
		w.WriteHeader(http.StatusBadRequest)
		json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
		return
	} else if err != nil {
		slog.Warn("Failed to unmarshal activity", "body", danger.String(rawActivity), "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if err := ap.ValidateOrigin(l.Domain, &activity, did); err != nil {
		slog.Warn("Activity is invalid", "activity", activity.ID, "error", err)
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if activity.Proof == (ap.Proof{}) {
		slog.Warn("Activity has no proof", "activity", activity.ID)
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]any{"error": "proof is required"})
		return
	}

	expectedPublicKey := did[len("did:key:"):]

	if m := ap.KeyRegex.FindStringSubmatch(activity.Proof.VerificationMethod); m == nil || m[1] != expectedPublicKey {
		slog.Warn("Could not find expected key in verification method", "activity", activity.ID, "method", activity.Proof.VerificationMethod)
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]any{"error": "invalid verificationMethod"})
		return
	}

	publicKey, err := data.DecodeEd25519PublicKey(expectedPublicKey)
	if err != nil {
		slog.Warn("Failed to decode key to verify proof", "activity", activity.ID, "error", err)
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
		return
	}

	if err := proof.Verify(publicKey, activity.Proof, rawActivity); err != nil {
		slog.Warn("Failed to verify proof", "activity", activity.ID, "error", err)
		w.WriteHeader(http.StatusForbidden)
		json.NewEncoder(w).Encode(map[string]any{"error": err.Error()})
		return
	}

	if _, err = l.DB.ExecContext(
		r.Context(),
		`INSERT OR IGNORE INTO inbox (path, sender, activity, raw) VALUES (?, ?, JSONB(?), ?)`,
		r.URL.Path,
		activity.Actor,
		rawActivity,
		danger.String(rawActivity),
	); err != nil {
		slog.Error("Failed to insert activity", "activity", activity.ID, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.WriteHeader(http.StatusAccepted)
}

func (l *Listener) handleApGatewayInboxGet(w http.ResponseWriter, r *http.Request, did string) {
	if _, key, err := l.verifyRequestUsingKeyID(r, nil); err != nil {
		slog.Warn("Failed to verify inbox request", "did", did, "error", err)
		w.WriteHeader(http.StatusNotFound)
		return
	} else if key != did[len("did:key:"):] {
		slog.Warn("Denying inbox request", "did", did, "key", key)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	var inbox string
	if err := l.DB.QueryRowContext(
		r.Context(),
		`select actor->>'$.inbox' from persons where cid = 'ap://' || ? || '/actor' and ed25519privkey is not null`,
		did,
	).Scan(&inbox); errors.Is(err, sql.ErrNoRows) {
		slog.Warn("Inbox does not exist", "did", did)
		w.WriteHeader(http.StatusNotFound)
		return
	} else if err != nil {
		slog.Warn("Failed to fetch inbox", "did", did, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if r.URL.RawQuery != "" {
		until, err := strconv.ParseInt(r.URL.RawQuery, 10, 64)
		if err != nil {
			slog.Warn("Received an invalid timestamp", "did", did, "until", r.URL.RawQuery)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		items := make([]json.RawMessage, l.Config.InboxPageSize)

		rows, err := l.DB.QueryContext(
			r.Context(),
			`
			select json(activity), inserted from history
			where (public = 1 or path = substr($1, 8 + instr(substr($1, 9), '/'))) and inserted <= $2
			order by inserted desc
			limit $3
			`,
			inbox,
			until,
			l.Config.InboxPageSize,
		)
		if err != nil {
			slog.Warn("Failed to fetch activities", "did", did, "until", until, "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		count := 0
		var earliest int64
		for rows.Next() {
			var inserted int64
			if err := rows.Scan((*[]byte)(&items[count]), &inserted); err != nil {
				slog.Warn("Failed to scan activity", "did", did, "until", until, "error", err)
				continue
			}

			count++
			earliest = inserted
		}

		if err := rows.Err(); err != nil {
			slog.Warn("Failed to scan all activities", "did", did, "until", until, "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if count == 0 {
			slog.Warn("Fetched an empty page", "did", did, "until", until)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		page := &ap.CollectionPage{
			Context:      "https://www.w3.org/ns/activitystreams",
			ID:           fmt.Sprintf("%s?%d", inbox, until),
			Type:         ap.OrderedCollectionPage,
			PartOf:       inbox,
			OrderedItems: items[:count],
		}

		var next sql.NullInt64
		if err := l.DB.QueryRowContext(
			r.Context(),
			`
			select max(inserted) from history
			where (public = 1 or path = substr($1, 8 + instr(substr($1, 9), '/'))) and inserted < $2
			`,
			inbox,
			earliest,
		).Scan(&next); err != nil {
			slog.Warn("Failed to fetch next timestamp", "did", did, "until", until, "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		} else if next.Valid {
			page.Next = fmt.Sprintf("%s?%d", inbox, next.Int64)
		}

		j, err := json.Marshal(page)
		if err != nil {
			slog.Warn("Failed to marshal inbox collection page", "did", did, "until", until, "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		slog.Info("Returning inbox collection page", "did", did, "until", until)

		w.Header().Set("Content-Type", `application/activity+json; charset=utf-8`)
		w.Write(j)

		return
	}

	var latest sql.NullInt64
	var count int64
	if err := l.DB.QueryRowContext(
		r.Context(),
		`
		select max(inserted), count(*) from history
		where public = 1 or path = substr($1, 8 + instr(substr($1, 9), '/'))
		`,
		inbox,
	).Scan(&latest, &count); err != nil {
		slog.Warn("Failed to count items", "did", did, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	collection := &ap.Collection{
		Context:    "https://www.w3.org/ns/activitystreams",
		ID:         inbox,
		Type:       ap.OrderedCollection,
		TotalItems: &count,
	}

	if latest.Valid {
		collection.First = fmt.Sprintf("%s?%d", inbox, latest.Int64)
	}

	j, err := json.Marshal(collection)
	if err != nil {
		slog.Warn("Failed to marshal inbox collection", "did", did, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	slog.Info("Returning inbox collection", "did", did)

	w.Header().Set("Content-Type", `application/activity+json; charset=utf-8`)
	w.Write(j)
}

func (l *Listener) handleApGatewayOutboxGet(w http.ResponseWriter, r *http.Request, did string) {
	if _, key, err := l.verifyRequestUsingKeyID(r, nil); err != nil {
		slog.Warn("Failed to verify outbox request", "did", did, "error", err)
		w.WriteHeader(http.StatusNotFound)
		return
	} else if key != did[len("did:key:"):] {
		slog.Warn("Denying outbox request", "did", did, "key", key)
		w.WriteHeader(http.StatusNotFound)
		return
	}

	actorCID := "ap://" + did + "/actor"

	var outbox string
	if err := l.DB.QueryRowContext(
		r.Context(),
		`select actor->>'$.outbox' from persons where cid = ? and ed25519privkey is not null`,
		actorCID,
	).Scan(&outbox); errors.Is(err, sql.ErrNoRows) {
		slog.Warn("Outbox does not exist", "did", did)
		w.WriteHeader(http.StatusNotFound)
		return
	} else if err != nil {
		slog.Warn("Failed to fetch outbox", "did", did, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	if r.URL.RawQuery != "" {
		until, err := strconv.ParseInt(r.URL.RawQuery, 10, 64)
		if err != nil {
			slog.Warn("Received an invalid timestamp", "did", did, "until", r.URL.RawQuery)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		items := make([]json.RawMessage, l.Config.OutboxPageSize)

		rows, err := l.DB.QueryContext(
			r.Context(),
			`
			select json(activity), inserted from outbox
			where activity->>'$.actor' in (select id from persons where cid = ?) and inserted <= $2
			order by inserted desc
			limit $3
			`,
			actorCID,
			until,
			l.Config.OutboxPageSize,
		)
		if err != nil {
			slog.Warn("Failed to fetch activities", "did", did, "until", until, "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		defer rows.Close()

		count := 0
		var earliest int64
		for rows.Next() {
			var inserted int64
			if err := rows.Scan((*[]byte)(&items[count]), &inserted); err != nil {
				slog.Warn("Failed to scan activity", "did", did, "until", until, "error", err)
				continue
			}

			count++
			earliest = inserted
		}

		if err := rows.Err(); err != nil {
			slog.Warn("Failed to scan all activities", "did", did, "until", until, "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if count == 0 {
			slog.Warn("Fetched an empty page", "did", did, "until", until)
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		page := &ap.CollectionPage{
			Context:      "https://www.w3.org/ns/activitystreams",
			ID:           fmt.Sprintf("%s?%d", outbox, until),
			Type:         ap.OrderedCollectionPage,
			PartOf:       outbox,
			OrderedItems: items[:count],
		}

		var next sql.NullInt64
		if err := l.DB.QueryRowContext(
			r.Context(),
			`
			select max(inserted) from outbox
			where activity->>'$.actor' in (select id from persons where cid = ?) and inserted < $2
			`,
			actorCID,
			earliest,
		).Scan(&next); err != nil {
			slog.Warn("Failed to fetch next timestamp", "did", did, "until", until, "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		} else if next.Valid {
			page.Next = fmt.Sprintf("%s?%d", outbox, next.Int64)
		}

		j, err := json.Marshal(page)
		if err != nil {
			slog.Warn("Failed to marshal outbox collection page", "did", did, "until", until, "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		slog.Info("Returning outbox collection page", "did", did, "until", until)

		w.Header().Set("Content-Type", `application/activity+json; charset=utf-8`)
		w.Write(j)

		return
	}

	var latest sql.NullInt64
	var count int64
	if err := l.DB.QueryRowContext(
		r.Context(),
		`
		select max(inserted), count(*) from outbox
		where activity->>'$.actor' in (select id from persons where cid = ?)
		`,
		actorCID,
	).Scan(&latest, &count); err != nil {
		slog.Warn("Failed to count items", "did", did, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	collection := &ap.Collection{
		Context:    "https://www.w3.org/ns/activitystreams",
		ID:         outbox,
		Type:       ap.OrderedCollection,
		TotalItems: &count,
	}

	if latest.Valid {
		collection.First = fmt.Sprintf("%s?%d", outbox, latest.Int64)
	}

	j, err := json.Marshal(collection)
	if err != nil {
		slog.Warn("Failed to marshal outbox collection", "did", did, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	slog.Info("Returning outbox collection", "did", did)

	w.Header().Set("Content-Type", `application/activity+json; charset=utf-8`)
	w.Write(j)
}

func (l *Listener) fetchFollowersByHost(
	w http.ResponseWriter,
	r *http.Request,
	did string,
) (bool, *ap.Actor, string, *sql.Rows) {
	_, sender, err := l.verifyRequest(r, nil, ap.InstanceActor, l.AppActorKeys)
	if err != nil {
		slog.Warn("Failed to verify followers request", "did", did, "error", err)
		w.WriteHeader(http.StatusUnauthorized)
		return false, nil, "", nil
	}

	u, err := url.Parse(sender.ID)
	if err != nil {
		slog.Warn("Failed to extract sender host", "did", did, "sender", sender.ID, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return false, nil, "", nil
	}

	rows, err := l.DB.QueryContext(
		r.Context(),
		`SELECT follower FROM follows WHERE followed = 'https://' || ? || '/.well-known/apgateway/' || ? || '/actor' AND follower LIKE 'https://' || ? || '/%' AND accepted = 1 ORDER BY inserted DESC, follower`,
		l.Domain,
		did,
		u.Host,
	)
	if err != nil {
		slog.Warn("Failed to fetch followers", "did", did, "sender", sender.ID, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return false, nil, "", nil
	}

	return true, sender, fmt.Sprintf("https://%s/.well-known/apgateway/%s/actor/followers?domain=%s", l.Domain, did, u.Host), rows
}

func (l *Listener) fetchSenderFollowers(
	w http.ResponseWriter,
	r *http.Request,
	did string,
) (bool, *ap.Actor, string, *sql.Rows) {
	_, key, err := l.verifyRequestUsingKeyID(r, nil)
	if err != nil {
		slog.Warn("Failed to verify followers request", "did", did, "error", err)
		w.WriteHeader(http.StatusNotFound)
		return false, nil, "", nil
	}

	if key != did[len("did:key:"):] {
		slog.Warn("Denying followers request", "did", did, "key", key)
		w.WriteHeader(http.StatusNotFound)
		return false, nil, "", nil
	}

	var actor ap.Actor
	if err := l.DB.QueryRowContext(
		r.Context(),
		`select json(actor) from persons where cid = 'ap://' || ? || '/actor' and ed25519privkey is not null`,
		did,
	).Scan(&actor); errors.Is(err, sql.ErrNoRows) {
		slog.Warn("Denying followers request for non-existing user", "did", did)
		w.WriteHeader(http.StatusNotFound)
		return false, nil, "", nil
	} else if err != nil {
		slog.Warn("Failed to check if user exists", "did", did, "error", err)
		w.WriteHeader(http.StatusNotFound)
		return false, nil, "", nil
	}

	rows, err := l.DB.QueryContext(
		r.Context(),
		`SELECT follower FROM follows WHERE followed = 'https://' || ? || '/.well-known/apgateway/' || ? || '/actor' AND accepted = 1 ORDER BY inserted DESC, follower`,
		l.Domain,
		did,
	)
	if err != nil {
		slog.Warn("Failed to fetch followers", "did", did, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return false, nil, "", nil
	}

	return true, &actor, fmt.Sprintf("https://%s/.well-known/apgateway/%s/actor/followers", l.Domain, did), rows
}

func (l *Listener) doHandleApGatewayFollowers(
	w http.ResponseWriter,
	r *http.Request,
	did string,
	fetch func(http.ResponseWriter, *http.Request, string) (bool, *ap.Actor, string, *sql.Rows),
) {
	ok, sender, collectionID, rows := fetch(w, r, did)
	if !ok {
		return
	}
	defer rows.Close()

	items := ap.Audience{}

	for rows.Next() {
		var follower string
		if err := rows.Scan(&follower); err != nil {
			slog.Warn("Failed to fetch followers", "did", did, "sender", sender.ID, "error", err)
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		items.Add(follower)
	}

	if err := rows.Err(); err != nil {
		slog.Warn("Failed to fetch followers", "did", did, "sender", sender.ID, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	collection, err := json.Marshal(&ap.Collection{
		Context:      "https://www.w3.org/ns/activitystreams",
		ID:           collectionID,
		Type:         ap.OrderedCollection,
		OrderedItems: items,
	})
	if err != nil {
		slog.Warn("Failed to fetch followers", "did", did, "sender", sender.ID, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	slog.Info("Received followers request", "did", did, "sender", sender.ID, "count", len(items.OrderedMap))

	w.Header().Set("Content-Type", `application/activity+json; charset=utf-8`)
	w.Write(collection)
}

func (l *Listener) handleAPGatewayFollowersSync(w http.ResponseWriter, r *http.Request, did string) {
	l.doHandleApGatewayFollowers(w, r, did, l.fetchFollowersByHost)
}

func (l *Listener) handleAPGatewayFollowers(w http.ResponseWriter, r *http.Request, did string) {
	l.doHandleApGatewayFollowers(w, r, did, l.fetchSenderFollowers)
}

func (l *Listener) handleAPGatewayGetObject(w http.ResponseWriter, r *http.Request, cid string) {
	slog.Info("Fetching object", "cid", cid)

	var raw string
	if err := l.DB.QueryRowContext(
		r.Context(),
		`
		select raw from
		(
			select json(actor) as raw from persons
			where cid = $1 and ed25519privkey is not null
			union all
			select json(notes.object) as raw from notes
			join persons on notes.author = persons.id
			where notes.cid = $1 and notes.deleted = 0 and notes.public = 1 and persons.ed25519privkey is not null
			union all
			select json(outbox.activity) as raw from outbox
			join persons on outbox.activity->>'$.actor' = persons.id
			where outbox.cid = $1 and (exists (select 1 from json_each(outbox.activity->'$.cc') where value = $2) or exists (select 1 from json_each(outbox.activity->'$.to') where value = $2)) and persons.ed25519privkey is not null
		)
		limit 1
		`,
		cid,
		ap.Public,
	).Scan(&raw); errors.Is(err, sql.ErrNoRows) {
		slog.Info("Notifying about missing object", "cid", cid)
		w.WriteHeader(http.StatusNotFound)
		return
	} else if err != nil {
		slog.Warn("Failed to fetch object", "cid", cid, "error", err)
		w.WriteHeader(http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)
	w.Write(danger.Bytes(raw))
}

func (l *Listener) handleAPGatewayPost(w http.ResponseWriter, r *http.Request) {
	if m := apGatewayPathRegex.FindStringSubmatch(r.URL.Path); m != nil {
		switch m[2] {
		case "/actor/inbox":
			l.handleApGatewayInboxPost(w, r, m[1])
			return

		case "/actor/outbox":
			l.handleApGatewayOutboxPost(w, r, m[1])
			return
		}
	}

	w.WriteHeader(http.StatusNotFound)
}

func (l *Listener) handleAPGatewayGet(w http.ResponseWriter, r *http.Request) {
	m := apGatewayPathRegex.FindStringSubmatch(r.URL.Path)
	if m == nil {
		w.WriteHeader(http.StatusNotFound)
		return
	}

	switch m[2] {
	case "/actor/followers_synchronization":
		l.handleAPGatewayFollowersSync(w, r, m[1])

	case "/actor/followers":
		l.handleAPGatewayFollowers(w, r, m[1])

	case "/actor/icon" + icon.FileNameExtension:
		l.doHandleIcon(w, r, "ap://"+m[1]+"/actor")

	case "/actor/inbox":
		l.handleApGatewayInboxGet(w, r, m[1])

	case "/actor/outbox":
		l.handleApGatewayOutboxGet(w, r, m[1])

	default:
		l.handleAPGatewayGetObject(w, r, "ap://"+m[1]+m[2])
	}
}
