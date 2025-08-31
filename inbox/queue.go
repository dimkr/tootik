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
	"net/url"
	"time"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/data"
	"github.com/dimkr/tootik/fed"
	"github.com/dimkr/tootik/httpsig"
	"github.com/dimkr/tootik/inbox/note"
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

func (q *Queue) processCreateActivity(ctx context.Context, tx *sql.Tx, sender *ap.Actor, activity *ap.Activity, rawActivity string, post *ap.Object, shared bool) error {
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
		slog.Warn("Post has too many recipients", "activity", activity, "to", len(post.To.OrderedMap), "cc", len(post.CC.OrderedMap))
		return nil
	}

	var audience sql.NullString
	if err := tx.QueryRowContext(ctx, `select object->>'$.audience' from notes where id = ?`, post.ID).Scan(&audience); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("failed to check of %s is a duplicate: %w", post.ID, err)
	} else if err == nil {
		if sender.ID == post.Audience && !audience.Valid {
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
		} else if shared {
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

		slog.Debug("Post is a duplicate", "activity", activity, "post", post.ID)
		return nil
	}

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

	if err := q.forwardActivity(ctx, tx, post, activity, rawActivity); err != nil {
		return fmt.Errorf("cannot forward %s: %w", post.ID, err)
	}

	slog.Info("Received a new post", "activity", activity, "post", post.ID)

	return nil
}

func (q *Queue) ProcessLocalActivity(ctx context.Context, tx *sql.Tx, sender *ap.Actor, activity *ap.Activity, rawActivity string) error {
	return q.processActivity(ctx, tx, sender, activity, rawActivity, 1, false)
}

