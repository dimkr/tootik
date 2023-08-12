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
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/data"
	"github.com/dimkr/tootik/note"
	"log/slog"
	"strings"
	"time"
)

const (
	batchSize             = 16
	deliveryRetryInterval = int64((time.Hour / 2) / time.Second)
	maxDeliveryAttempts   = 5
	pollingInterval       = time.Second * 5
	deliveryTimeout       = time.Minute * 5
	maxDeliveryQueueSize  = 128
)

var DeliveryQueueFull = errors.New("Delivery queue is full")

func DeliverPosts(ctx context.Context, log *slog.Logger, db *sql.DB) {
	t := time.NewTicker(pollingInterval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return

		case <-t.C:
			if err := deliverPosts(ctx, log, db); err != nil {
				log.Error("Failed to deliver posts", "error", err)
			}
		}
	}
}

func deliverPosts(ctx context.Context, log *slog.Logger, db *sql.DB) error {
	log.Debug("Polling delivery queue")

	rows, err := db.QueryContext(ctx, `select deliveries.id, deliveries.attempts, notes.object, persons.actor from deliveries join notes on notes.id = deliveries.id join persons on persons.id = notes.author where deliveries.attempts = 0 or (deliveries.attempts < ? and deliveries.last <= unixepoch() - ?) order by deliveries.attempts asc, deliveries.last asc limit ?`, maxDeliveryAttempts, deliveryRetryInterval, batchSize)
	if err != nil {
		return fmt.Errorf("Failed to fetch posts to deliver: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var deliveryID, noteString, authorString string
		var deliveryAttempts int
		if err := rows.Scan(&deliveryID, &deliveryAttempts, &noteString, &authorString); err != nil {
			log.Error("Failed to fetch post to deliver", "error", err)
			continue
		}

		if _, err := db.ExecContext(ctx, `update deliveries set last = unixepoch(), attempts = ? where id = ?`, deliveryAttempts+1, deliveryID); err != nil {
			log.Error("Failed to save last delivery attempt time", "id", deliveryID, "attempts", deliveryAttempts, "error", err)
			continue
		}

		var note ap.Object
		if err := json.Unmarshal([]byte(noteString), &note); err != nil {
			log.Error("Failed to unmarshal undelivered post", "id", deliveryID, "attempts", deliveryAttempts, "error", err)
			continue
		}

		var author ap.Actor
		if err := json.Unmarshal([]byte(authorString), &author); err != nil {
			log.Error("Failed to unmarshal undelivered post author", "id", deliveryID, "attempts", deliveryAttempts, "error", err)
			continue
		}

		if err := deliverWithTimeout(ctx, log, db, &note, &author); err != nil {
			log.Warn("Failed to deliver post", "id", deliveryID, "attempts", deliveryAttempts, "error", err)
			continue
		}

		if _, err := db.ExecContext(ctx, `delete from deliveries where id = ?`, deliveryID); err != nil {
			log.Error("Failed to delete delivery", "id", deliveryID, "attempts", deliveryAttempts, "error", err)
			continue
		}

		log.Info("Successfully delivered a post", "id", deliveryID, "attempts", deliveryAttempts)
	}

	return nil
}

func deliverWithTimeout(parent context.Context, log *slog.Logger, db *sql.DB, post *ap.Object, author *ap.Actor) error {
	ctx, cancel := context.WithTimeout(parent, deliveryTimeout)
	defer cancel()
	return deliver(ctx, log, db, post, author)
}

func deliver(ctx context.Context, log *slog.Logger, db *sql.DB, post *ap.Object, author *ap.Actor) error {
	create, err := json.Marshal(map[string]any{
		"@context": "https://www.w3.org/ns/activitystreams",
		"type":     ap.CreateActivity,
		"id":       post.ID,
		"actor":    author.ID,
		"object":   post,
	})
	if err != nil {
		return fmt.Errorf("Failed to marshal Create activity: %w", err)
	}

	// deduplicate recipients
	recipients := data.OrderedMap[string, struct{}]{}

	post.To.Range(func(id string, _ struct{}) bool {
		recipients.Store(id, struct{}{})
		return true
	})

	post.CC.Range(func(id string, _ struct{}) bool {
		recipients.Store(id, struct{}{})
		return true
	})

	actorIDs := data.OrderedMap[string, struct{}]{}

	// list the author's federated followers
	if post.IsPublic() || recipients.Contains(author.Followers) {
		followers, err := db.QueryContext(ctx, `select distinct follower from follows where followed = ? and follower not like ?`, author.ID, fmt.Sprintf("https://%s/%%", cfg.Domain))
		if err != nil {
			log.Warn("Failed to list followers", "post", post.ID, "error", err)
		} else {
			for followers.Next() {
				var follower string
				if err := followers.Scan(&follower); err != nil {
					log.Warn("Skipped a follower", "post", post.ID, "error", err)
					continue
				}

				actorIDs.Store(follower, struct{}{})
			}

			followers.Close()
		}
	}

	// assume that all other federated recipients are actors and not collections
	prefix := fmt.Sprintf("https://%s/", cfg.Domain)
	recipients.Range(func(recipient string, _ struct{}) bool {
		if recipient != ap.Public && !strings.HasPrefix(recipient, prefix) {
			actorIDs.Store(recipient, struct{}{})
		}

		return true
	})

	anyFailed := false

	actorIDs.Range(func(actorID string, _ struct{}) bool {
		log.Info("Delivering post to recipient", "to", actorID, "post", post.ID)

		r, err := Resolvers.Borrow(ctx)
		if err != nil {
			log.Warn("Cannot resolve a recipient", "to", actorID, "post", post.ID, "error", err)
			anyFailed = true
			return true
		}

		if to, err := r.Resolve(ctx, log, db, author, actorID); err != nil {
			log.Warn("Failed to resolve a recipient", "to", actorID, "post", post.ID, "error", err)
			anyFailed = true
		} else if err := Send(ctx, log, db, author, r, to, create); err != nil {
			log.Warn("Failed to send a post", "to", actorID, "post", post.ID, "error", err)
			anyFailed = true
		}

		Resolvers.Return(r)
		return true
	})

	if anyFailed {
		return fmt.Errorf("Failed to deliver post %s to at least one recipient", post.ID)
	}

	return nil
}

func Deliver(ctx context.Context, log *slog.Logger, db *sql.DB, post *ap.Object, author *ap.Actor) error {
	if err := note.Insert(ctx, db, post, log); err != nil {
		return fmt.Errorf("Failed to insert post: %w", err)
	}

	var queueSize int
	if err := db.QueryRowContext(ctx, `select count (*) from deliveries where attempts < ?`, maxDeliveryAttempts).Scan(&queueSize); err != nil {
		return fmt.Errorf("Failed to query delivery queue size: %w", err)
	}

	if queueSize >= maxDeliveryQueueSize {
		return DeliveryQueueFull
	}

	if _, err := db.ExecContext(ctx, `insert into deliveries(id) values(?)`, post.ID); err != nil {
		return fmt.Errorf("Failed to register post for delivery: %w", err)
	}

	return nil
}
