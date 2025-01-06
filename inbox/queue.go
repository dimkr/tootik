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

package inbox

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/data"
	"github.com/dimkr/tootik/fed"
	"github.com/dimkr/tootik/httpsig"
	"github.com/dimkr/tootik/inbox/note"
	"github.com/dimkr/tootik/outbox"
	"log/slog"
	"net/url"
	"strings"
	"time"
)

type Queue struct {
	Domain    string
	Config    *cfg.Config
	BlockList *fed.BlockList
	DB        *sql.DB
	Resolver  ap.Resolver
	Key       httpsig.Key
}

type batchItem struct {
	Activity    *ap.Activity
	RawActivity string
	Sender      *ap.Actor
}

const maxActivityDepth = 3

var ErrActivityTooNested = errors.New("exceeded activity depth limit")

func processCreateActivity[T ap.RawActivity](ctx context.Context, q *Queue, log *slog.Logger, sender *ap.Actor, activity *ap.Activity, rawActivity T, post *ap.Object, shared bool) error {
	prefix := fmt.Sprintf("https://%s/", q.Domain)
	if strings.HasPrefix(sender.ID, prefix) || strings.HasPrefix(post.ID, prefix) || strings.HasPrefix(post.AttributedTo, prefix) || strings.HasPrefix(activity.Actor, prefix) {
		return fmt.Errorf("received invalid Create for %s by %s from %s", post.ID, post.AttributedTo, activity.Actor)
	}

	u, err := url.Parse(post.ID)
	if err != nil {
		return fmt.Errorf("failed to parse post ID %s: %w", post.ID, err)
	}

	if !data.IsIDValid(u) {
		return fmt.Errorf("received invalid post ID: %s", post.ID)
	}

	if q.BlockList != nil && q.BlockList.Contains(u.Host) {
		return fmt.Errorf("ignoring post %s: %w", post.ID, fed.ErrBlockedDomain)
	}

	if len(post.To.OrderedMap)+len(post.CC.OrderedMap) > q.Config.MaxRecipients {
		log.Warn("Post has too many recipients", "to", len(post.To.OrderedMap), "cc", len(post.CC.OrderedMap))
		return nil
	}

	var audience sql.NullString
	if err := q.DB.QueryRowContext(ctx, `select object->>'$.audience' from notes where id = ?`, post.ID).Scan(&audience); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("failed to check of %s is a duplicate: %w", post.ID, err)
	} else if err == nil {
		if sender.ID == post.Audience && !audience.Valid {
			tx, err := q.DB.BeginTx(ctx, nil)
			if err != nil {
				return fmt.Errorf("cannot set %s audience: %w", post.ID, err)
			}
			defer tx.Rollback()

			if _, err := tx.ExecContext(ctx, `update notes set object = json_set(object, '$.audience', ?) where id = ? and object->>'$.audience' is null`, post.Audience, post.ID); err != nil {
				return fmt.Errorf("failed to set %s audience to %s: %w", post.ID, audience.String, err)
			}

			if _, err := tx.ExecContext(ctx, `update feed set note = json_set(note, '$.audience', ?) where note->>'$.id' = ? and note->>'$.audience' is null`, post.Audience, post.ID); err != nil {
				return fmt.Errorf("failed to set %s audience to %s: %w", post.ID, audience.String, err)
			}

			if shared {
				if _, err := tx.ExecContext(
					ctx,
					`INSERT OR IGNORE INTO shares (note, by, activity) VALUES(?,?,?)`,
					post.ID,
					sender.ID,
					activity.ID,
				); err != nil {
					return fmt.Errorf("cannot insert share for %s by %s: %w", post.ID, sender.ID, err)
				}
			}

			if err := tx.Commit(); err != nil {
				return fmt.Errorf("cannot set %s audience: %w", post.ID, err)
			}
		} else if shared {
			if _, err := q.DB.ExecContext(
				ctx,
				`INSERT OR IGNORE INTO shares (note, by, activity) VALUES(?,?,?)`,
				post.ID,
				sender.ID,
				activity.ID,
			); err != nil {
				return fmt.Errorf("cannot insert share for %s by %s: %w", post.ID, sender.ID, err)
			}
		}

		log.Debug("Post is a duplicate")
		return nil
	}

	if _, err := q.Resolver.ResolveID(ctx, q.Key, post.AttributedTo, 0); err != nil {
		return fmt.Errorf("failed to resolve %s: %w", post.AttributedTo, err)
	}

	tx, err := q.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("cannot insert %s: %w", post.ID, err)
	}
	defer tx.Rollback()

	// only the group itself has the authority to decide which posts belong to it
	if post.Audience != sender.ID {
		post.Audience = ""
	}

	if err := note.Insert(ctx, tx, post); err != nil {
		return fmt.Errorf("cannot insert %s: %w", post.ID, err)
	}

	if shared {
		if _, err := tx.ExecContext(
			ctx,
			`INSERT OR IGNORE INTO shares (note, by, activity) VALUES(?,?,?)`,
			post.ID,
			sender.ID,
			activity.ID,
		); err != nil {
			return fmt.Errorf("cannot insert share for %s by %s: %w", post.ID, sender.ID, err)
		}
	}

	if err := outbox.ForwardActivity(ctx, q.Domain, q.Config, tx, post, activity, rawActivity); err != nil {
		return fmt.Errorf("cannot forward %s: %w", post.ID, err)
	}

	log.Info("Received a new post")

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("cannot insert %s: %w", post.ID, err)
	}

	mentionedUsers := ap.Audience{}

	for _, tag := range post.Tag {
		if tag.Type == ap.Mention && tag.Href != post.AttributedTo {
			mentionedUsers.Add(tag.Href)
		}
	}

	for id := range mentionedUsers.Keys() {
		if _, err := q.Resolver.ResolveID(ctx, q.Key, id, 0); err != nil {
			log.Warn("Failed to resolve mention", "mention", id, "error", err)
		}
	}

	return nil
}