func (q *Queue) processActivity(ctx context.Context, tx *sql.Tx, sender *ap.Actor, activity *ap.Activity, rawActivity string, depth int, shared bool) error {
	if depth == ap.MaxActivityDepth {
		return ErrActivityTooNested
	}

	slog.Debug("Processing activity", "activity", activity)

	switch activity.Type {
	case ap.Delete:
		deleted := ""
		if o, ok := activity.Object.(*ap.Object); ok {
			deleted = o.ID
		} else if s, ok := activity.Object.(string); ok {
			deleted = s
		}
		if deleted == "" {
			return errors.New("received an invalid delete activity")
		}

		slog.Info("Received delete request", "activity", activity, "deleted", deleted)

		if deleted == activity.Actor {
			if _, err := tx.ExecContext(ctx, `delete from persons where id = ?`, deleted); err != nil {
				return fmt.Errorf("failed to delete person %s: %w", deleted, err)
			}
		} else {
			var note ap.Object
			if err := tx.QueryRowContext(ctx, `select json(object) from notes where id = ?`, deleted).Scan(&note); err != nil && errors.Is(err, sql.ErrNoRows) {
				slog.Debug("Received delete request for non-existing post", "activity", activity, "deleted", deleted)
				return nil
			} else if err != nil {
				return fmt.Errorf("failed to delete %s: %w", deleted, err)
			}

			if err := q.forwardActivity(ctx, tx, &note, activity, rawActivity); err != nil {
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
		}

	case ap.Follow:
		if sender.ID != activity.Actor {
			return errors.New("received unauthorized follow request")
		}

		followedID, ok := activity.Object.(string)
		if !ok {
			return errors.New("received a request to follow a non-link object")
		}
		if followedID == "" {
			return errors.New("received an invalid follow request")
		}

		var localFollowed int
		var followed ap.Actor
		if err := tx.QueryRowContext(ctx, `select ed25519privkey is not null, json(actor) from persons where id = ? order by ed25519privkey is not null desc limit 1`, followedID).Scan(&localFollowed, &followed); errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("received an invalid follow request for %s by %s", followedID, activity.Actor)
		} else if err != nil {
			return fmt.Errorf("failed to fetch %s: %w", followed.ID, err)
		}

		if localFollowed == 0 || followed.ManuallyApprovesFollowers {
			slog.Info("Not approving follow request", "activity", activity, "follower", activity.Actor, "followed", followed.ID)

			if _, err := tx.ExecContext(
				ctx,
				`INSERT INTO follows (id, follower, followed) VALUES($1, $2, $3) ON CONFLICT(follower, followed) DO UPDATE SET id = $1, accepted = NULL, inserted = UNIXEPOCH()`,
				activity.ID,
				activity.Actor,
				followed.ID,
			); err != nil {
				return fmt.Errorf("failed to insert follow %s: %w", activity.ID, err)
			}
		} else if localFollowed == 1 && !followed.ManuallyApprovesFollowers {
			slog.Info("Approving follow request", "activity", activity, "follower", activity.Actor, "followed", followed.ID)

			if _, err := tx.ExecContext(
				ctx,
				`INSERT INTO follows (id, follower, followed, accepted) VALUES($1, $2, $3, 1) ON CONFLICT(follower, followed) DO UPDATE SET id = $1, accepted = 1, inserted = UNIXEPOCH()`,
				activity.ID,
				activity.Actor,
				followed.ID,
			); err != nil {
				return fmt.Errorf("failed to insert follow %s: %w", activity.ID, err)
			}

			if err := q.Accept(ctx, &followed, activity.Actor, activity.ID, tx); err != nil {
				return fmt.Errorf("failed to accept %s: %w", activity.ID, err)
			}
		} else {
			return fmt.Errorf("received an invalid follow request for %s by %s", followed.ID, activity.Actor)
		}

	case ap.Accept:
		if sender.ID != activity.Actor {
			return fmt.Errorf("received an invalid Accept for %s by %s", activity.Actor, sender.ID)
		}

		followID, ok := activity.Object.(string)
		if ok && followID != "" {
			slog.Info("Follow is accepted", "activity", activity, "follow", followID)
		} else if followActivity, ok := activity.Object.(*ap.Activity); ok && followActivity.Type == ap.Follow && followActivity.ID != "" {
			slog.Info("Follow is accepted", "activity", activity, "follow", followActivity.ID)
			followID = followActivity.ID
		} else {
			return errors.New("received an invalid Accept")
		}

		if _, err := tx.ExecContext(
			ctx,
			`
			INSERT INTO follows (id, follower, followed, accepted)
			SELECT $1, others.follower, $2, 1
			FROM follows others
			WHERE others.id = $1 AND others.followedcid = $3
			ON CONFLICT(follower, followed) DO UPDATE SET id = $1, accepted = 1, inserted = UNIXEPOCH()
			`,
			followID,
			sender.ID,
			ap.Canonical(sender.ID),
		); err != nil {
			return fmt.Errorf("failed to insert follow: %w", err)
		}

	case ap.Reject:
		if sender.ID != activity.Actor {
			return fmt.Errorf("received an invalid Reject for %s by %s", activity.Actor, sender.ID)
		}

		followID, ok := activity.Object.(string)
		if ok && followID != "" {
			slog.Info("Follow is rejected", "activity", activity, "follow", followID)
		} else if followActivity, ok := activity.Object.(*ap.Activity); ok && followActivity.Type == ap.Follow && followActivity.ID != "" {
			slog.Info("Follow is rejected", "activity", activity, "follow", followActivity.ID)
			followID = followActivity.ID
		} else {
			return errors.New("received an invalid Reject")
		}

		if res, err := tx.ExecContext(ctx, `update follows set accepted = 0 where id = ? and followed = ? and (accepted is null or accepted = 1)`, followID, sender.ID); err != nil {
			return fmt.Errorf("failed to reject follow %s: %w", followID, err)
		} else if n, err := res.RowsAffected(); err != nil {
			return fmt.Errorf("failed to reject follow %s: %w", followID, err)
		} else if n == 0 {
			return fmt.Errorf("failed to reject follow %s: not found", followID)
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
			if _, err := tx.ExecContext(
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
			slog.Debug("Ignoring request to undo a non-Follow activity", "activity", activity)
			return nil
		}

		if sender.ID != activity.Actor {
			return fmt.Errorf("received an invalid undo request for %s by %s", activity.Actor, sender.ID)
		}

		follower := activity.Actor

		var followed string
		if follow, ok := inner.Object.(*ap.Activity); ok {
			followed = follow.Actor
		} else if actor, ok := inner.Object.(*ap.Object); ok {
			followed = actor.ID
		} else if actorID, ok := inner.Object.(string); ok {
			followed = actorID
		} else {
			return errors.New("received a request to undo follow on unknown object")
		}
		if followed == "" {
			return errors.New("received an undo request with empty ID")
		}

		if _, err := tx.ExecContext(ctx, `delete from follows where follower = ? and followed = ?`, follower, followed); err != nil {
			return fmt.Errorf("failed to remove follow of %s by %s: %w", followed, follower, err)
		}

		slog.Info("Removed a Follow", "activity", activity, "follower", follower, "followed", followed)

	case ap.Create:
		post, ok := activity.Object.(*ap.Object)
		if !ok {
			return errors.New("received invalid Create")
		}

		return q.processCreateActivity(ctx, tx, sender, activity, rawActivity, post, shared)

	case ap.Announce:
		inner, ok := activity.Object.(*ap.Activity)
		if !ok {
			if postID, ok := activity.Object.(string); ok && postID != "" {
				if _, err := tx.ExecContext(
					ctx,
					`INSERT OR IGNORE INTO shares (note, by, activity) VALUES(?,?,?)`,
					postID,
					sender.ID,
					activity.ID,
				); err != nil {
					return fmt.Errorf("cannot insert share for %s by %s: %w", postID, sender.ID, err)
				}
			} else {
				slog.Debug("Ignoring unsupported Announce object", "activity", activity)
			}
			return nil
		}

		depth++
		return q.processActivity(ctx, tx, sender, inner, rawActivity, depth, true)

	case ap.Update:
		post, ok := activity.Object.(*ap.Object)
		if !ok || ap.Canonical(post.ID) == ap.Canonical(activity.Actor) || ap.Canonical(post.ID) == ap.Canonical(sender.ID) {
			slog.Debug("Ignoring unsupported Update object", "activity", activity)
			return nil
		}

		if post.ID == "" || post.AttributedTo == "" {
			return errors.New("received invalid Update")
		}

		var oldPost ap.Object
		var lastChange int64
		if err := tx.QueryRowContext(ctx, `select max(inserted, updated), json(object) from notes where id = ? and author in (select id from persons where cid = ?)`, post.ID, ap.Canonical(post.AttributedTo)).Scan(&lastChange, &oldPost); err != nil && errors.Is(err, sql.ErrNoRows) {
			slog.Debug("Received Update for non-existing post", "activity", activity)
			return q.processCreateActivity(ctx, tx, sender, activity, rawActivity, post, shared)
		} else if err != nil {
			return fmt.Errorf("failed to get last update time for %s: %w", post.ID, err)
		}

		// if specified, prefer post publication or editing time to insertion or last update time
		if oldPost.Updated != (ap.Time{}) {
			lastChange = oldPost.Updated.UnixNano()
		} else if oldPost.Published != (ap.Time{}) {
			lastChange = oldPost.Published.UnixNano()
		} else {
			lastChange *= 1000000000
		}

		if (post.Type == ap.Question && post.Updated != (ap.Time{}) && lastChange >= post.Updated.UnixNano()) || (post.Type != ap.Question && (post.Updated == (ap.Time{}) || lastChange >= post.Updated.UnixNano())) {
			slog.Debug("Received old update request for new post", "activity", activity)
			return nil
		}

		// only the group can decide if audience has changed
		if sender.ID != oldPost.Audience {
			post.Audience = oldPost.Audience
		}

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

		if err := q.forwardActivity(ctx, tx, post, activity, rawActivity); err != nil {
			return fmt.Errorf("failed to forward update post %s: %w", post.ID, err)
		}

		slog.Info("Updated post", "activity", activity, "post", post.ID)

	case ap.Move:
		slog.Debug("Ignoring Move activity", "activity", activity)

	case ap.Like, ap.Dislike, ap.EmojiReact, ap.Add, ap.Remove:
		slog.Debug("Ignoring activity", "activity", activity)

	default:
		if sender.ID == activity.Actor {
			slog.Warn("Received unknown request", "activity", activity)
		} else {
			slog.Warn("Received unknown, unauthorized request", "activity", activity)
		}
	}

	return nil
}

func (q *Queue) processActivityWithTimeout(parent context.Context, tx *sql.Tx, sender *ap.Actor, activity *ap.Activity, rawActivity string, shared bool) {
	ctx, cancel := context.WithTimeout(parent, q.Config.ActivityProcessingTimeout)
	defer cancel()

	if err := q.processActivity(ctx, tx, sender, activity, rawActivity, 1, shared); err != nil {
		slog.Warn("Failed to process activity", "activity", activity, "error", err)
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

	tx, err := q.DB.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("failed to process batch: %w", err)
	}
	defer tx.Rollback()

	for _, item := range batch {
		q.processActivityWithTimeout(ctx, tx, item.Sender, item.Activity, item.RawActivity, item.Shared)
	}

	if _, err := tx.ExecContext(ctx, `delete from inbox where id <= ?`, maxID); err != nil {
		return 0, fmt.Errorf("failed to delete processed activities: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("failed to process batch: %w", err)
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
