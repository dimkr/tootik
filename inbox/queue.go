/*
Copyright 2023, 2024 Dima Krasner

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
	"encoding/json"
	"errors"
	"fmt"
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/data"
	"github.com/dimkr/tootik/fed"
	"github.com/dimkr/tootik/inbox/note"
	"github.com/dimkr/tootik/outbox"
	"log/slog"
	"net/url"
	"strings"
	"time"
)

type Queue struct {
	Domain   string
	Config   *cfg.Config
	Log      *slog.Logger
	DB       *sql.DB
	Resolver *fed.Resolver
	Actor    *ap.Actor
}

// a reply by B in a thread started by A is forwarded to all followers of A
func (q *Queue) forwardActivity(ctx context.Context, log *slog.Logger, tx *sql.Tx, activity *ap.Activity, rawActivity []byte) error {
	obj, ok := activity.Object.(*ap.Object)
	if !ok {
		return nil
	}

	// poll votes don't need to be forwarded
	if obj.Name != "" && obj.Content == "" {
		return nil
	}

	var firstPostID, threadStarterID string
	var depth int
	if err := tx.QueryRowContext(ctx, `with recursive thread(id, author, parent, depth) as (select notes.id, notes.author, notes.object->>'inReplyTo' as parent, 1 as depth from notes where id = $1 union select notes.id, notes.author, notes.object->>'inReplyTo' as parent, t.depth + 1 from thread t join notes on notes.id = t.parent where t.depth <= $2) select id, author, depth from thread order by depth desc limit 1`, obj.ID, q.Config.MaxForwardingDepth+1).Scan(&firstPostID, &threadStarterID, &depth); err != nil && errors.Is(err, sql.ErrNoRows) {
		log.Debug("Failed to find thread for post", "post", obj.ID)
		return nil
	} else if err != nil {
		return fmt.Errorf("failed to fetch first post in thread: %w", err)
	}
	if depth > q.Config.MaxForwardingDepth {
		log.Debug("Thread exceeds depth limit for forwarding")
		return nil
	}

	prefix := fmt.Sprintf("https://%s/", q.Domain)
	if !strings.HasPrefix(threadStarterID, prefix) {
		log.Debug("Thread starter is federated")
		return nil
	}

	var shouldForward int
	if err := tx.QueryRowContext(ctx, `select exists (select 1 from notes join persons on persons.id = notes.author and (notes.public = 1 or exists (select 1 from json_each(notes.object->'to') where value = persons.actor->>'followers') or exists (select 1 from json_each(notes.object->'cc') where value = persons.actor->>'followers')) where notes.id = ?)`, firstPostID).Scan(&shouldForward); err != nil {
		return err
	}
	if shouldForward == 0 {
		log.Debug("Activity does not need to be forwarded")
		return nil
	}

	if _, err := tx.ExecContext(
		ctx,
		`INSERT INTO outbox (activity, sender) VALUES(?,?)`,
		string(rawActivity),
		threadStarterID,
	); err != nil {
		return err
	}

	log.Info("Forwarding activity to followers of thread starter", "thread", firstPostID, "starter", threadStarterID)
	return nil
}

func (q *Queue) processCreateActivity(ctx context.Context, log *slog.Logger, sender *ap.Actor, req *ap.Activity, rawActivity []byte, post *ap.Object) error {
	prefix := fmt.Sprintf("https://%s/", q.Domain)
	if strings.HasPrefix(sender.ID, prefix) || strings.HasPrefix(post.ID, prefix) || strings.HasPrefix(post.AttributedTo, prefix) || strings.HasPrefix(req.Actor, prefix) {
		return fmt.Errorf("received invalid Create for %s by %s from %s", post.ID, post.AttributedTo, req.Actor)
	}

	u, err := url.Parse(post.ID)
	if err != nil {
		return fmt.Errorf("failed to parse post ID %s: %w", post.ID, err)
	}

	if !data.IsIDValid(u) {
		return fmt.Errorf("received invalid post ID: %s", post.ID)
	}

	var duplicate int
	if err := q.DB.QueryRowContext(ctx, `select exists (select 1 from notes where id = ?)`, post.ID).Scan(&duplicate); err != nil {
		return fmt.Errorf("failed to check of %s is a duplicate: %w", post.ID, err)
	} else if duplicate == 1 {
		log.Debug("Post is a duplicate")
		return nil
	}

	if _, err := q.Resolver.ResolveID(ctx, log, q.DB, q.Actor, post.AttributedTo, false); err != nil {
		return fmt.Errorf("failed to resolve %s: %w", post.AttributedTo, err)
	}

	tx, err := q.DB.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("cannot insert %s: %w", post.ID, err)
	}
	defer tx.Rollback()

	if err := note.Insert(ctx, log, tx, post); err != nil {
		return fmt.Errorf("cannot insert %s: %w", post.ID, err)
	}

	if err := q.forwardActivity(ctx, log, tx, req, rawActivity); err != nil {
		return fmt.Errorf("cannot forward %s: %w", post.ID, err)
	}

	log.Info("Received a new post")

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("cannot insert %s: %w", post.ID, err)
	}

	mentionedUsers := ap.Audience{}

	for _, tag := range post.Tag {
		if tag.Type == ap.MentionMention && tag.Href != post.AttributedTo {
			mentionedUsers.Add(tag.Href)
		}
	}

	mentionedUsers.Range(func(id string, _ struct{}) bool {
		if _, err := q.Resolver.ResolveID(ctx, log, q.DB, q.Actor, id, false); err != nil {
			log.Warn("Failed to resolve mention", "mention", id, "error", err)
		}

		return true
	})

	return nil
}

func (q *Queue) processActivity(ctx context.Context, log *slog.Logger, sender *ap.Actor, req *ap.Activity, rawActivity []byte) error {
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
			return errors.New("received an invalid delete request")
		}

		log.Info("Received delete request", "deleted", deleted)

		if deleted == sender.ID {
			if _, err := q.DB.ExecContext(ctx, `delete from persons where id = ?`, deleted); err != nil {
				return fmt.Errorf("failed to delete person %s: %w", req.ID, err)
			}
		} else {
			if _, err := q.DB.ExecContext(ctx, `delete from notesfts where id = $1 and exists (select 1 from notes where id = $1 and author = $2)`, deleted, sender.ID); err != nil {
				return fmt.Errorf("failed to delete posts by %s", req.ID)
			}
			if _, err := q.DB.ExecContext(ctx, `delete from notes where id = ? and author = ?`, deleted, sender.ID); err != nil {
				return fmt.Errorf("failed to delete posts by %s", req.ID)
			}
			if _, err := q.DB.ExecContext(ctx, `delete from shares where note = $1 and exists (select 1 from notes where id = $1 and author = $2)`, deleted, sender.ID); err != nil {
				return fmt.Errorf("failed to delete posts by %s", req.ID)
			}
		}

	case ap.FollowActivity:
		if sender.ID != req.Actor {
			return errors.New("received unauthorized follow request")
		}

		followed, ok := req.Object.(string)
		if !ok {
			return errors.New("received a request to follow a non-link object")
		}
		if followed == "" {
			return errors.New("received an invalid follow request")
		}

		prefix := fmt.Sprintf("https://%s/", q.Domain)
		if strings.HasPrefix(req.Actor, prefix) || !strings.HasPrefix(followed, prefix) {
			return fmt.Errorf("received an invalid follow request for %s by %s", followed, req.Actor)
		}

		var from ap.Actor
		if err := q.DB.QueryRowContext(ctx, `select actor from persons where id = ?`, followed).Scan(&from); err != nil {
			return fmt.Errorf("failed to fetch %s: %w", followed, err)
		}

		log.Info("Approving follow request", "follower", req.Actor, "followed", followed)

		if err := outbox.Accept(ctx, q.Domain, followed, req.Actor, req.ID, q.DB); err != nil {
			return fmt.Errorf("failed to marshal accept response: %w", err)
		}

	case ap.AcceptActivity:
		if sender.ID != req.Actor {
			return fmt.Errorf("received an invalid follow request for %s by %s", req.Actor, sender.ID)
		}

		followID, ok := req.Object.(string)
		if ok && followID != "" {
			log.Info("Follow is accepted", "follow", followID)
		} else if followActivity, ok := req.Object.(*ap.Activity); ok && followActivity.Type == ap.FollowActivity && followActivity.ID != "" {
			log.Info("Follow is accepted", "follow", followActivity.ID)
			followID = followActivity.ID
		} else {
			return errors.New("received an invalid accept notification")
		}

		if _, err := q.DB.ExecContext(ctx, `update follows set accepted = 1 where id = ? and followed = ?`, followID, sender.ID); err != nil {
			return fmt.Errorf("failed to accept follow %s: %w", followID, err)
		}

	case ap.UndoActivity:
		if sender.ID != req.Actor {
			return fmt.Errorf("received an invalid undo request for %s by %s", req.Actor, sender.ID)
		}

		inner, ok := req.Object.(*ap.Activity)
		if !ok {
			return errors.New("received a request to undo a non-activity object")
		}

		if inner.Type == ap.AnnounceActivity {
			noteID, ok := inner.Object.(string)
			if !ok {
				return errors.New("cannot undo Announce")
			}
			if _, err := q.DB.ExecContext(
				ctx,
				`delete from shares where note = ? and by = ?`,
				noteID,
				req.Actor,
			); err != nil {
				return fmt.Errorf("failed to remove share for %s by %s: %w", noteID, req.Actor, err)
			}
			return nil
		}

		if inner.Type != ap.FollowActivity {
			log.Debug("Ignoring request to undo a non-Follow activity")
			return nil
		}

		follower := req.Actor

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

	case ap.CreateActivity:
		post, ok := req.Object.(*ap.Object)
		if !ok {
			return errors.New("received invalid Create")
		}

		return q.processCreateActivity(ctx, log, sender, req, rawActivity, post)

	case ap.AnnounceActivity:
		create, ok := req.Object.(*ap.Activity)
		if !ok {
			if postID, ok := req.Object.(string); ok && postID != "" {
				if _, err := q.DB.ExecContext(
					ctx,
					`INSERT OR IGNORE INTO shares (note, by) VALUES(?,?)`,
					postID,
					sender.ID,
				); err != nil {
					return fmt.Errorf("cannot insert share for %s by %s: %w", postID, sender.ID, err)
				}
			} else {
				log.Debug("Ignoring unsupported Announce object")
			}
			return nil
		}
		if create.Type != ap.CreateActivity {
			log.Debug("Ignoring unsupported Announce type", "type", create.Type)
			return nil
		}

		post, ok := create.Object.(*ap.Object)
		if !ok {
			return errors.New("received invalid Create")
		}

		if err := q.processCreateActivity(ctx, log, sender, create, rawActivity, post); err != nil {
			return err
		}

		if _, err := q.DB.ExecContext(
			ctx,
			`INSERT OR IGNORE INTO shares (note, by) VALUES(?,?)`,
			post.ID,
			sender.ID,
		); err != nil {
			return fmt.Errorf("cannot insert share for %s by %s: %w", post.ID, sender.ID, err)
		}

	case ap.UpdateActivity:
		post, ok := req.Object.(*ap.Object)
		if !ok || post.ID == sender.ID {
			log.Debug("Ignoring unsupported Update object")
			return nil
		}

		if post.ID == "" || post.AttributedTo == "" {
			return errors.New("received invalid Update")
		}

		prefix := fmt.Sprintf("https://%s/", q.Domain)
		if strings.HasPrefix(post.ID, prefix) {
			return fmt.Errorf("%s cannot update posts by %s", sender.ID, post.AttributedTo)
		}

		var oldPost ap.Object
		var lastUpdate int64
		if err := q.DB.QueryRowContext(ctx, `select max(inserted, updated), object from notes where id = ? and author = ?`, post.ID, post.AttributedTo).Scan(&lastUpdate, &oldPost); err != nil && errors.Is(err, sql.ErrNoRows) {
			log.Debug("Received Update for non-existing post")
			return q.processCreateActivity(ctx, log, sender, req, rawActivity, post)
		} else if err != nil {
			return fmt.Errorf("failed to get last update time for %s: %w", post.ID, err)
		}

		body := post
		var err error
		if (post.Type == ap.QuestionObject && post.Updated != nil && lastUpdate >= post.Updated.Unix()) || (post.Type != ap.QuestionObject && (post.Updated == nil || lastUpdate >= post.Updated.Unix())) {
			log.Debug("Received old update request for new post")
			return nil
		} else if post.Type == ap.QuestionObject && oldPost.Closed != nil {
			log.Debug("Received update request for closed poll")
			return nil
		} else if post.Type == ap.QuestionObject && post.Updated == nil {
			oldPost.VotersCount = post.VotersCount
			oldPost.OneOf = post.OneOf
			oldPost.AnyOf = post.AnyOf
			oldPost.EndTime = post.EndTime
			oldPost.Closed = post.Closed

			body = &oldPost
		}

		if err != nil {
			return fmt.Errorf("failed to marshal updated post %s: %w", post.ID, err)
		}

		tx, err := q.DB.BeginTx(ctx, nil)
		if err != nil {
			return fmt.Errorf("cannot insert %s: %w", post.ID, err)
		}
		defer tx.Rollback()

		if _, err := tx.ExecContext(
			ctx,
			`update notes set object = ?, updated = unixepoch() where id = ?`,
			body,
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

		if err := q.forwardActivity(ctx, log, tx, req, rawActivity); err != nil {
			return fmt.Errorf("failed to forward update pos %s: %w", post.ID, err)
		}

		if err := tx.Commit(); err != nil {
			return fmt.Errorf("failed to update post %s: %w", post.ID, err)
		}

		log.Info("Updated post")

	case ap.MoveActivity:
		log.Debug("Ignoring Move activity")

	case ap.LikeActivity:
		log.Debug("Ignoring Like activity")

	default:
		if sender.ID == req.Actor {
			log.Warn("Received unknown request")
		} else {
			log.Warn("Received unknown, unauthorized request")
		}
	}

	return nil
}

func (q *Queue) processActivityWithTimeout(parent context.Context, sender *ap.Actor, activity *ap.Activity, rawActivity []byte) {
	ctx, cancel := context.WithTimeout(parent, q.Config.ActivityProcessingTimeout)
	defer cancel()

	log := q.Log
	if o, ok := activity.Object.(*ap.Object); ok {
		log = q.Log.With(slog.Group("activity", "id", activity.ID, "sender", sender.ID, "type", activity.Type, "actor", activity.Actor, slog.Group("object", "kind", "object", "id", o.ID, "type", o.Type, "attributed_to", o.AttributedTo)))
	} else if a, ok := activity.Object.(*ap.Activity); ok {
		log = q.Log.With(slog.Group("activity", "id", activity.ID, "sender", sender.ID, "type", activity.Type, "actor", activity.Actor, slog.Group("object", "kind", "activity", "id", a.ID, "type", a.Type, "actor", a.Actor)))
	} else if s, ok := activity.Object.(string); ok {
		log = q.Log.With(slog.Group("activity", "id", activity.ID, "sender", sender.ID, "type", activity.Type, "actor", activity.Actor, slog.Group("object", "kind", "string", "id", s)))
	}

	if err := q.processActivity(ctx, log, sender, activity, rawActivity); err != nil {
		log.Warn("Failed to process activity", "error", err)
	}
}

// ProcessBatch processes one batch of incoming activites in the queue.
func (q *Queue) ProcessBatch(ctx context.Context) (int, error) {
	q.Log.Debug("Polling activities queue")

	rows, err := q.DB.QueryContext(ctx, `select inbox.id, persons.actor, inbox.activity from (select * from inbox limit -1 offset case when (select count(*) from inbox) >= $1 then $1/10 else 0 end) inbox left join persons on persons.id = inbox.sender order by inbox.id limit $2`, q.Config.MaxActivitiesQueueSize, q.Config.ActivitiesBatchSize)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch activities to process: %w", err)
	}
	defer rows.Close()

	activities := data.OrderedMap[string, *ap.Actor]{}
	var maxID int64
	var rowsCount int

	for rows.Next() {
		rowsCount += 1

		var id int64
		var activityString string
		var sender sql.Null[ap.Actor]
		if err := rows.Scan(&id, &sender, &activityString); err != nil {
			q.Log.Error("Failed to scan activity", "error", err)
			continue
		}

		maxID = id

		if !sender.Valid {
			q.Log.Warn("Sender is unknown", "id", id)
			continue
		}

		activities.Store(activityString, &sender.V)
	}
	rows.Close()

	if len(activities) == 0 {
		return 0, nil
	}

	activities.Range(func(activityString string, sender *ap.Actor) bool {
		var activity ap.Activity
		if err := json.Unmarshal([]byte(activityString), &activity); err != nil {
			q.Log.Error("Failed to unmarshal activity", "raw", activityString, "error", err)
			return true
		}

		q.processActivityWithTimeout(ctx, sender, &activity, []byte(activityString))
		return true
	})

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
