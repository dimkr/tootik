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
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/data"
	"github.com/dimkr/tootik/note"
	_ "github.com/mattn/go-sqlite3"
	log "github.com/sirupsen/logrus"
	"strings"
	"time"
)

const (
	maxActivitiesQueueSize    = 10000
	activitiesBatchSize       = 64
	activitiesPollingInterval = time.Second * 5
	activitiesBatchDelay      = time.Millisecond * 100
	activityProcessingTimeout = time.Second * 15
)

func processsActivity(ctx context.Context, sender *ap.Actor, body []byte, db *sql.DB, logger *log.Logger) error {
	var req ap.Activity
	if err := json.NewDecoder(bytes.NewReader(body)).Decode(&req); err != nil {
		logger.WithField("body", string(body)).WithError(err).Warn("Failed to unmarshal request")
		return err
	}

	id := req.ID

	switch req.Type {
	case ap.DeleteActivity:
		deleted := ""
		if _, ok := req.Object.(*ap.Object); ok {
			deleted = req.Object.(*ap.Object).ID
		} else if s, ok := req.Object.(string); ok {
			deleted = s
		}
		if deleted == "" {
			return errors.New("Received an invalid delete request")
		}

		logger.WithFields(log.Fields{"id": id, "sender": sender.ID, "deleted": deleted}).Info("Received delete request")

		if deleted == sender.ID {
			if _, err := db.ExecContext(ctx, `delete from persons where id =`, deleted); err != nil {
				return fmt.Errorf("Failed to delete person %s", id)
			}
		} else if _, err := db.ExecContext(ctx, `delete from notes where id = ? and author = ?`, deleted, sender.ID); err != nil {
			return fmt.Errorf("Failed to notes by %s", id)
		}

	case ap.FollowActivity:
		if sender.ID != req.Actor {
			return errors.New("Received unauthorized follow request")
		}

		followed, ok := req.Object.(string)
		if !ok {
			return errors.New("Received a request to follow a non-link object")
		}
		if followed == "" {
			return errors.New("Received an invalid follow request")
		}

		prefix := fmt.Sprintf("https://%s/", cfg.Domain)
		if strings.HasPrefix(req.Actor, prefix) || !strings.HasPrefix(followed, prefix) {
			return fmt.Errorf("Received an invalid follow request for %s by %s", followed, req.Actor)
		}

		followedString := ""
		if err := db.QueryRowContext(ctx, `select actor from persons where id = ?`, followed).Scan(&followedString); err != nil {
			return fmt.Errorf("Failed to fetch %s: %w", followed, err)
		}

		from := ap.Actor{}
		if err := json.Unmarshal([]byte(followedString), &from); err != nil {
			return fmt.Errorf("Failed to unmarshal %s: %w", followed, err)
		}

		var duplicate int
		if err := db.QueryRowContext(ctx, `select exists (select 1 from follows where follower = ? and followed = ?)`, req.Actor, followed).Scan(&duplicate); err != nil {
			return fmt.Errorf("Failed to check if %s already follows %s: %w", req.Actor, followed, err)
		}

		logger.WithFields(log.Fields{"follower": req.Actor, "followed": followed}).Info("Approving follow request")

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
			return fmt.Errorf("Failed to marshal accept response: %w", err)
		}

		resolver, err := Resolvers.Borrow(ctx)
		if err != nil {
			return fmt.Errorf("Cannot resolve %s: %w", req.Actor, err)
		}

		to, err := resolver.Resolve(ctx, db, &from, req.Actor)
		if err != nil {
			Resolvers.Return(resolver)
			return fmt.Errorf("Failed to resolve %s: %w", req.Actor, err)
		}

		if err := Send(ctx, db, &from, resolver, to, j); err != nil {
			Resolvers.Return(resolver)
			return fmt.Errorf("Failed to send Accept response to %s: %w", req.Actor, err)
		}

		Resolvers.Return(resolver)

		if duplicate == 1 {
			logger.WithFields(log.Fields{"follower": req.Actor, "followed": followed, "dupicate": duplicate}).Info("User is already followed")
		} else {
			if _, err := db.ExecContext(
				ctx,
				`INSERT INTO follows (id, follower, followed ) VALUES(?,?,?)`,
				req.ID,
				req.Actor,
				followed,
			); err != nil {
				return fmt.Errorf("Failed to insert follow %s: %w", req.ID, err)
			}
		}

	case ap.AcceptActivity:
		if sender.ID != req.Actor {
			return fmt.Errorf("Received an invalid follow request for %s by %s", req.Actor, sender.ID)
		}

		if follow, ok := req.Object.(string); ok && follow != "" {
			logger.WithFields(log.Fields{"sender": sender.ID, "actor": req.Actor, "follow": follow}).Info("Follow is accepted")
		} else if followObject, ok := req.Object.(*ap.Object); ok && followObject.Type == ap.FollowObject && followObject.ID != "" {
			logger.WithFields(log.Fields{"sender": sender.ID, "actor": req.Actor, "follow": followObject.ID}).Info("Follow is accepted")
		} else {
			return errors.New("Received an invalid accept notification")
		}

	case ap.UndoActivity:
		if sender.ID != req.Actor {
			return fmt.Errorf("Received an invalid undo request for %s by %s", req.Actor, sender.ID)
		}

		follow, ok := req.Object.(*ap.Object)
		if !ok {
			return errors.New("Received a request to undo a non-object object")
		}
		if follow.Type != ap.FollowObject {
			return errors.New("Received a request to undo a non-Follow object")
		}
		if follow.ID == "" {
			return errors.New("Received an undo request with empty ID")
		}

		follower := req.Actor
		if _, err := db.ExecContext(ctx, `delete from follows where id = ? and follower = ?`, follow.ID, follower); err != nil {
			return fmt.Errorf("Failed to remove follow %s: %w", follow.ID, err)
		}

		logger.WithFields(log.Fields{"follow": follow.ID, "follower": follower}).Info("Removed a Follow")

	case ap.CreateActivity:
		post, ok := req.Object.(*ap.Object)
		if !ok {
			return errors.New("Received invalid Create")
		}

		prefix := fmt.Sprintf("https://%s/", cfg.Domain)
		if strings.HasPrefix(sender.ID, prefix) || strings.HasPrefix(post.ID, prefix) || strings.HasPrefix(post.AttributedTo, prefix) || strings.HasPrefix(req.Actor, prefix) {
			return fmt.Errorf("Received invalid Create for %s by %s from %s", post.ID, post.AttributedTo, req.Actor)
		}

		var duplicate int
		if err := db.QueryRowContext(ctx, `select exists (select 1 from notes where id = ?)`, post.ID).Scan(&duplicate); err != nil {
			return fmt.Errorf("Failed to check of %s is a duplicate: %w", post.ID, err)
		} else if duplicate == 1 {
			logger.WithField("create", req.ID).Info("Note is a duplicate")
			return nil
		}

		resolver, err := Resolvers.Borrow(ctx)
		if err != nil {
			return fmt.Errorf("Cannot resolve %s: %w", post.AttributedTo, err)
		}

		if _, err := resolver.Resolve(ctx, db, nil, post.AttributedTo); err != nil {
			Resolvers.Return(resolver)
			return fmt.Errorf("Failed to resolve %s: %w", post.AttributedTo, err)
		}

		Resolvers.Return(resolver)

		if err := note.Insert(ctx, db, post, logger); err != nil {
			return fmt.Errorf("Cannot insert %s: %w", post.ID, err)
		}
		logger.WithField("note", post.ID).Info("Received a new Note")

		mentionedUsers := data.OrderedMap[string, struct{}]{}

		for _, tag := range post.Tag {
			if tag.Type == ap.MentionMention && tag.Href != post.AttributedTo {
				mentionedUsers.Store(tag.Href, struct{}{})
			}
		}

		mentionedUsers.Range(func(id string, _ struct{}) bool {
			resolver, err := Resolvers.Borrow(ctx)
			if err != nil {
				logger.WithFields(log.Fields{"note": post.ID, "mention": id}).WithError(err).Warn("Cannot resolve mention")
				return true
			}

			if _, err := resolver.Resolve(ctx, db, nil, post.AttributedTo); err != nil {
				Resolvers.Return(resolver)
				logger.WithFields(log.Fields{"note": post.ID, "mention": id}).WithError(err).Warn("Failed to resolve mention")
				return true
			}

			Resolvers.Return(resolver)
			return true
		})

	default:
		if sender.ID == req.Actor {
			logger.WithFields(log.Fields{"sender": sender.ID, "type": req.Type, "body": string(body)}).Warn("Received unknown request")
		} else {
			logger.WithFields(log.Fields{"sender": sender.ID, "actor": req.Actor, "type": req.Type, "body": string(body)}).Warn("Received unknown, unauthorized request")
		}
	}

	return nil
}

