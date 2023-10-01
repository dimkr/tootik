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

package outbox

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/cfg"
	"strings"
	"time"
)

func Follow(ctx context.Context, follower *ap.Actor, followed string, db *sql.DB) error {
	if followed == follower.ID {
		return fmt.Errorf("%s cannot follow %s", follower.ID, followed)
	}

	followID := fmt.Sprintf("https://%s/follow/%x", cfg.Domain, sha256.Sum256([]byte(fmt.Sprintf("%s|%s|%d", follower.ID, followed, time.Now().UnixNano()))))

	to := ap.Audience{}
	to.Add(followed)

	body, err := json.Marshal(ap.Activity{
		ID:     followID,
		Type:   ap.FollowActivity,
		Actor:  follower.ID,
		Object: followed,
		To:     to,
	})
	if err != nil {
		return fmt.Errorf("Failed to marshal follow: %w", err)
	}

	isLocal := strings.HasPrefix(followed, fmt.Sprintf("https://%s/", cfg.Domain))

	if _, err := db.ExecContext(
		ctx,
		`INSERT INTO follows (id, follower, followed, accepted) VALUES(?,?,?,?)`,
		followID,
		follower.ID,
		followed,
		isLocal, // local follows don't need to be accepted
	); err != nil {
		return fmt.Errorf("Failed to insert follow: %w", err)
	}

	if _, err := db.ExecContext(
		ctx,
		`INSERT INTO outbox (activity) VALUES(?)`,
		string(body),
	); err != nil {
		return fmt.Errorf("Failed to insert follow activity: %w", err)
	}

	return nil
}