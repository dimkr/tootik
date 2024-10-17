/*
Copyright 2024 Dima Krasner

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

package outbox

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/cfg"
	"time"
)

func undoFollows(ctx context.Context, domain, actorID string, db *sql.DB) error {
	follows, err := db.QueryContext(ctx, `select id, followed from follows where follower = ? order by inserted`, actorID)
	if err != nil {
		return err
	}
	defer follows.Close()

	for follows.Next() {
		var followID, followed string
		if err := follows.Scan(&followID, &followed); err != nil {
			return err
		}

		if err := Unfollow(ctx, domain, db, actorID, followed, followID); err != nil {
			return err
		}
	}

	return nil
}

func undoShares(ctx context.Context, domain, actorID string, db *sql.DB) error {
	shares, err := db.QueryContext(ctx, `select activity from outbox where activity->>'$.actor' = $1 and sender = $1 and activity->>'$.type' = 'Announce' order by inserted`, actorID)
	if err != nil {
		return err
	}
	defer shares.Close()

	for shares.Next() {
		var share ap.Activity
		if err := shares.Scan(&share); err != nil {
			return err
		}

		if err := Undo(ctx, domain, db, &share); err != nil {
			return err
		}
	}

	return nil
}

func deletePosts(ctx context.Context, domain, actorID string, cfg *cfg.Config, db *sql.DB) error {
	posts, err := db.QueryContext(ctx, `select object from notes where author = ? order by inserted`, actorID)
	if err != nil {
		return err
	}
	defer posts.Close()

	for posts.Next() {
		var post ap.Object
		if err := posts.Scan(&post); err != nil {
			return err
		}

		if err := Delete(ctx, domain, cfg, db, &post); err != nil {
			return err
		}
	}

	return nil
}

func Suspend(ctx context.Context, domain, user string, cfg *cfg.Config, db *sql.DB) error {
	actorID := fmt.Sprintf("https://%s/user/%s", domain, user)

	var actor ap.Actor
	if err := db.QueryRowContext(ctx, `select actor from persons where id = ?`, actorID).Scan(&actor); err != nil {
		return err
	}

	now := time.Now()

	// clear display name, summary and avatar
	actor.Name = ""
	actor.Summary = "Suspended " + now.Format(time.DateOnly)
	actor.Icon = nil

	// mark as suspended
	actor.Suspended = true

	actor.Updated = &ap.Time{Time: now}

	// deny access
	if _, err := db.ExecContext(ctx, `update persons set privkey = null, actor = ? where id = ?`, &actor, actorID); err != nil {
		return err
	}

	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	if err := UpdateActor(ctx, domain, tx, actorID); err != nil {
		return err
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	if err := undoFollows(ctx, domain, actorID, db); err != nil {
		return err
	}

	if err := undoShares(ctx, domain, actorID, db); err != nil {
		return err
	}

	if err := deletePosts(ctx, domain, actorID, cfg, db); err != nil {
		return err
	}

	if _, err := db.ExecContext(ctx, `delete from feed where follower = ?`, actorID); err != nil {
		return err
	}

	if _, err := db.ExecContext(ctx, `delete from feed where followed = ?`, actorID); err != nil {
		return err
	}

	if _, err := db.ExecContext(ctx, `delete from feed where author->>'$.id' = ?`, actorID); err != nil {
		return err
	}

	if _, err := db.ExecContext(ctx, `update feed set sharer = null where sharer->>'$.id' = ?`, actorID); err != nil {
		return err
	}

	if _, err := db.ExecContext(ctx, `delete from bookmarks where by = ?`, actorID); err != nil {
		return err
	}

	return nil
}
