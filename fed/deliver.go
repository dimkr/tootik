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

package fed

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/data"
	"github.com/dimkr/tootik/httpsig"
	"hash/crc32"
	"log/slog"
	"net/url"
	"sync"
	"time"
)

type Queue struct {
	Domain   string
	Config   *cfg.Config
	Log      *slog.Logger
	DB       *sql.DB
	Resolver *Resolver
}

type delivery struct {
	Activity    ap.Activity
	RawActivity string
	PrivKeyPem  string
	Actor       ap.Actor
	Inserted    int64
	Attempts    int
}

// Process polls the queue of outgoing activities and delivers them to other servers with timeout and retries.
func (q *Queue) Process(ctx context.Context) error {
	t := time.NewTicker(q.Config.OutboxPollingInterval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil

		case <-t.C:
			for {
				if n, err := q.process(ctx); err != nil {
					q.Log.Error("Failed to deliver posts", "error", err)
					break
				} else if n == 0 {
					q.Log.Debug("Nothing to deliver")
					break
				}
			}
		}
	}
}

func (q *Queue) process(ctx context.Context) (int, error) {
	chs := make([]chan *delivery, 0, q.Config.DeliveryBatchSize)
	var wg sync.WaitGroup

	defer func() {
		for _, ch := range chs {
			close(ch)
		}
		wg.Wait()
	}()

	for i := range q.Config.DeliveryWorkers {
		ch := make(chan *delivery)

		wg.Add(1)
		go func() {
			q.worker(ctx, i, ch)
			wg.Done()
		}()

		chs = append(chs, ch)
	}

	rows, err := q.DB.QueryContext(ctx, `select outbox.attempts, outbox.activity, outbox.activity, outbox.inserted, persons.actor, persons.privkey from outbox join persons on persons.id = outbox.sender where outbox.sent = 0 and (outbox.attempts = 0 or (outbox.attempts < ? and outbox.last <= unixepoch() - ?)) order by outbox.attempts asc, outbox.last asc limit ?`, q.Config.MaxDeliveryAttempts, q.Config.DeliveryRetryInterval, q.Config.DeliveryBatchSize)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch posts to deliver: %w", err)
	}
	defer rows.Close()

	n := 0
	for rows.Next() {
		var d delivery
		if err := rows.Scan(&d.Attempts, &d.Activity, &d.RawActivity, &d.Inserted, &d.Actor, &d.PrivKeyPem); err != nil {
			q.Log.Error("Failed to fetch post to deliver", "error", err)
			continue
		}

		chs[crc32.ChecksumIEEE([]byte(d.Activity.ID))%uint32(len(chs))] <- &d
		n++
	}

	return n, nil
}

func (q *Queue) worker(ctx context.Context, id int, ch <-chan *delivery) {
	for {
		select {
		case <-ctx.Done():
			return

		case d, ok := <-ch:
			if !ok {
				return
			}

			privKey, err := data.ParsePrivateKey(d.PrivKeyPem)
			if err != nil {
				q.Log.Error("Failed to parse private key", "worker", id, "error", err)
				continue
			}

			if _, err := q.DB.ExecContext(ctx, `update outbox set last = unixepoch(), attempts = ? where activity->>'$.id' = ? and sender = ?`, d.Attempts+1, d.Activity.ID, d.Actor.ID); err != nil {
				q.Log.Error("Failed to save last delivery attempt time", "worker", id, "id", d.Activity.ID, "attempts", d.Attempts, "error", err)
				continue
			}

			if err := q.deliverWithTimeout(ctx, &d.Activity, []byte(d.RawActivity), &d.Actor, httpsig.Key{ID: d.Actor.PublicKey.ID, PrivateKey: privKey}, time.Unix(d.Inserted, 0)); err != nil {
				q.Log.Warn("Failed to deliver activity", "worker", id, "id", d.Activity.ID, "attempts", d.Attempts, "error", err)
				continue
			}

			if _, err := q.DB.ExecContext(ctx, `update outbox set sent = 1 where activity->>'$.id' = ? and sender = ?`, d.Activity.ID, d.Actor.ID); err != nil {
				q.Log.Error("Failed to mark delivery as completed", "worker", id, "id", d.Activity.ID, "attempts", d.Attempts, "error", err)
				continue
			}

			q.Log.Info("Successfully delivered an activity", "worker", id, "id", d.Activity.ID, "attempts", d.Attempts)
		}
	}
}

func (q *Queue) deliverWithTimeout(parent context.Context, activity *ap.Activity, rawActivity []byte, actor *ap.Actor, key httpsig.Key, inserted time.Time) error {
	ctx, cancel := context.WithTimeout(parent, q.Config.DeliveryTimeout)
	defer cancel()
	return q.deliver(ctx, activity, rawActivity, actor, key, inserted)
}

