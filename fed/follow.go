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

func Follow(ctx context.Context, log *slog.Logger, follower *ap.Actor, followed string, db *sql.DB, resolver *Resolver) error {
	if followed == follower.ID {
		return fmt.Errorf("%s cannot follow %s", follower.ID, followed)
	}

	followID := fmt.Sprintf("https://%s/follow/%x", cfg.Domain, sha256.Sum256([]byte(fmt.Sprintf("%s|%s", follower.ID, followed))))

	body, err := json.Marshal(map[string]any{
		"@context": "https://www.w3.org/ns/activitystreams",
		"id":       followID,
		"type":     ap.FollowActivity,
		"actor":    follower.ID,
		"object":   followed,
	})
	if err != nil {
		return fmt.Errorf("%s cannot follow %s: %w", follower.ID, followed, err)
	}

	to, err := resolver.Resolve(ctx, log, db, follower, followed)
	if err != nil {
		return fmt.Errorf("%s cannot follow %s: %w", follower.ID, followed, err)
	}
	followed = to.ID

	if err := Send(ctx, log, db, follower, resolver, to, body); err != nil {
		return fmt.Errorf("Failed to send follow %s: %w", followID, err)
	}

	if _, err := db.ExecContext(ctx, `delete from follows where id = ?`, followID); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("Failed to delete duplicate of follow %s: %w", followID, err)
	}

	if _, err := db.ExecContext(
		ctx,
		`INSERT INTO follows (id, follower, followed) VALUES(?,?,?)`,
		followID,
		follower.ID,
		followed,
	); err != nil {
		return fmt.Errorf("Failed to insert follow %s: %w", followID, err)
	}

	return nil
}
