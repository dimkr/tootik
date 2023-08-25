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
	"github.com/dimkr/tootik/data"
	"github.com/dimkr/tootik/note"
	_ "github.com/mattn/go-sqlite3"
	"log/slog"
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

func processCreateActivity(ctx context.Context, log *slog.Logger, sender *ap.Actor, req *ap.Activity, post *ap.Object, db *sql.DB, resolver *Resolver, from *ap.Actor) error {
	prefix := fmt.Sprintf("https://%s/", cfg.Domain)
	if strings.HasPrefix(sender.ID, prefix) || strings.HasPrefix(post.ID, prefix) || strings.HasPrefix(post.AttributedTo, prefix) || strings.HasPrefix(req.Actor, prefix) {
		return fmt.Errorf("Received invalid Create for %s by %s from %s", post.ID, post.AttributedTo, req.Actor)
	}

	var duplicate int
	if err := db.QueryRowContext(ctx, `select exists (select 1 from notes where id = ?)`, post.ID).Scan(&duplicate); err != nil {
		return fmt.Errorf("Failed to check of %s is a duplicate: %w", post.ID, err)
	} else if duplicate == 1 {
		log.Info("Post is a duplicate")
		return nil
	}

	if _, err := resolver.Resolve(ctx, log, db, from, post.AttributedTo, false); err != nil {
		return fmt.Errorf("Failed to resolve %s: %w", post.AttributedTo, err)
	}

	if err := note.Insert(ctx, log, db, post); err != nil {
		return fmt.Errorf("Cannot insert %s: %w", post.ID, err)
	}
	log.Info("Received a new post")

	mentionedUsers := data.OrderedMap[string, struct{}]{}

	for _, tag := range post.Tag {
		if tag.Type == ap.MentionMention && tag.Href != post.AttributedTo {
			mentionedUsers.Store(tag.Href, struct{}{})
		}
	}

	mentionedUsers.Range(func(id string, _ struct{}) bool {
		if _, err := resolver.Resolve(ctx, log, db, from, id, false); err != nil {
			log.Warn("Failed to resolve mention", "mention", id, "error", err)
		}

		return true
	})

	return nil
}