func (q *Queue) deliver(ctx context.Context, activity *ap.Activity, rawActivity []byte, actor *ap.Actor, key httpsig.Key, inserted time.Time) error {
	activityID, err := url.Parse(activity.ID)
	if err != nil {
		return err
	}

	recipients := ap.Audience{}

	// deduplicate recipients or skip if we're forwarding an activity
	if activity.Actor == actor.ID {
		activity.To.Range(func(id string, _ struct{}) bool {
			recipients.Add(id)
			return true
		})

		activity.CC.Range(func(id string, _ struct{}) bool {
			recipients.Add(id)
			return true
		})
	}

	actorIDs := ap.Audience{}
	wideDelivery := activity.Actor != actor.ID || activity.IsPublic() || recipients.Contains(actor.Followers)

	// list the actor's federated followers if we're forwarding an activity by another actor, or if addressed by actor
	if wideDelivery {
		followers, err := q.DB.QueryContext(ctx, `select distinct follower from follows where followed = ? and follower not like ? and follower not like ? and accepted = 1 and inserted < ?`, actor.ID, fmt.Sprintf("https://%s/%%", q.Domain), fmt.Sprintf("https://%s/%%", activityID.Host), inserted.Unix())
		if err != nil {
			q.Log.Warn("Failed to list followers", "activity", activity.ID, "error", err)
		} else {
			for followers.Next() {
				var follower string
				if err := followers.Scan(&follower); err != nil {
					q.Log.Warn("Skipped a follower", "activity", activity.ID, "error", err)
					continue
				}

				actorIDs.Add(follower)
			}

			followers.Close()
		}
	}

	// assume that all other federated recipients are actors and not collections
	recipients.Range(func(recipient string, _ struct{}) bool {
		actorIDs.Add(recipient)
		return true
	})

	anyFailed := false

	var author string
	if obj, ok := activity.Object.(*ap.Object); ok {
		author = obj.AttributedTo
	}

	sent := map[string]struct{}{}

	var followers partialFollowers
	if recipients.Contains(actor.Followers) {
		followers = partialFollowers{}
	}

	actorIDs.Range(func(actorID string, _ struct{}) bool {
		if actorID == author || actorID == ap.Public {
			q.Log.Debug("Skipping recipient", "to", actorID, "activity", activity.ID)
			return true
		}

		to, err := q.Resolver.ResolveID(ctx, q.Log, q.DB, key, actorID, 0)
		if err != nil {
			q.Log.Warn("Failed to resolve a recipient", "to", actorID, "activity", activity.ID, "error", err)
			if !errors.Is(err, ErrActorGone) && !errors.Is(err, ErrBlockedDomain) {
				anyFailed = true
			}
			return true
		}

		// if possible, use the recipients's shared inbox and skip other recipients with the same shared inbox
		inbox := to.Inbox
		if wideDelivery {
			if sharedInbox, ok := to.Endpoints["sharedInbox"]; ok && sharedInbox != "" {
				q.Log.Debug("Using shared inbox inbox", "to", actorID, "activity", activity.ID, "shared_inbox", inbox)
				inbox = sharedInbox
			}
		}

		if _, ok := sent[inbox]; ok {
			q.Log.Info("Skipping recipient", "to", actorID, "activity", activity.ID, "inbox", inbox)
			return true
		}

		var delivered int
		if err := q.DB.QueryRowContext(ctx, `select exists (select 1 from deliveries where activity = ? and inbox = ?)`, activity.ID, inbox).Scan(&delivered); err != nil {
			q.Log.Error("Failed to check if delivered already", "to", actorID, "activity", activity.ID, "inbox", inbox, "error", err)
			anyFailed = true
			return false
		}

		sent[inbox] = struct{}{}

		if delivered == 1 {
			q.Log.Info("Skipping recipient", "to", actorID, "activity", activity.ID, "inbox", inbox)
			return true
		}

		q.Log.Info("Delivering activity to recipient", "to", actorID, "inbox", inbox, "activity", activity.ID)

		if err := q.Resolver.post(ctx, q.Log, q.DB, actor, key, followers, inbox, rawActivity); err != nil {
			if errors.Is(err, ErrLocalInbox) {
				q.Log.Info("Skipping local recipient", "from", actor.ID, "to", actorID, "activity", activity.ID, "error", err)
				return true
			}

			q.Log.Warn("Failed to send an activity", "from", actor.ID, "to", actorID, "activity", activity.ID, "error", err)
			if !errors.Is(err, ErrBlockedDomain) {
				anyFailed = true
			}
			return true
		}

		if _, err := q.DB.ExecContext(ctx, `insert into deliveries(activity, inbox) values (?, ?)`, activity.ID, inbox); err != nil {
			q.Log.Error("Failed to record delivery", "activity", activity.ID, "inbox", inbox, "error", err)
			anyFailed = true
			return false
		}

		return true
	})

	if anyFailed {
		return fmt.Errorf("failed to deliver activity %s to at least one recipient", activity.ID)
	}

	return nil
}
