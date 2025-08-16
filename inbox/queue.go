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
	"log/slog"
	"strings"
	"time"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/fed"
	"github.com/dimkr/tootik/httpsig"
	"github.com/dimkr/tootik/inbox/note"
	"github.com/dimkr/tootik/outbox"
)

type Queue struct {
	Domain    string
	Config    *cfg.Config
	BlockList *fed.BlockList
	DB        *sql.DB
	Resolver  ap.Resolver
	Keys      [2]httpsig.Key
}

type batchItem struct {
	Activity    *ap.Activity
	RawActivity string
	Sender      *ap.Actor
	Shared      bool
}

var ErrActivityTooNested = errors.New("exceeded activity depth limit")

func (q *Queue) processCreateActivity(ctx context.Context, log *slog.Logger, sender *ap.Actor, activity *ap.Activity, rawActivity string, post *ap.Object, shared bool) error {
	prefix := fmt.Sprintf("https://%s/", q.Domain)
	if strings.HasPrefix(sender.ID, prefix) || strings.HasPrefix(post.ID, prefix) || strings.HasPrefix(post.AttributedTo, prefix) || strings.HasPrefix(activity.Actor, prefix) {
		return fmt.Errorf("received invalid Create for %s by %s from %s", post.ID, post.AttributedTo, activity.Actor)
	}

	post.ID = ap.Canonicalize(post.ID)

	origin, err := ap.GetOrigin(post.ID)
	if err != nil {
		return fmt.Errorf("failed to parse post ID %s: %w", post.ID, err)
	}

	if q.BlockList != nil && q.BlockList.Contains(origin) {
		return fmt.Errorf("ignoring post %s: %w", post.ID, fed.ErrBlockedDomain)
	}

	/*
		if !ap.IsPortable(post.ID) && !data.IsIDValid(post.ID) {
			return fmt.Errorf("received invalid post ID: %s", post.ID)
		}
	*/

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

			if _, err := tx.ExecContext(ctx, `update notes set object = jsonb_set(object, '$.audience', ?) where id = ? and object->>'$.audience' is null`, post.Audience, post.ID); err != nil {
				return fmt.Errorf("failed to set %s audience to %s: %w", post.ID, audience.String, err)
			}

			if _, err := tx.ExecContext(ctx, `update feed set note = jsonb_set(note, '$.audience', ?) where note->>'$.id' = ? and note->>'$.audience' is null`, post.Audience, post.ID); err != nil {
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

	if _, err := q.Resolver.ResolveID(ctx, q.Keys, post.AttributedTo, 0); err != nil {
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

	if err := outbox.ForwardActivity(ctx, q.Config, tx, post, activity, rawActivity); err != nil {
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
		if _, err := q.Resolver.ResolveID(ctx, q.Keys, id, 0); err != nil {
			log.Warn("Failed to resolve mention", "mention", id, "error", err)
		}
	}

	return nil
}

func (q *Queue) processActivity(ctx context.Context, log *slog.Logger, sender *ap.Actor, activity *ap.Activity, rawActivity string, depth int, shared bool) error {
	if depth == ap.MaxActivityDepth {
		return ErrActivityTooNested
	}

	log.Debug("Processing activity")

	activity.ID = ap.Canonicalize(activity.ID)
	activity.Actor = ap.Canonicalize(activity.Actor)

	switch activity.Type {
	case ap.Delete:
		deleted := ""
		if _, ok := activity.Object.(*ap.Object); ok {
			deleted = ap.Canonicalize(activity.Object.(*ap.Object).ID)
		} else if s, ok := activity.Object.(string); ok {
			deleted = ap.Canonicalize(s)
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
			if err := q.DB.QueryRowContext(ctx, `select json(object) from notes where id = ?`, deleted).Scan(&note); err != nil && errors.Is(err, sql.ErrNoRows) {
				log.Debug("Received delete request for non-existing post", "deleted", deleted)
				return nil
			} else if err != nil {
				return fmt.Errorf("failed to delete %s: %w", deleted, err)
			}

			if err := outbox.ForwardActivity(ctx, q.Config, tx, &note, activity, rawActivity); err != nil {
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
		if sender.ID != activity.Actor {
			return errors.New("received unauthorized follow request")
		}

		followed, ok := activity.Object.(string)
		if !ok {
			return errors.New("received a request to follow a non-link object")
		}
		if followed == "" {
			return errors.New("received an invalid follow request")
		}
		followed = ap.Canonicalize(followed)

		var manual sql.NullInt32
		if err := q.DB.QueryRowContext(ctx, `select actor->>'$.manuallyApprovesFollowers' from persons where id = ?`, followed).Scan(&manual); errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("received an invalid follow request for %s by %s", followed, activity.Actor)
		} else if err != nil {
			return fmt.Errorf("failed to fetch %s: %w", followed, err)
		}

		if manual.Valid && manual.Int32 == 1 {
			log.Debug("Not approving follow request", "follower", activity.Actor, "followed", followed)

			if _, err := q.DB.ExecContext(
				ctx,
				`INSERT INTO follows (id, follower, followed) VALUES($1, $2, $3) ON CONFLICT(follower, followed) DO UPDATE SET id = $1, accepted = NULL, inserted = UNIXEPOCH()`,
				activity.ID,
				activity.Actor,
				followed,
			); err != nil {
				return fmt.Errorf("failed to insert follow %s: %w", activity.ID, err)
			}
		} else {
			log.Info("Approving follow request", "follower", activity.Actor, "followed", followed)

			tx, err := q.DB.BeginTx(ctx, nil)
			if err != nil {
				return fmt.Errorf("failed to begin transaction: %w", err)
			}
			defer tx.Rollback()

			if err := outbox.Accept(ctx, followed, activity.Actor, activity.ID, tx); err != nil {
				return fmt.Errorf("failed to accept %s: %w", activity.ID, err)
			}

			if err := tx.Commit(); err != nil {
				return fmt.Errorf("failed to accept follow: %w", err)
			}
		}

	case ap.Accept:
		followID, ok := activity.Object.(string)
		if ok && followID != "" {
			followID = ap.Canonicalize(followID)
			log.Info("Follow is accepted", "follow", followID)
		} else if followActivity, ok := activity.Object.(*ap.Activity); ok && followActivity.Type == ap.Follow && followActivity.ID != "" {
			log.Info("Follow is accepted", "follow", followActivity.ID)
			followID = ap.Canonicalize(followActivity.ID)
		} else {
			return errors.New("received an invalid Accept")
		}

		if res, err := q.DB.ExecContext(ctx, `update follows set accepted = 1 where id = ? and followed = ?`, followID, sender.ID); err != nil {
			return fmt.Errorf("failed to accept follow %s: %w", followID, err)
		} else if n, err := res.RowsAffected(); err != nil {
			return fmt.Errorf("failed to accept follow %s: %w", followID, err)
		} else if n == 0 {
			return fmt.Errorf("received an invalid Accept for %s by %s", activity.Actor, sender.ID)
		}

	case ap.Reject:
		if sender.ID != activity.Actor {
			return fmt.Errorf("received an invalid Reject for %s by %s", activity.Actor, sender.ID)
		}

		followID, ok := activity.Object.(string)
		if ok && followID != "" {
			followID = ap.Canonicalize(followID)
			log.Info("Follow is rejected", "follow", followID)
		} else if followActivity, ok := activity.Object.(*ap.Activity); ok && followActivity.Type == ap.Follow && followActivity.ID != "" {
			log.Info("Follow is rejected", "follow", followActivity.ID)
			followID = ap.Canonicalize(followActivity.ID)
		} else {
			return errors.New("received an invalid Reject")
		}

		if _, err := q.DB.ExecContext(ctx, `update follows set accepted = 0 where id = ? and followed = ?`, followID, activity.Actor); err != nil {
			return fmt.Errorf("failed to reject follow %s: %w", followID, err)
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
			noteID = ap.Canonicalize(noteID)

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

		if sender.ID != activity.Actor {
			return fmt.Errorf("received an invalid undo request for %s by %s", activity.Actor, sender.ID)
		}

		follower := activity.Actor

		var followed string
		if actor, ok := inner.Object.(*ap.Object); ok {
			followed = ap.Canonicalize(actor.ID)
		} else if actorID, ok := inner.Object.(string); ok {
			followed = ap.Canonicalize(actorID)
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

		if res, err := q.DB.ExecContext(ctx, `update follows set accepted = 0 where follower = ? and followed = ? and exists (select 1 from persons where persons.id = follows.followed and persons.ed25519privkey is not null)`, follower, followed); err != nil {
			return fmt.Errorf("failed to remove follow of %s by %s: %w", followed, follower, err)
		} else if n, err := res.RowsAffected(); err != nil {
			return fmt.Errorf("failed to remove follow of %s by %s: %w", followed, follower, err)
		} else if n == 0 {
			return fmt.Errorf("failed to remove follow of %s by %s: not found", followed, follower)
		}

		log.Info("Removed a Follow", "follower", follower, "followed", followed)

	case ap.Create:
		post, ok := activity.Object.(*ap.Object)
		if !ok {
			return errors.New("received invalid Create")
		}

		return q.processCreateActivity(ctx, log, sender, activity, rawActivity, post, shared)

	case ap.Announce:
		inner, ok := activity.Object.(*ap.Activity)
		if !ok {
			if postID, ok := activity.Object.(string); ok && postID != "" {
				if _, err := q.DB.ExecContext(
					ctx,
					`INSERT OR IGNORE INTO shares (note, by, activity) VALUES(?,?,?)`,
					ap.Canonicalize(postID),
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
		return q.processActivity(ctx, log.With("activity", inner, "depth", depth), sender, inner, rawActivity, depth, true)

	case ap.Update:
		post, ok := activity.Object.(*ap.Object)
		if !ok {
			log.Debug("Ignoring unsupported Update object")
			return nil
		}

		post.ID = ap.Canonicalize(post.ID)
		if post.ID == activity.Actor || post.ID == sender.ID {
			log.Debug("Ignoring unsupported Update object")
			return nil
		}

		if post.ID == "" || post.AttributedTo == "" {
			return errors.New("received invalid Update")
		}
		post.AttributedTo = ap.Canonicalize(post.AttributedTo)

		var oldPost ap.Object
		var lastChange int64
		if err := q.DB.QueryRowContext(ctx, `select max(inserted, updated), json(object) from notes where id = ? and author = ?`, post.ID, post.AttributedTo).Scan(&lastChange, &oldPost); err != nil && errors.Is(err, sql.ErrNoRows) {
			log.Debug("Received Update for non-existing post")
			return q.processCreateActivity(ctx, log, sender, activity, rawActivity, post, shared)
		} else if err != nil {
			return fmt.Errorf("failed to get last update time for %s: %w", post.ID, err)
		}

		// if specified, prefer post publication or editing time to insertion or last update time
		var sec int64
		if oldPost.Updated != (ap.Time{}) {
			sec = oldPost.Updated.UnixNano()
		}
		if sec == 0 {
			sec = oldPost.Published.UnixNano()
		}
		if sec > 0 {
			lastChange = sec
		} else {
			lastChange *= 1000000000
		}

		if (post.Type == ap.Question && post.Updated != (ap.Time{}) && lastChange >= post.Updated.UnixNano()) || (post.Type != ap.Question && (post.Updated == (ap.Time{}) || lastChange >= post.Updated.UnixNano())) {
			log.Debug("Received old update request for new post")
			return nil
		}

		// only the group can decide if audience has changed
		oldPost.Audience = ap.Canonicalize(oldPost.Audience)
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
			`update notes set object = jsonb(?), updated = unixepoch() where id = ?`,
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
			`update feed set note = jsonb(?) where note->>'$.id' = ?`,
			post,
			post.ID,
		); err != nil {
			return fmt.Errorf("failed to update post %s: %w", post.ID, err)
		}

		if err := outbox.ForwardActivity(ctx, q.Config, tx, post, activity, rawActivity); err != nil {
			return fmt.Errorf("failed to forward update post %s: %w", post.ID, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to update post %s: %w", post.ID, err)
		}

		log.Info("Updated post")

	case ap.Move:
		log.Debug("Ignoring Move activity")

	case ap.Like, ap.Dislike, ap.EmojiReact, ap.Add, ap.Remove:
		log.Debug("Ignoring activity")

	default:
		if sender.ID == activity.Actor {
			log.Warn("Received unknown request")
		} else {
			log.Warn("Received unknown, unauthorized request")
		}
	}

	return nil
}

func (q *Queue) processActivityWithTimeout(parent context.Context, sender *ap.Actor, activity *ap.Activity, rawActivity string, shared bool) {
	ctx, cancel := context.WithTimeout(parent, q.Config.ActivityProcessingTimeout)
	defer cancel()

	log := slog.With("activity", activity, "sender", sender.ID)
	if err := q.processActivity(ctx, log, sender, activity, rawActivity, 1, shared); err != nil {
		log.Warn("Failed to process activity", "error", err)
	}
}

// ProcessBatch processes one batch of incoming activites in the queue.
func (q *Queue) ProcessBatch(ctx context.Context) (int, error) {
	slog.Debug("Polling activities queue")

	rows, err := q.DB.QueryContext(ctx, `select inbox.id, json(persons.actor), json(inbox.activity), inbox.raw, inbox.raw->>'$.type' = 'Announce' as shared from (select * from inbox limit -1 offset case when (select count(*) from inbox) >= $1 then $1/10 else 0 end) inbox left join persons on persons.id = inbox.sender order by inbox.id limit $2`, q.Config.MaxActivitiesQueueSize, q.Config.ActivitiesBatchSize)
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
		var shared bool
		if err := rows.Scan(&id, &sender, &activity, &activityString, &shared); err != nil {
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
			Shared:      shared,
		})
	}
	rows.Close()

	if len(batch) == 0 {
		return 0, nil
	}

	for _, item := range batch {
		q.processActivityWithTimeout(ctx, item.Sender, item.Activity, item.RawActivity, item.Shared)
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
