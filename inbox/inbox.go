/*
Copyright 2024, 2025 Dima Krasner

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

// Package inbox creates and processes activities.
//
// Incoming activities are received and queued by [fed.Listener].
package inbox

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"net/url"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/data"
	"github.com/dimkr/tootik/fed"
	"github.com/dimkr/tootik/httpsig"
	"github.com/dimkr/tootik/inbox/note"
)

type Inbox struct {
	Domain    string
	Config    *cfg.Config
	BlockList *fed.BlockList
	DB        *sql.DB
}

var ErrActivityTooNested = errors.New("exceeded activity depth limit")

func (inbox *Inbox) processCreateActivity(ctx context.Context, tx *sql.Tx, sender *ap.Actor, activity *ap.Activity, rawActivity string, post *ap.Object, shared bool) error {
	u, err := url.Parse(post.ID)
	if err != nil {
		return fmt.Errorf("failed to parse post ID %s: %w", post.ID, err)
	}

	if !data.IsIDValid(u) {
		return fmt.Errorf("received invalid post ID: %s", post.ID)
	}

	if inbox.BlockList != nil && inbox.BlockList.Contains(u.Host) {
		return fmt.Errorf("ignoring post %s: %w", post.ID, fed.ErrBlockedDomain)
	}

	if len(post.To.OrderedMap)+len(post.CC.OrderedMap) > inbox.Config.MaxRecipients {
		slog.WarnContext(ctx, "Post has too many recipients", "to", len(post.To.OrderedMap), "cc", len(post.CC.OrderedMap))
		return nil
	}

	var audience sql.NullString
	if err := tx.QueryRowContext(ctx, `select object->>'$.audience' from notes where id = ?`, post.ID).Scan(&audience); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("failed to check of %s is a duplicate: %w", post.ID, err)
	} else if err == nil {
		if sender.ID == post.Audience && !audience.Valid {
			if _, err := tx.ExecContext(ctx, `update notes set object = jsonb_set(jsonb_remove(object, '$.proof', '$.signature'), '$.audience', ?) where id = ? and object->>'$.audience' is null`, post.Audience, post.ID); err != nil {
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

		slog.DebugContext(ctx, "Post is a duplicate", "post", post.ID)
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

	if err := inbox.forwardActivity(ctx, tx, post, activity, rawActivity); err != nil {
		return fmt.Errorf("cannot forward %s: %w", post.ID, err)
	}

	slog.InfoContext(ctx, "Received a new post", "post", post.ID)

	return nil
}

func (inbox *Inbox) ProcessActivity(ctx context.Context, tx *sql.Tx, sender *ap.Actor, activity *ap.Activity, rawActivity string, depth int, shared bool) error {
	if depth == ap.MaxActivityDepth {
		return ErrActivityTooNested
	}

	slog.DebugContext(ctx, "Processing activity")

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

		slog.InfoContext(ctx, "Received delete request", "deleted", deleted)

		if deleted == activity.Actor {
			if _, err := tx.ExecContext(ctx, `delete from persons where id = ?`, deleted); err != nil {
				return fmt.Errorf("failed to delete person %s: %w", deleted, err)
			}
		} else {
			var note ap.Object
			if err := tx.QueryRowContext(ctx, `select json(object) from notes where id = ?`, deleted).Scan(&note); err != nil && errors.Is(err, sql.ErrNoRows) {
				slog.DebugContext(ctx, "Received delete request for non-existing post", "deleted", deleted)
				return nil
			} else if err != nil {
				return fmt.Errorf("failed to delete %s: %w", deleted, err)
			}

			if err := inbox.forwardActivity(ctx, tx, &note, activity, rawActivity); err != nil {
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
		followedID, ok := activity.Object.(string)
		if !ok {
			return errors.New("received a request to follow a non-link object")
		}
		if followedID == "" {
			return errors.New("received an invalid follow request")
		}

		var ed25519PrivKeyMultibase sql.NullString
		var followed ap.Actor
		if err := tx.QueryRowContext(ctx, `select ed25519privkey, json(actor) from persons where cid = ? order by ed25519privkey is not null desc limit 1`, ap.Canonical(followedID)).Scan(&ed25519PrivKeyMultibase, &followed); errors.Is(err, sql.ErrNoRows) {
			var localFollowerID string
			if err := tx.QueryRowContext(ctx, `select id from persons where cid = ? and ed25519privkey is not null`, ap.Canonical(activity.Actor)).Scan(&localFollowerID); errors.Is(err, sql.ErrNoRows) {
				return fmt.Errorf("received an invalid follow request for %s by %s", followedID, activity.Actor)
			} else if err != nil {
				return fmt.Errorf("failed to validate follow request for %s by %s: %w", followedID, activity.Actor, err)
			} else if _, err := tx.ExecContext(
				ctx,
				`INSERT INTO follows (id, follower, followed) VALUES($1, $2, $3) ON CONFLICT(follower, followed) DO UPDATE SET id = $1, accepted = NULL, inserted = UNIXEPOCH()`,
				activity.ID,
				localFollowerID,
				followedID,
			); err != nil {
				return fmt.Errorf("failed to insert follow %s: %w", activity.ID, err)
			}

			return nil
		} else if err != nil {
			return fmt.Errorf("failed to fetch %s: %w", followed.ID, err)
		}

		if !ed25519PrivKeyMultibase.Valid || followed.ManuallyApprovesFollowers {
			slog.InfoContext(ctx, "Not approving follow request", "follower", activity.Actor, "followed", followed.ID)

			if _, err := tx.ExecContext(
				ctx,
				`INSERT INTO follows (id, follower, followed) VALUES($1, $2, $3) ON CONFLICT(follower, followed) DO UPDATE SET id = $1, accepted = NULL, inserted = UNIXEPOCH()`,
				activity.ID,
				activity.Actor,
				followed.ID,
			); err != nil {
				return fmt.Errorf("failed to insert follow %s: %w", activity.ID, err)
			}
		} else if ed25519PrivKeyMultibase.Valid && !followed.ManuallyApprovesFollowers {
			slog.InfoContext(ctx, "Approving follow request", "follower", activity.Actor, "followed", followed.ID)

			if _, err := tx.ExecContext(
				ctx,
				`INSERT INTO follows (id, follower, followed, accepted) VALUES($1, $2, $3, 1) ON CONFLICT(follower, followed) DO UPDATE SET id = $1, accepted = 1, inserted = UNIXEPOCH()`,
				activity.ID,
				activity.Actor,
				followed.ID,
			); err != nil {
				return fmt.Errorf("failed to insert follow %s: %w", activity.ID, err)
			}

			ed25519PrivKey, err := data.DecodeEd25519PrivateKey(ed25519PrivKeyMultibase.String)
			if err != nil {
				return fmt.Errorf("failed to accept %s: %w", activity.ID, err)
			}

			if err := inbox.Accept(ctx, &followed, httpsig.Key{ID: followed.AssertionMethod[0].ID, PrivateKey: ed25519PrivKey}, activity.Actor, activity.ID, tx); err != nil {
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
		} else if followActivity, ok := activity.Object.(*ap.Activity); ok && followActivity.Type == ap.Follow && followActivity.ID != "" {
			followID = followActivity.ID
		} else {
			return errors.New("received an invalid Accept")
		}

		slog.InfoContext(ctx, "Follow is accepted", "follow", followID)

		if _, err := tx.ExecContext(
			ctx,
			`
			UPDATE follows SET accepted = 1 WHERE id = ? AND followed = ?
			`,
			followID,
			sender.ID,
		); err != nil {
			return fmt.Errorf("failed to insert follow: %w", err)
		}

	case ap.Reject:
		if sender.ID != activity.Actor {
			return fmt.Errorf("received an invalid Reject for %s by %s", activity.Actor, sender.ID)
		}

		followID, ok := activity.Object.(string)
		if !ok || followID == "" {
			if followActivity, ok := activity.Object.(*ap.Activity); ok && followActivity.Type == ap.Follow && followActivity.ID != "" {
				followID = followActivity.ID
			} else {
				return errors.New("received an invalid Reject")
			}
		}

		slog.InfoContext(ctx, "Follow is rejected", "follow", followID)

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
			slog.DebugContext(ctx, "Ignoring request to undo a non-Follow activity")
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

		slog.InfoContext(ctx, "Removed a Follow", "follower", follower, "followed", followed)

	case ap.Create:
		post, ok := activity.Object.(*ap.Object)
		if !ok {
			return errors.New("received invalid Create")
		}

		return inbox.processCreateActivity(ctx, tx, sender, activity, rawActivity, post, shared)

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
				slog.DebugContext(ctx, "Ignoring unsupported Announce object")
			}
			return nil
		}

		depth++
		return inbox.ProcessActivity(ctx, tx, sender, inner, rawActivity, depth, true)

	case ap.Update:
		post, ok := activity.Object.(*ap.Object)
		if !ok || ap.Canonical(post.ID) == ap.Canonical(activity.Actor) || ap.Canonical(post.ID) == ap.Canonical(sender.ID) {
			slog.DebugContext(ctx, "Ignoring unsupported Update object")
			return nil
		}

		if post.ID == "" || post.AttributedTo == "" {
			return errors.New("received invalid Update")
		}

		var oldPost ap.Object
		var lastChange int64
		if err := tx.QueryRowContext(ctx, `select max(inserted, updated), json(object) from notes where id = ? and author in (select id from persons where cid = ?)`, post.ID, ap.Canonical(post.AttributedTo)).Scan(&lastChange, &oldPost); err != nil && errors.Is(err, sql.ErrNoRows) {
			slog.DebugContext(ctx, "Received Update for non-existing post")
			return inbox.processCreateActivity(ctx, tx, sender, activity, rawActivity, post, shared)
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
			slog.DebugContext(ctx, "Received old update request for new post")
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

		if err := inbox.forwardActivity(ctx, tx, post, activity, rawActivity); err != nil {
			return fmt.Errorf("failed to forward update post %s: %w", post.ID, err)
		}

		slog.InfoContext(ctx, "Updated post", "post", post.ID)

	case ap.Move:
		slog.DebugContext(ctx, "Ignoring Move activity")

	case ap.Like, ap.Dislike, ap.EmojiReact, ap.Add, ap.Remove:
		slog.DebugContext(ctx, "Ignoring activity")

	default:
		if sender.ID == activity.Actor {
			slog.WarnContext(ctx, "Received unknown request")
		} else {
			slog.WarnContext(ctx, "Received unknown, unauthorized request")
		}
	}

	return nil
}
