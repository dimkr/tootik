/*
Copyright 2025 Dima Krasner

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

package ap

import (
	"context"
	"database/sql"

	"github.com/dimkr/tootik/cfg"
)

// Inbox creates and processes activities.
type Inbox interface {
	NewID(actorID, prefix string) (string, error)
	Accept(ctx context.Context, followed *Actor, follower, followID string, tx *sql.Tx) error
	Announce(ctx context.Context, tx *sql.Tx, actor *Actor, note *Object) error
	Create(ctx context.Context, cfg *cfg.Config, db *sql.DB, post *Object, author *Actor) error
	Delete(ctx context.Context, db *sql.DB, actor *Actor, note *Object) error
	Follow(ctx context.Context, follower *Actor, followed string, db *sql.DB) error
	Move(ctx context.Context, db *sql.DB, from *Actor, to string) error
	Reject(ctx context.Context, followed *Actor, follower, followID string, tx *sql.Tx) error
	Undo(ctx context.Context, db *sql.DB, actor *Actor, activity *Activity) error
	UpdateActor(ctx context.Context, tx *sql.Tx, actorID string) error
	UpdateNote(ctx context.Context, db *sql.DB, actor *Actor, note *Object) error
	Unfollow(ctx context.Context, db *sql.DB, follower *Actor, followed, followID string) error
	ProcessActivity(ctx context.Context, tx *sql.Tx, sender *Actor, activity *Activity, rawActivity string, depth int, shared bool) error
}
