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

type deliveryJob struct {
	Activity *ap.Activity
	Sender   *ap.Actor
}

type deliveryTask struct {
	deliveryJob
	Key       httpsig.Key
	Followers partialFollowers
	Inbox     string
	Body      []byte
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
			if err := q.process(ctx); err != nil {
				q.Log.Error("Failed to deliver posts", "error", err)
			}
		}
	}
}

func (q *Queue) process(ctx context.Context) error {
	q.Log.Debug("Polling delivery queue")

	rows, err := q.DB.QueryContext(ctx, `select outbox.attempts, outbox.activity, outbox.activity, outbox.inserted, persons.actor, persons.privkey from outbox join persons on persons.id = outbox.sender where outbox.sent = 0 and (outbox.attempts = 0 or (outbox.attempts < ? and outbox.last <= unixepoch() - ?)) order by outbox.attempts asc, outbox.last asc limit ?`, q.Config.MaxDeliveryAttempts, q.Config.DeliveryRetryInterval, q.Config.DeliveryBatchSize)
	if err != nil {
		return fmt.Errorf("failed to fetch posts to deliver: %w", err)
	}
	defer rows.Close()

	status := make(map[deliveryJob]bool, q.Config.DeliveryBatchSize)

	tasks := make([]chan deliveryTask, 0, q.Config.DeliveryWorkers)
	for range q.Config.DeliveryWorkers {
		tasks = append(tasks, make(chan deliveryTask))
	}
	failures := make(chan deliveryJob)

	var workers sync.WaitGroup

	workers.Add(q.Config.DeliveryWorkers)
	for i := range q.Config.DeliveryWorkers {
		go func() {
			q.worker(ctx, tasks[i], failures)
			workers.Done()
		}()
	}

	for rows.Next() {
		var activity ap.Activity
		var rawActivity, privKeyPem string
		var actor ap.Actor
		var inserted int64
		var deliveryAttempts int
		if err := rows.Scan(&deliveryAttempts, &activity, &rawActivity, &inserted, &actor, &privKeyPem); err != nil {
			q.Log.Error("Failed to fetch post to deliver", "error", err)
			continue
		}

		privKey, err := data.ParsePrivateKey(privKeyPem)
		if err != nil {
			q.Log.Error("Failed to parse private key", "error", err)
			continue
		}

		if _, err := q.DB.ExecContext(ctx, `update outbox set last = unixepoch(), attempts = ? where activity->>'$.id' = ? and sender = ?`, deliveryAttempts+1, activity.ID, actor.ID); err != nil {
			q.Log.Error("Failed to save last delivery attempt time", "id", activity.ID, "attempts", deliveryAttempts, "error", err)
			continue
		}

		status[deliveryJob{
			Activity: &activity,
			Sender:   &actor,
		}] = true

		if err := q.queueDeliveries(ctx, &activity, []byte(rawActivity), &actor, httpsig.Key{ID: actor.PublicKey.ID, PrivateKey: privKey}, time.Unix(inserted, 0), tasks, failures); err != nil {
			q.Log.Warn("Failed to queue activity for delivery", "id", activity.ID, "attempts", deliveryAttempts, "error", err)
			continue
		}
	}

	for _, ch := range tasks {
		close(ch)
	}

	done := make(chan struct{})

	go func() {
		for job := range failures {
			status[job] = false
		}

		for job, ok := range status {
			if !ok {
				q.Log.Info("Failed to deliver an activity to at least one recipient", "id", job.Activity.ID)
				continue
			}

			if _, err := q.DB.ExecContext(ctx, `update outbox set sent = 1 where activity->>'$.id' = ? and sender = ?`, job.Activity.ID, job.Sender.ID); err != nil {
				q.Log.Error("Failed to mark delivery as completed", "id", job.Activity.ID, "error", err)
			} else {
				q.Log.Info("Successfully delivered an activity", "id", job.Activity.ID)
			}
		}

		done <- struct{}{}
	}()

	workers.Wait()
	close(failures)
	<-done

	return nil
}

func (q *Queue) deliverWithTimeout(parent context.Context, d deliveryTask) error {
	ctx, cancel := context.WithTimeout(parent, q.Config.DeliveryTimeout)
	defer cancel()
	return q.Resolver.post(ctx, q.Log, q.DB, d.Sender, d.Key, d.Followers, d.Inbox, d.Body)
}

func (q *Queue) worker(ctx context.Context, requests <-chan deliveryTask, failures chan<- deliveryJob) {
	for task := range requests {
		var delivered int
		if err := q.DB.QueryRowContext(ctx, `select exists (select 1 from deliveries where activity = ? and inbox = ?)`, task.Activity.ID, task.Inbox).Scan(&delivered); err != nil {
			q.Log.Error("Failed to check if delivered already", "to", task.Inbox, "activity", task.Activity.ID, "error", err)
			failures <- task.deliveryJob
			continue
		}

		if delivered == 1 {
			q.Log.Info("Skipping recipient", "to", task.Inbox, "activity", task.Activity.ID)
			continue
		}

		q.Log.Info("Delivering activity to recipient", "inbox", task.Inbox, "activity", task.Activity.ID)

		if err := q.deliverWithTimeout(ctx, task); err != nil {
			if errors.Is(err, ErrLocalInbox) {
				q.Log.Info("Skipping local recipient", "from", task.Sender.ID, "to", task.Inbox, "activity", task.Activity.ID)
			} else {
				q.Log.Warn("Failed to send an activity", "from", task.Sender.ID, "to", task.Inbox, "activity", task.Activity.ID, "error", err)
				if !errors.Is(err, ErrBlockedDomain) {
					failures <- task.deliveryJob
				}
			}

			continue
		}

		if _, err := q.DB.ExecContext(ctx, `insert into deliveries(activity, inbox) values (?, ?)`, task.Activity.ID, task.Inbox); err != nil {
			q.Log.Error("Failed to record delivery", "activity", task.Activity.ID, "inbox", task.Inbox, "error", err)
			failures <- task.deliveryJob
		}
	}
}

func (q *Queue) queueDeliveries(ctx context.Context, activity *ap.Activity, rawActivity []byte, actor *ap.Actor, key httpsig.Key, inserted time.Time, tasks []chan deliveryTask, failures chan<- deliveryJob) error {
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

	var author string
	if obj, ok := activity.Object.(*ap.Object); ok {
		author = obj.AttributedTo
	}

	queued := map[string]struct{}{}

	var followers partialFollowers
	if recipients.Contains(actor.Followers) {
		followers = partialFollowers{}
	}

	job := deliveryJob{
		Activity: activity,
		Sender:   actor,
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
				failures <- job
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

		if _, ok := queued[inbox]; ok {
			return true
		}

		q.Log.Info("Queueing activity for delivery", "inbox", inbox, "activity", activity.ID)

		tasks[crc32.ChecksumIEEE([]byte(inbox))%uint32(len(tasks))] <- deliveryTask{
			deliveryJob: job,
			Key:         key,
			Followers:   followers,
			Inbox:       inbox,
			Body:        rawActivity,
		}

		queued[inbox] = struct{}{}
		return true
	})

	return nil
}