func processActivity[T ap.RawActivity](ctx context.Context, q *Queue, log *slog.Logger, sender *ap.Actor, activity *ap.Activity, rawActivity T, depth int, shared bool) error {
	if depth == maxActivityDepth {
		return ErrActivityTooNested
	}

	log.Debug("Processing activity")

	switch activity.Type {
	case ap.Delete:
		deleted := ""
		if _, ok := activity.Object.(*ap.Object); ok {
			deleted = activity.Object.(*ap.Object).ID
		} else if s, ok := activity.Object.(string); ok {
			deleted = s
		}
		if deleted == "" {
			return errors.New("received an invalid delete activity")
		}

		log.Info("Received delete request", "deleted", deleted)

		if deleted == activity.Actor {
			if _, err := q.DB.ExecContext(ctx, `delete from persons where id = ?`, deleted); err != nil {
				return fmt.Errorf("failed to delete person %s: %w", deleted, err)
			}
		} else {
			tx, err := q.DB.BeginTx(ctx, nil)
			if err != nil {
				return fmt.Errorf("cannot delete %s: %w", deleted, err)
			}
			defer tx.Rollback()

			var note ap.Object
			if err := q.DB.QueryRowContext(ctx, `select object from notes where id = ?`, deleted).Scan(&note); err != nil && errors.Is(err, sql.ErrNoRows) {
				log.Debug("Received delete request for non-existing post", "deleted", deleted)
				return nil
			} else if err != nil {
				return fmt.Errorf("failed to delete %s: %w", deleted, err)
			}

			if err := outbox.ForwardActivity(ctx, q.Domain, q.Config, tx, &note, activity, rawActivity); err != nil {
				return fmt.Errorf("failed to delete %s: %w", deleted, err)
			}

			if _, err := tx.ExecContext(ctx, `delete from notesfts where id = ?`, deleted); err != nil {
				return fmt.Errorf("cannot delete %s: %w", deleted, err)
			}
			if _, err := tx.ExecContext(ctx, `delete from notes where id = ?`, deleted); err != nil {
				return fmt.Errorf("cannot delete %s: %w", deleted, err)
			}
			if _, err := tx.ExecContext(ctx, `delete from shares where note = ?`, deleted); err != nil {
				return fmt.Errorf("cannot delete %s: %w", deleted, err)
			}
			if _, err := tx.ExecContext(ctx, `delete from feed where note->>'$.id' = ?`, deleted); err != nil {
				return fmt.Errorf("cannot delete %s: %w", deleted, err)
			}

			if err := tx.Commit(); err != nil {
				return fmt.Errorf("failed to delete %s: %w", deleted, err)
			}
		}

	case ap.Follow:
		followed, ok := activity.Object.(string)
		if !ok {
			return errors.New("received a request to follow a non-link object")
		}
		if followed == "" {
			return errors.New("received an invalid follow request")
		}

		prefix := fmt.Sprintf("https://%s/", q.Domain)
		if strings.HasPrefix(activity.Actor, prefix) || !strings.HasPrefix(followed, prefix) {
			return fmt.Errorf("received an invalid follow request for %s by %s", followed, activity.Actor)
		}

		var from ap.Actor
		if err := q.DB.QueryRowContext(ctx, `select actor from persons where id = ?`, followed).Scan(&from); err != nil {
			return fmt.Errorf("failed to fetch %s: %w", followed, err)
		}

		log.Info("Approving follow request", "follower", activity.Actor, "followed", followed)

		if err := outbox.Accept(ctx, q.Domain, followed, activity.Actor, activity.ID, q.DB); err != nil {
			return fmt.Errorf("failed to marshal accept response: %w", err)
		}

	case ap.Accept:
		followID, ok := activity.Object.(string)
		if ok && followID != "" {
			log.Info("Follow is accepted", "follow", followID)
		} else if followActivity, ok := activity.Object.(*ap.Activity); ok && followActivity.Type == ap.Follow && followActivity.ID != "" {
			log.Info("Follow is accepted", "follow", followActivity.ID)
			followID = followActivity.ID
		} else {
			return errors.New("received an invalid accept notification")
		}

		if _, err := q.DB.ExecContext(ctx, `update follows set accepted = 1 where id = ? and followed = ?`, followID, activity.Actor); err != nil {
			return fmt.Errorf("failed to accept follow %s: %w", followID, err)
		}

	case ap.Undo:
		inner, ok := activity.Object.(*ap.Activity)
		if !ok {
			return errors.New("received a request to undo a non-activity object")
		}

		if inner.Type == ap.Announce {
			noteID, ok := inner.Object.(string)
			if !ok {
				return errors.New("cannot undo Announce")
			}
			if _, err := q.DB.ExecContext(
				ctx,
				`delete from shares where note = ? and by = ?`,
				noteID,
				activity.Actor,
			); err != nil {
				return fmt.Errorf("failed to remove share for %s by %s: %w", noteID, activity.Actor, err)
			}
			return nil
		}

		if inner.Type != ap.Follow {
			log.Debug("Ignoring request to undo a non-Follow activity")
			return nil
		}

		follower := activity.Actor

		var followed string
		if actor, ok := inner.Object.(*ap.Object); ok {
			followed = actor.ID
		} else if actorID, ok := inner.Object.(string); ok {
			followed = actorID
		} else {
			return errors.New("received a request to undo follow on unknown object")
		}
		if followed == "" {
			return errors.New("received an undo request with empty ID")
		}

		prefix := fmt.Sprintf("https://%s/", q.Domain)
		if strings.HasPrefix(follower, prefix) {
			return errors.New("received an undo request from local actor")
		}
		if !strings.HasPrefix(followed, prefix) {
			return errors.New("received an undo request on federated actor")
		}

		if _, err := q.DB.ExecContext(ctx, `delete from follows where follower = ? and followed = ?`, follower, followed); err != nil {
			return fmt.Errorf("failed to remove follow of %s by %s: %w", followed, follower, err)
		}

		log.Info("Removed a Follow", "follower", follower, "followed", followed)

	case ap.Create:
		post, ok := activity.Object.(*ap.Object)
		if !ok {
			return errors.New("received invalid Create")
		}

		return processCreateActivity(ctx, q, log, sender, activity, rawActivity, post, shared)

	case ap.Announce:
		inner, ok := activity.Object.(*ap.Activity)
		if !ok {
			if postID, ok := activity.Object.(string); ok && postID != "" {
				if _, err := q.DB.ExecContext(
					ctx,
					`INSERT OR IGNORE INTO shares (note, by, activity) VALUES(?,?,?)`,
					postID,
					sender.ID,
					activity.ID,
				); err != nil {
					return fmt.Errorf("cannot insert share for %s by %s: %w", postID, sender.ID, err)
				}
			} else {
				log.Debug("Ignoring unsupported Announce object")
			}
			return nil
		}

		depth++
		return processActivity(ctx, q, log.With("activity", inner, "depth", depth), sender, inner, rawActivity, depth, true)

	case ap.Update:
		post, ok := activity.Object.(*ap.Object)
		if !ok || post.ID == activity.Actor || post.ID == sender.ID {
			log.Debug("Ignoring unsupported Update object")
			return nil
		}

		if post.ID == "" || post.AttributedTo == "" {
			return errors.New("received invalid Update")
		}

		var oldPost ap.Object
		var lastChange int64
		if err := q.DB.QueryRowContext(ctx, `select max(inserted, updated), object from notes where id = ? and author = ?`, post.ID, post.AttributedTo).Scan(&lastChange, &oldPost); err != nil && errors.Is(err, sql.ErrNoRows) {
			log.Debug("Received Update for non-existing post")
			return processCreateActivity(ctx, q, log, sender, activity, rawActivity, post, shared)
		} else if err != nil {
			return fmt.Errorf("failed to get last update time for %s: %w", post.ID, err)
		}

		// if specified, prefer post publication or editing time to insertion or last update time
		var sec int64
		if oldPost.Updated != nil {
			sec = oldPost.Updated.Unix()
		}
		if sec == 0 {
			sec = oldPost.Published.Unix()
		}
		if sec > 0 {
			lastChange = sec
		}

		if (post.Type == ap.Question && post.Updated != nil && lastChange >= post.Updated.Unix()) || (post.Type != ap.Question && (post.Updated == nil || lastChange >= post.Updated.Unix())) {
			log.Debug("Received old update request for new post")
			return nil
		}

		// only the group can decide if audience has changed
		if sender.ID != oldPost.Audience {
			post.Audience = oldPost.Audience
		}

		tx, err := q.DB.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("cannot insert %s: %w", post.ID, err)
		}
		defer tx.Rollback()

		if _, err := tx.ExecContext(
			ctx,
			`update notes set object = ?, updated = unixepoch() where id = ?`,
			post,
			post.ID,
		); err != nil {
			return fmt.Errorf("failed to update post %s: %w", post.ID, err)
		}

		if post.Content != oldPost.Content {
			if _, err := tx.ExecContext(
				ctx,
				`update notesfts set content = ? where id = ?`,
				note.Flatten(post),
				post.ID,
			); err != nil {
				return fmt.Errorf("failed to update post %s: %w", post.ID, err)
			}
		}

		if _, err := tx.ExecContext(
			ctx,
			`update feed set note = ? where note->>'$.id' = ?`,
			post,
			post.ID,
		); err != nil {
			return fmt.Errorf("failed to update post %s: %w", post.ID, err)
		}

		if err := outbox.ForwardActivity(ctx, q.Domain, q.Config, tx, post, activity, rawActivity); err != nil {
			return fmt.Errorf("failed to forward update post %s: %w", post.ID, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to update post %s: %w", post.ID, err)
		}

		log.Info("Updated post")

	case ap.Move:
		log.Debug("Ignoring Move activity")

	case ap.Like:
		log.Debug("Ignoring Like activity")

	case ap.Dislike:
		log.Debug("Ignoring Dislike activity")

	default:
		if sender.ID == activity.Actor {
			log.Warn("Received unknown request")
		} else {
			log.Warn("Received unknown, unauthorized request")
		}
	}

	return nil
}

