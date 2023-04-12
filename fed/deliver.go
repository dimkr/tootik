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
	log "github.com/sirupsen/logrus"
	"strings"
	"time"
)

const (
	batchSize             = 16
	deliveryRetryInterval = (time.Hour / 2) / time.Second
	maxDeliveryAttempts   = 5
	pollingInterval       = time.Second * 5
	deliveryTimeout       = time.Minute * 5
	maxDeliveryQueueSize  = 128
)

var DeliveryQueueFull = errors.New("Delivery queue is full")

func DeliverPosts(ctx context.Context, db *sql.DB, logger *log.Logger) {
	t := time.NewTicker(pollingInterval)

	for {
		select {
		case <-ctx.Done():
			return

		case <-t.C:
			if err := deliverPosts(ctx, db, logger); err != nil {
				logger.WithError(err).Error("Failed to deliver posts")
			}
		}
	}
}

func deliverPosts(ctx context.Context, db *sql.DB, logger *log.Logger) error {
	logger.Debug("Polling delivery queue")

	rows, err := db.QueryContext(ctx, `select deliveries.id, deliveries.attempts, notes.object, persons.actor from deliveries join notes on notes.id = deliveries.id join persons on persons.id = notes.author where deliveries.attempts < ? and deliveries.last > unixepoch() - ? order by deliveries.attempts asc, deliveries.last asc limit ?`, maxDeliveryAttempts, deliveryRetryInterval, batchSize)
	if err != nil {
		return fmt.Errorf("Failed to fetch posts to deliver: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var deliveryID, noteString, authorString string
		var deliveryAttempts int
		if err := rows.Scan(&deliveryID, &deliveryAttempts, &noteString, &authorString); err != nil {
			logger.WithError(err).Error("Failed to fetch post to deliver")
			continue
		}

		if _, err := db.ExecContext(ctx, `update deliveries set last = unixepoch(), attempts = ? where id = ?`, deliveryID, deliveryAttempts+1); err != nil {
			logger.WithFields(log.Fields{"id": deliveryID, "attempts": deliveryAttempts}).WithError(err).Error("Failed to save last delivery attempt time")
			continue
		}

		var note ap.Object
		if err := json.Unmarshal([]byte(noteString), &note); err != nil {
			logger.WithFields(log.Fields{"id": deliveryID, "attempts": deliveryAttempts}).WithError(err).Error("Failed to unmarshal undelivered post")
			continue
		}

		var author ap.Actor
		if err := json.Unmarshal([]byte(authorString), &author); err != nil {
			logger.WithFields(log.Fields{"id": deliveryID, "attempts": deliveryAttempts}).WithError(err).Error("Failed to unmarshal undelivered post author")
			continue
		}

		if err := deliverWithTimeout(ctx, db, logger, &note, &author); err != nil {
			logger.WithFields(log.Fields{"id": deliveryID, "attempts": deliveryAttempts}).WithError(err).Warn("Failed to deliver post")
			continue
		}

		if _, err := db.ExecContext(ctx, `delete from deliveries where id = ?`, deliveryID); err != nil {
			logger.WithFields(log.Fields{"id": deliveryID, "attempts": deliveryAttempts}).WithError(err).Error("Failed to delete delivery")
			continue
		}

		logger.WithFields(log.Fields{"id": deliveryID, "attempts": deliveryAttempts}).Info("Successfully delivered a post")
	}

	return nil
}

func deliverWithTimeout(parent context.Context, db *sql.DB, logger *log.Logger, post *ap.Object, author *ap.Actor) error {
	ctx, cancel := context.WithTimeout(parent, deliveryTimeout)
	defer cancel()
	return deliver(ctx, db, logger, post, author)
}

func deliver(ctx context.Context, db *sql.DB, logger *log.Logger, post *ap.Object, author *ap.Actor) error {
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

	prefix := fmt.Sprintf("https://%s/", cfg.Domain)
	actorIDs := data.OrderedMap[string, struct{}]{}

	// list the author's federated followers
	if recipients.Contains(author.Followers) {
		followers, err := db.QueryContext(ctx, `select distinct follower from follows where followed = ?`, author.ID)
		if err != nil {
			logger.WithField("post", post.ID).WithError(err).Warn("Failed to list followers")
		} else {
			for followers.Next() {
				var follower string
				if err := followers.Scan(&follower); err != nil {
					logger.WithField("post", post.ID).WithError(err).Warn("Skipped a follower")
					continue
				}

				if !strings.HasPrefix(follower, prefix) {
					actorIDs.Store(follower, struct{}{})
				}
			}

			followers.Close()
		}
	}

	// assume that all other federated recipients are actors and not collections
	recipients.Range(func(recipient string, _ struct{}) bool {
		if recipient != ap.Public && !strings.HasPrefix(recipient, prefix) {
			actorIDs.Store(recipient, struct{}{})
		}

		return true
	})

	anyFailed := false

	actorIDs.Range(func(actorID string, _ struct{}) bool {
		logger.WithFields(log.Fields{"to": actorID, "post": post.ID}).Info("Delivering post to recipient")

		r, err := Resolvers.Borrow(ctx)
		if err != nil {
			logger.WithFields(log.Fields{"to": actorID, "post": post.ID}).WithError(err).Warn("Cannot resolve a recipient")
			anyFailed = true
			return true
		}

		if to, err := r.Resolve(ctx, db, author, actorID); err != nil {
			logger.WithFields(log.Fields{"to": actorID, "post": post.ID}).WithError(err).Warn("Failed to resolve a recipient")
			anyFailed = true
		} else if err := Send(ctx, db, author, r, to, create); err != nil {
			logger.WithFields(log.Fields{"to": to.ID, "post": post.ID}).WithError(err).Warn("Failed to send a post")
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

func Deliver(ctx context.Context, db *sql.DB, logger *log.Logger, post *ap.Object, author *ap.Actor) error {
	if err := note.Insert(ctx, db, post); err != nil {
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
