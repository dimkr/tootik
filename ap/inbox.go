/*
Copyright 2025, 2026 Dima Krasner

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
	"github.com/dimkr/tootik/httpsig"
)

// Inbox creates and processes activities.
type Inbox interface {
	NewID(actorID, prefix string) (string, error)
	Accept(ctx context.Context, followed *Actor, key httpsig.Key, follower, followID string, tx *sql.Tx) error
	Announce(ctx context.Context, tx *sql.Tx, actor *Actor, key httpsig.Key, note *Object) error
	Create(ctx context.Context, cfg *cfg.Config, post *Object, author *Actor, key httpsig.Key) error
	Delete(ctx context.Context, actor *Actor, key httpsig.Key, note *Object) error
	Follow(ctx context.Context, follower *Actor, key httpsig.Key, followed string) error
	Move(ctx context.Context, from *Actor, key httpsig.Key, to string) error
	Reject(ctx context.Context, followed *Actor, key httpsig.Key, follower, followID string, tx *sql.Tx) error
	Undo(ctx context.Context, actor *Actor, key httpsig.Key, activity *Activity) error
	UpdateActorTx(ctx context.Context, tx *sql.Tx, actor *Actor, key httpsig.Key) error
	UpdateActor(ctx context.Context, actor *Actor, key httpsig.Key) error
	UpdateNote(ctx context.Context, actor *Actor, key httpsig.Key, note *Object) error
	Unfollow(ctx context.Context, follower *Actor, key httpsig.Key, followed, followID string) error
	ProcessActivity(ctx context.Context, tx *sql.Tx, path sql.NullString, sender *Actor, activity *Activity, rawActivity string, depth int, shared bool) error
}