func processActivity(ctx context.Context, log *slog.Logger, sender *ap.Actor, req *ap.Activity, db *sql.DB, resolver *Resolver, from *ap.Actor) error {
	log.Debug("Processing activity")

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

		log.Info("Received delete request", "deleted", deleted)

		if deleted == sender.ID {
			if _, err := db.ExecContext(ctx, `delete from persons where id = ?`, deleted); err != nil {
				return fmt.Errorf("Failed to delete person %s: %w", req.ID, err)
			}
		} else if _, err := db.ExecContext(ctx, `delete from notes where id = ? and author = ?`, deleted, sender.ID); err != nil {
			return fmt.Errorf("Failed to delete posts by %s", req.ID)
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

		log.Info("Approving follow request", "follower", req.Actor, "followed", followed)

		j, err := json.Marshal(map[string]any{
			"@context": "https://www.w3.org/ns/activitystreams",
			"type":     ap.AcceptActivity,
			"id":       fmt.Sprintf("https://%s/accept/%x", cfg.Domain, sha256.Sum256([]byte(fmt.Sprintf("%s|%s", from.ID, followed)))),
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

		to, err := resolver.Resolve(ctx, log, db, &from, req.Actor, false)
		if err != nil {
			return fmt.Errorf("Failed to resolve %s: %w", req.Actor, err)
		}

		if err := Send(ctx, log, db, &from, resolver, to, j); err != nil {
			return fmt.Errorf("Failed to send Accept response to %s: %w", req.Actor, err)
		}

		if duplicate == 1 {
			log.Info("User is already followed", "follower", req.Actor, "followed", followed)
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
			log.Info("Follow is accepted", "follow", follow)
		} else if followObject, ok := req.Object.(*ap.Object); ok && followObject.Type == ap.FollowObject && followObject.ID != "" {
			log.Info("Follow is accepted", "follow", followObject.ID)
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

		log.Info("Removed a Follow", "follow", follow.ID, "follower", follower)

	case ap.CreateActivity:
		post, ok := req.Object.(*ap.Object)
		if !ok {
			return errors.New("Received invalid Create")
		}

		return processCreateActivity(ctx, log, sender, req, post, db, resolver, from)

	case ap.AnnounceActivity:
		create, ok := req.Object.(*ap.Activity)
		if !ok {
			return errors.New("Received invalid Announce")
		}
		if create.Type != ap.CreateActivity {
			return fmt.Errorf("Received unsupported Announce type: %s", create.Type)
		}

		post, ok := create.Object.(*ap.Object)
		if !ok {
			return errors.New("Received invalid Create")
		}
		if !post.IsPublic() {
			return errors.New("Received Announce for private post")
		}

		return processCreateActivity(ctx, log, sender, create, post, db, resolver, from)

	case ap.UpdateActivity:
		post, ok := req.Object.(*ap.Object)
		if !ok || post.ID == "" || post.AttributedTo == "" {
			return errors.New("Received invalid Update")
		}

		if sender.ID != post.AttributedTo {
			return fmt.Errorf("%s cannot update posts by %s", sender.ID, post.AttributedTo)
		}

		var lastUpdate sql.NullInt64
		if err := db.QueryRowContext(ctx, `select max(inserted, updated) from notes where id = ? and author = ?`, post.ID, post.AttributedTo).Scan(&lastUpdate); err != nil && errors.Is(err, sql.ErrNoRows) {
			log.Info("Received Update for non-existing post")
			return processCreateActivity(ctx, log, sender, req, post, db, resolver, from)
		} else if err != nil {
			return fmt.Errorf("Failed to get last update time for %s: %w", post.ID, err)
		}

		if !lastUpdate.Valid || lastUpdate.Int64 >= post.Updated.UnixNano() {
			return fmt.Errorf("Received old update request for new post: %s", post.ID)
		}

		body, err := json.Marshal(post)
		if err != nil {
			return fmt.Errorf("Failed to update post %s: %w", post.ID, err)
		}

		if _, err := db.ExecContext(
			ctx,
			`update notes set object = ?, updated = unixepoch() where id = ?`,
			string(body),
			post.ID,
		); err != nil {
			return fmt.Errorf("Failed to update post %s: %w", post.ID, err)
		}

		log.Info("Updated post")

	default:
		if sender.ID == req.Actor {
			log.Warn("Received unknown request")
		} else {
			log.Warn("Received unknown, unauthorized request")
		}
	}

	return nil
}

func processActivityWithTimeout(parent context.Context, log *slog.Logger, sender *ap.Actor, activity *ap.Activity, db *sql.DB, resolver *Resolver, from *ap.Actor) {
	ctx, cancel := context.WithTimeout(parent, activityProcessingTimeout)
	defer cancel()

	if o, ok := activity.Object.(*ap.Object); ok {
		log = log.With(slog.Group("activity", "sender", sender.ID, "type", activity.Type, "actor", activity.Actor, slog.Group("object", "kind", "object", "id", o.ID, "type", o.Type, "attributed_to", o.AttributedTo)))
	} else if a, ok := activity.Object.(*ap.Activity); ok {
		log = log.With(slog.Group("activity", "sender", sender.ID, "type", activity.Type, "actor", activity.Actor, slog.Group("object", "kind", "activity", "id", a.ID, "type", a.Type, "actor", a.Actor)))
	} else if s, ok := activity.Object.(string); ok {
		log = log.With(slog.Group("activity", "sender", sender.ID, "type", activity.Type, "actor", activity.Actor, slog.Group("object", "kind", "string", "id", s)))
	}

	if err := processActivity(ctx, log, sender, activity, db, resolver, from); err != nil {
		log.Warn("Failed to process activity", "error", err)
	}
}

func processActivitiesBatch(ctx context.Context, log *slog.Logger, db *sql.DB, resolver *Resolver, from *ap.Actor) (int, error) {
	log.Debug("Polling activities queue")

	rows, err := db.QueryContext(ctx, `select activities.id, persons.actor, activities.activity from (select * from activities limit -1 offset case when (select count(*) from activities) >= $1 then $1/10 else 0 end) activities left join persons on persons.id = activities.sender order by activities.id limit $2`, maxActivitiesQueueSize, activitiesBatchSize)
	if err != nil {
		return 0, fmt.Errorf("Failed to fetch activities to process: %w", err)
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
			log.Error("Failed to scan activity", "error", err)
			continue
		}

		maxID = id

		if !senderString.Valid {
			log.Warn("Sender is unknown", "id", id)
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
			log.Error("Failed to unmarshal activity", "raw", activityString, "error", err)
			return true
		}

		var sender ap.Actor
		if err := json.Unmarshal([]byte(senderString), &sender); err != nil {
			log.Error("Failed to unmarshal actor", "raw", senderString, "error", err)
			return true
		}

		processActivityWithTimeout(ctx, log, &sender, &activity, db, resolver, from)
		return true
	})

	if _, err := db.ExecContext(ctx, `delete from activities where id <= ?`, maxID); err != nil {
		return 0, fmt.Errorf("Failed to delete processed activities: %w", err)
	}

	return rowsCount, nil
}

func processActivities(ctx context.Context, log *slog.Logger, db *sql.DB, resolver *Resolver, from *ap.Actor) error {
	t := time.NewTicker(activitiesBatchDelay)
	defer t.Stop()

	for {
		n, err := processActivitiesBatch(ctx, log, db, resolver, from)
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

func ProcessActivities(ctx context.Context, log *slog.Logger, db *sql.DB, resolver *Resolver, from *ap.Actor) error {
	t := time.NewTicker(activitiesPollingInterval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil

		case <-t.C:
			if err := processActivities(ctx, log, db, resolver, from); err != nil {
				return err
			}
		}
	}
}
