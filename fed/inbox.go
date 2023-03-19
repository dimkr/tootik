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
	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/data"
	"github.com/go-ap/activitypub"
	_ "github.com/mattn/go-sqlite3"
	log "github.com/sirupsen/logrus"
	"io/ioutil"
	"net/http"
)

func inboxHandler(w http.ResponseWriter, r *http.Request, db *sql.DB) {
	if r.Method != http.MethodPost {
		w.WriteHeader(http.StatusMethodNotAllowed)
		return
	}

	body, err := ioutil.ReadAll(r.Body)
	if err != nil {
		return
	}

	var req activitypub.Activity
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&req); err != nil {
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	if req.Type == activitypub.DeleteType {
		deleted := string(req.ID.GetLink())
		_, err = db.Exec(`delete from objects where id = ? or actor = ?`, deleted, deleted)
		if err != nil {
			log.WithField("id", deleted).WithError(err).Warn("Failed to delete objects")
		}
		w.WriteHeader(http.StatusOK)
		return
	}

	r.Body = ioutil.NopCloser(bytes.NewReader(body))

	actorID := string(req.Actor.GetLink())
	if err := verify(r.Context(), actorID, r, db); err != nil {
		log.WithField("actor", actorID).WithError(err).Warn("Failed to verify message")
		w.WriteHeader(http.StatusUnauthorized)
		return
	}

	switch req.Type {
	case activitypub.FollowType:
		if !req.Object.IsLink() {
			log.Info("Received a request to follow a non-link object")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		followed := string(req.Object.GetLink())
		if followed == "" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		u, err := data.Objects.GetByID(followed, db)
		if err != nil {
			log.WithFields(log.Fields{"follower": req.Actor, "followed": followed}).WithError(err).Info("Received request to follow a non-existing user")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		duplicate := ""
		err = db.QueryRow(`select id from objects where type = "Follow" and actor = ? and object = ?`, string(req.Actor.GetLink()), followed).Scan(&duplicate)
		if err == nil {
			log.WithFields(log.Fields{"follower": req.Actor, "followed": followed, "dupicate": duplicate}).Info("User is already followed")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}
		if !errors.Is(err, sql.ErrNoRows) {
			log.WithFields(log.Fields{"follower": req.Actor, "followed": followed}).WithError(err).Warn("Failed to check if user is already followed")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		log.WithFields(log.Fields{"follower": req.Actor, "followed": followed}).Info("Approving follow request")

		j, err := json.Marshal(map[string]any{
			"@context": "https://www.w3.org/ns/activitystreams",
			"type":     "Accept",
			"id":       fmt.Sprintf("https://%s/accept/%x", cfg.Domain, sha256.Sum256(body)),
			"actor":    followed,
			"to":       []string{string(req.Actor.GetLink())},
			"object": map[string]any{
				"type": "Follow",
				"id":   req.ID,
			},
		})
		if err != nil {
			log.WithFields(log.Fields{"follower": req.Actor, "followed": followed}).WithError(err).Info("Failed to marshal Accept response")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		if err := Send(r.Context(), db, u, string(req.Actor.GetLink()), string(j)); err != nil {
			log.WithFields(log.Fields{"follower": req.Actor, "followed": followed}).WithError(err).Info("Failed to send Accept response")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		follow := data.Object{
			ID:     string(req.ID),
			Type:   string(req.Type),
			Actor:  string(req.Actor.GetLink()),
			Object: followed,
		}

		if err := data.Objects.Insert(db, &follow); err != nil {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte(err.Error()))
			return
		}

	case activitypub.AcceptType:
		// do nothing

	case activitypub.UndoType:
		if !req.Object.IsObject() {
			log.Info("Received a request to undo a non-object object")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		if req.Object.GetType() != activitypub.FollowType {
			log.Info("Received a request to undo a non-Follow object")
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		follow := string(req.Object.GetID())
		if follow == "" {
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		followed := string(req.Actor.GetLink())
		_, err := db.Exec(`delete from objects where id = ? and type = 'Follow' and actor = ?`, follow, followed)
		if err != nil {
			log.WithFields(log.Fields{"follow": follow, "followed": followed}).WithError(err).Info("Received request to undo a non-existing Follow")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		log.WithFields(log.Fields{"follow": follow, "followed": followed}).Info("Removed a Follow")

	case activitypub.CreateType:
		note, ok := req.Object.(*activitypub.Object)
		if !ok {
			log.WithField("create", string(req.ID.GetLink())).WithError(err).Info("Received invalid Create")
			w.WriteHeader(http.StatusBadRequest)
			return
		}

		body, err := json.Marshal(note)
		if err != nil {
			log.WithField("create", string(req.ID.GetLink())).WithError(err).Info("Failed to marshal Note")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

		post := data.Object{
			ID:     string(note.GetLink()),
			Type:   string(note.Type),
			Actor:  string(note.AttributedTo.GetLink()),
			Object: string(body),
		}

		if err := data.Objects.Insert(db, &post); err != nil {
			log.WithField("create", string(req.ID.GetLink())).WithError(err).Info("Failed to insert Note")
			w.WriteHeader(http.StatusInternalServerError)
			return
		}

	default:
		log.WithFields(log.Fields{"actor": actorID, "body": string(body)}).Warn("Received unknown request")
		w.WriteHeader(http.StatusBadRequest)
		return
	}

	w.WriteHeader(http.StatusOK)
}