func processsActivityWithTimeout(parent context.Context, sender *ap.Actor, body []byte, db *sql.DB, logger *log.Logger) {
	ctx, cancel := context.WithTimeout(parent, activityProcessingTimeout)
	defer cancel()
	processsActivity(ctx, sender, body, db, logger)
}

func processsActivitiesBatch(ctx context.Context, db *sql.DB, logger *log.Logger) (int, error) {
	logger.Debug("Polling activities queue")

	rows, err := db.QueryContext(ctx, `select activities.id, persons.actor, activities.activity from (select * from activities limit -1 offset case when (select count(*) from activities) >= $1 then $1/10 else 0 end) activities left join persons on persons.id = activities.sender order by activities.id limit $2`, maxActivitiesQueueSize, activitiesBatchSize)
	if err != nil {
		return 0, fmt.Errorf("Failed to fetch activities to processs: %w", err)
	}
	defer rows.Close()

	activities := data.OrderedMap[string, string]{}
	var maxID int64
	var rowsCount int

	for rows.Next() {
		rowsCount += 1

		var id int64
		var activityString string
		var senderString sql.NullString
		if err := rows.Scan(&id, &senderString, &activityString); err != nil {
			logger.WithError(err).Error("Failed to scan activity")
			continue
		}

		maxID = id

		if !senderString.Valid {
			logger.WithField("id", id).Warn("Sender is unknown")
			continue
		}

		activities.Store(activityString, senderString.String)
	}
	rows.Close()

	if len(activities) == 0 {
		return 0, nil
	}

	activities.Range(func(activityString, senderString string) bool {
		var activity ap.Activity
		if err := json.Unmarshal([]byte(activityString), &activity); err != nil {
			logger.WithError(err).Error("Failed to unmarshal activity")
			return true
		}

		var sender ap.Actor
		if err := json.Unmarshal([]byte(senderString), &sender); err != nil {
			logger.WithError(err).Error("Failed to unmarshal actor")
			return true
		}

		logger.WithFields(log.Fields{"sender": sender.ID, "activity": activity.ID, "type": activity.Type}).Debug("Processing activity")

		if err := processsActivity(ctx, &sender, []byte(activityString), db, logger); err != nil {
			if _, ok := activity.Object.(*ap.Object); ok {
				logger.WithFields(log.Fields{"sender": sender.ID, "activity": activity.ID, "type": activity.Type, "object": activity.Object.(*ap.Object).ID}).WithError(err).Warn("Failed to process activity")
			} else {
				logger.WithFields(log.Fields{"sender": sender.ID, "activity": activity.ID, "type": activity.Type, "object": activity.Object.(string)}).WithError(err).Warn("Failed to process activity")
			}
		}

		return true
	})

	if _, err := db.ExecContext(ctx, `delete from activities where id <= ?`, maxID); err != nil {
		return 0, fmt.Errorf("Failed to delete processed activities: %w", err)
	}

	return rowsCount, nil
}

func processsActivities(ctx context.Context, db *sql.DB, logger *log.Logger) error {
	t := time.NewTicker(activitiesBatchDelay)
	defer t.Stop()

	for {
		n, err := processsActivitiesBatch(ctx, db, logger)
		if err != nil {
			return err
		}

		if n < activitiesBatchSize {
			return nil
		}

		select {
		case <-ctx.Done():
			return nil

		case <-t.C:
		}
	}
}

func ProcessActivities(ctx context.Context, db *sql.DB, logger *log.Logger) error {
	t := time.NewTicker(activitiesPollingInterval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil

		case <-t.C:
			if err := processsActivities(ctx, db, logger); err != nil {
				return err
			}
		}
	}
}