func (q *Queue) processActivityWithTimeout(parent context.Context, sender *ap.Actor, activity *ap.Activity, rawActivity data.JSON) {
	ctx, cancel := context.WithTimeout(parent, q.Config.ActivityProcessingTimeout)
	defer cancel()

	log := slog.With("activity", activity, "sender", sender.ID)
	if err := processActivity(ctx, q, log, sender, activity, rawActivity, 1, false); err != nil {
		log.Warn("Failed to process activity", "error", err)
	}
}

// ProcessBatch processes one batch of incoming activites in the queue.
func (q *Queue) ProcessBatch(ctx context.Context) (int, error) {
	slog.Debug("Polling activities queue")

	rows, err := q.DB.QueryContext(ctx, `select inbox.id, persons.actor, inbox.activity, inbox.raw from (select * from inbox limit -1 offset case when (select count(*) from inbox) >= $1 then $1/10 else 0 end) inbox left join persons on persons.id = inbox.sender order by inbox.id limit $2`, q.Config.MaxActivitiesQueueSize, q.Config.ActivitiesBatchSize)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch activities to process: %w", err)
	}
	defer rows.Close()

	batch := make([]batchItem, 0, q.Config.ActivitiesBatchSize)
	var maxID int64
	var rowsCount int

	for rows.Next() {
		rowsCount += 1

		var id int64
		var activityString string
		var activity ap.Activity
		var sender sql.Null[ap.Actor]
		if err := rows.Scan(&id, &sender, &activity, &activityString); err != nil {
			slog.Error("Failed to scan activity", "error", err)
			continue
		}

		maxID = id

		if !sender.Valid {
			slog.Warn("Sender is unknown", "id", id)
			continue
		}

		batch = append(batch, batchItem{
			Activity:    &activity,
			RawActivity: activityString,
			Sender:      &sender.V,
		})
	}
	rows.Close()

	if len(batch) == 0 {
		return 0, nil
	}

	for _, item := range batch {
		q.processActivityWithTimeout(ctx, item.Sender, item.Activity, data.JSON(item.RawActivity))
	}

	if _, err := q.DB.ExecContext(ctx, `delete from inbox where id <= ?`, maxID); err != nil {
		return 0, fmt.Errorf("failed to delete processed activities: %w", err)
	}

	return rowsCount, nil
}

func (q *Queue) process(ctx context.Context) error {
	t := time.NewTicker(q.Config.ActivitiesBatchDelay)
	defer t.Stop()

	for {
		n, err := q.ProcessBatch(ctx)
		if err != nil {
			return err
		}

		if n < q.Config.ActivitiesBatchSize {
			return nil
		}

		select {
		case <-ctx.Done():
			return nil

		case <-t.C:
		}
	}
}

// Process polls the queue of incoming activities and processes them.
func (q *Queue) Process(ctx context.Context) error {
	t := time.NewTicker(q.Config.ActivitiesPollingInterval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil

		case <-t.C:
			if err := q.process(ctx); err != nil {
				return err
			}
		}
	}
}
