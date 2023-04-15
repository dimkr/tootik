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
	"bytes"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/note"
	_ "github.com/mattn/go-sqlite3"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
	"path/filepath"
	"strings"
)

type inboxHandler struct {
	Log *log.Logger
	DB  *sql.DB
}

func (h *inboxHandler) Handle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	receiver := fmt.Sprintf("https://%s/user/%s", cfg.Domain, filepath.Base(r.URL.Path))
	var registered int
	if err := h.DB.QueryRowContext(r.Context(), `select exists (select 1 from persons where id = ?)`, receiver).Scan(&registered); err != nil {
		h.Log.WithField("receiver", receiver).WithError(err).Warn("Failed to check if receiving user exists")
		w.WriteHeader(http.StatusInternalServerError)
		return
	} else if registered == 0 {
		h.Log.WithField("receiver", receiver).Warn("Receiving user does not exist")
		w.WriteHeader(http.StatusNotFound)
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return
	}

	r.Body = ioutil.NopCloser(bytes.NewReader(body))

	sender, err := verify(r.Context(), r, h.DB)
	if err != nil {
		if errors.Is(err, goneError) {
			w.WriteHeader(http.StatusOK)
			return
		}
		h.Log.WithError(err).Warn("Failed to verify message")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	var known int
	if err := h.DB.QueryRowContext(r.Context(), `select exists (select 1 from follows where followed = ?)`, sender.ID).Scan(&known); err != nil {
		h.Log.WithField("sender", sender.ID).WithError(err).Warn("Failed to check if sender is followed")
		w.WriteHeader(http.StatusInternalServerError)
		return
	} else if known == 0 {
		if err := h.DB.QueryRowContext(r.Context(), `select exists (select 1 from notes where author like $1 and ($2 in (to0, to1, to2, cc0, cc1, cc2) or (to2 is not null and exists (select 1 from json_each(object->'to') where value = $2)) or (cc2 is not null and exists (select 1 from json_each(object->'cc') where value = $2))))`, fmt.Sprintf("https://%s/%%", cfg.Domain), sender.ID).Scan(&known); err != nil {
			h.Log.WithField("sender", sender.ID).WithError(err).Warn("Failed to check if sender has received any post")
			w.WriteHeader(http.StatusInternalServerError)
			return
		} else if known == 0 {
			h.Log.WithField("sender", sender.ID).Warn("Sender is unknown")
			w.WriteHeader(http.StatusOK)
			return
		}
	}

	var req ap.Activity
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&req); err != nil {
		h.Log.WithFields(log.Fields{"sender": sender.ID, "body": string(body)}).WithError(err).Warn("Failed to unmarshal request")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	id := req.ID

	switch req.Type {
	case ap.DeleteActivity:
		if _, ok := req.Object.(*ap.Object); !ok {
			h.Log.WithFields(log.Fields{"id": id, "sender": sender.ID}).Info("Deleted object is not an object")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		deleted := req.Object.(*ap.Object).ID
		h.Log.WithFields(log.Fields{"id": id, "sender": sender.ID, "deleted": deleted}).Info("Received delete request")

		if deleted == sender.ID {
			if _, err = h.DB.ExecContext(r.Context(), `delete from persons where id =`, deleted); err != nil {
				h.Log.WithField("id", deleted).WithError(err).Warn("Failed to delete person")
			}
		} else if _, err = h.DB.ExecContext(r.Context(), `delete from notes where id = ? and author = ?`, deleted, sender.ID); err != nil {
			h.Log.WithField("id", deleted).WithError(err).Warn("Failed to delete notes by actor")
		}

	case ap.FollowActivity:
		if sender.ID != req.Actor {
			h.Log.WithFields(log.Fields{"sender": sender.ID, "actor": req.Actor}).Warn("Received unauthorized follow request")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		followed, ok := req.Object.(string)
		if !ok {
			h.Log.Info("Received a request to follow a non-link object")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if followed == "" {
			h.Log.Info("Received an invalid follow request")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		prefix := fmt.Sprintf("https://%s/", cfg.Domain)
		if strings.HasPrefix(req.Actor, prefix) || !strings.HasPrefix(followed, prefix) {
			h.Log.WithFields(log.Fields{"follower": req.Actor, "followed": followed}).Warn("Received an invalid follow request")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		followedString := ""
		if err := h.DB.QueryRowContext(r.Context(), `select actor from persons where id = ?`, followed).Scan(&followedString); err != nil {
			h.Log.WithFields(log.Fields{"follower": req.Actor, "followed": followed}).WithError(err).Info("Failed to fetch user to follow")
			w.WriteHeader(http.StatusNotFound)
			return
		}

		from := ap.Actor{}
		if err := json.Unmarshal([]byte(followedString), &from); err != nil {
			h.Log.WithFields(log.Fields{"follower": req.Actor, "followed": followed}).WithError(err).Warn("Failed to unmarshal actor")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		var duplicate int
		if err := h.DB.QueryRowContext(r.Context(), `select exists (select 1 from follows where follower = ? and followed = ?)`, req.Actor, followed).Scan(&duplicate); err != nil {
			h.Log.WithFields(log.Fields{"follower": req.Actor, "followed": followed}).WithError(err).Warn("Failed to check if user is already followed")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		h.Log.WithFields(log.Fields{"follower": req.Actor, "followed": followed}).Info("Approving follow request")

		j, err := json.Marshal(map[string]any{
			"@context": "https://www.w3.org/ns/activitystreams",
			"type":     ap.AcceptActivity,
			"id":       fmt.Sprintf("https://%s/accept/%x", cfg.Domain, sha256.Sum256(body)),
			"actor":    followed,
			"to":       []string{req.Actor},
			"object": map[string]any{
				"type": ap.FollowObject,
				"id":   req.ID,
			},
		})
		if err != nil {
			h.Log.WithFields(log.Fields{"follower": req.Actor, "followed": followed}).WithError(err).Info("Failed to marshal Accept response")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		resolver, err := Resolvers.Borrow(r.Context())
		if err != nil {
			h.Log.WithFields(log.Fields{"follower": req.Actor, "followed": followed}).WithError(err).Info("Failed to get resolver to send Accept response")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		to, err := resolver.Resolve(r.Context(), h.DB, &from, req.Actor)
		if err != nil {
			Resolvers.Return(resolver)
			h.Log.WithFields(log.Fields{"follower": req.Actor, "followed": followed}).WithError(err).Info("Failed to resolve recipient for Accept response")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if err := Send(r.Context(), h.DB, &from, resolver, to, j); err != nil {
			Resolvers.Return(resolver)
			h.Log.WithFields(log.Fields{"follower": req.Actor, "followed": followed}).WithError(err).Info("Failed to send Accept response")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		Resolvers.Return(resolver)

		if duplicate == 1 {
			h.Log.WithFields(log.Fields{"follower": req.Actor, "followed": followed, "dupicate": duplicate}).Info("User is already followed")
		} else {
			if _, err := h.DB.ExecContext(
				r.Context(),
				`INSERT INTO follows (id, follower, followed ) VALUES(?,?,?)`,
				req.ID,
				req.Actor,
				followed,
			); err != nil {
				h.Log.WithFields(log.Fields{"follower": req.Actor, "followed": followed}).WithError(err).Warn("Failed to insert Follow")
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
		}

	case ap.AcceptActivity:
		if sender.ID != req.Actor {
			h.Log.WithFields(log.Fields{"sender": sender.ID, "actor": req.Actor}).Warn("Received unauthorized accept notification")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		if follow, ok := req.Object.(string); ok && follow != "" {
			h.Log.WithFields(log.Fields{"sender": sender.ID, "actor": req.Actor, "follow": follow}).Info("Follow is accepted")
		} else if followObject, ok := req.Object.(*ap.Object); ok && followObject.Type == ap.FollowObject && followObject.ID != "" {
			h.Log.WithFields(log.Fields{"sender": sender.ID, "actor": req.Actor, "follow": followObject.ID}).Info("Follow is accepted")
		} else {
			h.Log.Info("Received an invalid accept notification")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

	case ap.UndoActivity:
		if sender.ID != req.Actor {
			h.Log.WithFields(log.Fields{"sender": sender.ID, "actor": req.Actor}).Warn("Received unauthorized undo notification")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		follow, ok := req.Object.(*ap.Object)
		if !ok {
			h.Log.Info("Received a request to undo a non-object object")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if follow.Type != ap.FollowObject {
			h.Log.Info("Received a request to undo a non-Follow object")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if follow.ID == "" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		follower := req.Actor
		if _, err := h.DB.ExecContext(r.Context(), `delete from follows where id = ? and follower = ?`, follow.ID, follower); err != nil {
			h.Log.WithFields(log.Fields{"follow": follow.ID, "follower": follower}).WithError(err).Info("Failed to undo a Follow")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		h.Log.WithFields(log.Fields{"follow": follow.ID, "follower": follower}).Info("Removed a Follow")

	case ap.CreateActivity:
		post, ok := req.Object.(*ap.Object)
		if !ok {
			h.Log.WithField("create", req.ID).Info("Received invalid Create")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		prefix := fmt.Sprintf("https://%s/", cfg.Domain)
		if strings.HasPrefix(sender.ID, prefix) || strings.HasPrefix(post.ID, prefix) || strings.HasPrefix(post.AttributedTo, prefix) || strings.HasPrefix(req.Actor, prefix) {
			h.Log.WithFields(log.Fields{"create": req.ID, "note": post.ID, "author": post.AttributedTo, "sender": req.Actor}).Info("Ignoring create request for local note")
			w.WriteHeader(http.StatusUnauthorized)
			return
		}

		var duplicate int
		if err := h.DB.QueryRowContext(r.Context(), `select exists (select 1 from notes where id = ?)`, post.ID).Scan(&duplicate); err != nil {
			h.Log.WithFields(log.Fields{"create": req.ID, "author": post.AttributedTo}).WithError(err).Warn("Failed to check for duplicate notes")
		}

		resolver, err := Resolvers.Borrow(r.Context())
		if err != nil {
			h.Log.WithFields(log.Fields{"create": req.ID, "author": post.AttributedTo}).WithError(err).Info("Failed to get resolver to resolve author")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if _, err := resolver.Resolve(r.Context(), h.DB, nil, post.AttributedTo); err != nil {
			Resolvers.Return(resolver)
			h.Log.WithFields(log.Fields{"create": req.ID, "author": post.AttributedTo}).WithError(err).Info("Failed to resolve author")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		Resolvers.Return(resolver)

		if duplicate == 1 {
			h.Log.WithField("create", req.ID).Info("Note is a duplicate")
		} else {
			if err := note.Insert(r.Context(), h.DB, post, h.Log); err != nil {
				h.Log.WithField("create", req.ID).WithError(err).Info("Failed to insert Note")
				w.WriteHeader(http.StatusInternalServerError)
				return
			}
			h.Log.WithField("note", post.ID).Info("Received a new Note")
		}

	default:
		if sender.ID == req.Actor {
			h.Log.WithFields(log.Fields{"sender": sender.ID, "type": req.Type, "body": string(body)}).Warn("Received unknown request")
			w.WriteHeader(http.StatusBadRequest)
		} else {
			h.Log.WithFields(log.Fields{"sender": sender.ID, "actor": req.Actor, "type": req.Type, "body": string(body)}).Warn("Received unknown, unauthorized request")
			w.WriteHeader(http.StatusUnauthorized)
		}
		return
	}

	w.WriteHeader(http.StatusOK)
}
