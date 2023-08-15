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
	"log/slog"
)

func Unfollow(ctx context.Context, log *slog.Logger, db *sql.DB, follower *ap.Actor, followed, followID string) error {
	if followed == follower.ID {
		return fmt.Errorf("%s cannot unfollow %s", follower.ID, followed)
	}

	undoID := fmt.Sprintf("https://%s/undo/%x", cfg.Domain, sha256.Sum256([]byte(fmt.Sprintf("%s|%s", follower.ID, followed))))

	body, err := json.Marshal(map[string]any{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id":       undoID,
		"type":     ap.UndoActivity,
		"actor":    follower.ID,
		"object": map[string]any{
			"id":     followID,
			"type":   ap.FollowObject,
			"actor":  follower.ID,
			"object": followed,
		},
	})
	if err != nil {
		return fmt.Errorf("%s cannot unfollow %s: %w", follower.ID, followed, err)
	}

	resolver, err := Resolvers.Borrow(ctx)
	if err != nil {
		return fmt.Errorf("%s cannot unfollow %s: %w", follower.ID, followed, err)
	}

	to, err := resolver.Resolve(ctx, log, db, follower, followed)
	if err != nil {
		Resolvers.Return(resolver)
		return fmt.Errorf("%s cannot unfollow %s: %w", follower.ID, followed, err)
	}
	followed = to.ID

	if err := Send(ctx, log, db, follower, resolver, to, body); err != nil {
		Resolvers.Return(resolver)
		return fmt.Errorf("Failed to send unfollow %s: %w", followID, err)
	}

	Resolvers.Return(resolver)

	if _, err := db.ExecContext(ctx, `delete from follows where id = ?`, followID); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("Failed to unfollow %s: %w", followID, err)
	}

	return nil
}
