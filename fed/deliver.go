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
	"bytes"
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
	"net/http"
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
	Job     deliveryJob
	Key     httpsig.Key
	Request *http.Request
	Inbox   string
}

type deliveryEvent struct {
	Job  deliveryJob
	Done bool
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

	events := make(chan deliveryEvent)
	tasks := make([]chan deliveryTask, 0, q.Config.DeliveryWorkers)
	var wg sync.WaitGroup
	done := make(chan map[deliveryJob]bool)

	wg.Add(q.Config.DeliveryWorkers)
	for range q.Config.DeliveryWorkers {
		ch := make(chan deliveryTask, q.Config.DeliveryWorkerBuffer)

		go func() {
			q.consume(ctx, ch, events)
			wg.Done()
		}()

		tasks = append(tasks, ch)
	}

	go func() {
		results := make(map[deliveryJob]bool, q.Config.DeliveryBatchSize)

		for event := range events {
			results[event.Job] = event.Done
		}

		done <- results
	}()

	followers := partialFollowers{}

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

		job := deliveryJob{
			Activity: &activity,
			Sender:   &actor,
		}

		// notify about the new job
		events <- deliveryEvent{job, true}

		// queue tasks for all outgoing requests
		if err := q.queueTasks(ctx, job, []byte(rawActivity), httpsig.Key{ID: actor.PublicKey.ID, PrivateKey: privKey}, time.Unix(inserted, 0), &followers, tasks, events); err != nil {
			q.Log.Warn("Failed to queue activity for delivery", "id", activity.ID, "attempts", deliveryAttempts, "error", err)
		}
	}

	// wait for all tasks to finish
	for _, ch := range tasks {
		close(ch)
	}
	wg.Wait()

	// receive all job results
	close(events)
	results := <-done

	for job, done := range results {
		if !done {
			q.Log.Info("Failed to deliver an activity to at least one recipient", "id", job.Activity.ID)
			continue
		}

		if _, err := q.DB.ExecContext(ctx, `update outbox set sent = 1 where activity->>'$.id' = ? and sender = ?`, job.Activity.ID, job.Sender.ID); err != nil {
			q.Log.Error("Failed to mark delivery as completed", "id", job.Activity.ID, "error", err)
		} else {
			q.Log.Info("Successfully delivered an activity to all recipients", "id", job.Activity.ID)
		}
	}

	return nil
}

func (q *Queue) deliverWithTimeout(parent context.Context, task deliveryTask) error {
	ctx, cancel := context.WithTimeout(parent, q.Config.DeliveryTimeout)
	defer cancel()

	req := task.Request.WithContext(ctx)

	resp, err := q.Resolver.send(q.Log, task.Key, req)
	if err == nil {
		resp.Body.Close()
	}
	return err
}

func (q *Queue) consume(ctx context.Context, requests <-chan deliveryTask, events chan<- deliveryEvent) {
	tried := map[string]map[string]struct{}{}

	for task := range requests {
		if m, ok := tried[task.Job.Activity.ID]; ok {
			if _, ok := m[task.Inbox]; ok {
				continue
			}
			m[task.Inbox] = struct{}{}
		} else {
			tried[task.Job.Activity.ID] = map[string]struct{}{task.Inbox: {}}
		}

		var delivered int
		if err := q.DB.QueryRowContext(ctx, `select exists (select 1 from deliveries where activity = ? and inbox = ?)`, task.Job.Activity.ID, task.Inbox).Scan(&delivered); err != nil {
			q.Log.Error("Failed to check if delivered already", "to", task.Inbox, "activity", task.Job.Activity.ID, "error", err)
			events <- deliveryEvent{task.Job, false}
			continue
		}

		if delivered == 1 {
			q.Log.Info("Skipping recipient", "to", task.Inbox, "activity", task.Job.Activity.ID)
			continue
		}

		q.Log.Info("Delivering activity to recipient", "inbox", task.Inbox, "activity", task.Job.Activity.ID)

		if err := q.deliverWithTimeout(ctx, task); err == nil {
			q.Log.Info("Successfully sent an activity", "from", task.Job.Sender.ID, "to", task.Inbox, "activity", task.Job.Activity.ID)
		} else {
			q.Log.Warn("Failed to send an activity", "from", task.Job.Sender.ID, "to", task.Inbox, "activity", task.Job.Activity.ID, "error", err)
			if !errors.Is(err, ErrBlockedDomain) {
				events <- deliveryEvent{task.Job, false}
			}

			continue
		}

		if _, err := q.DB.ExecContext(ctx, `insert into deliveries(activity, inbox) values (?, ?)`, task.Job.Activity.ID, task.Inbox); err != nil {
			q.Log.Error("Failed to record delivery", "activity", task.Job.Activity.ID, "inbox", task.Inbox, "error", err)
			events <- deliveryEvent{task.Job, false}
		}
	}
}

func (q *Queue) queueTasks(ctx context.Context, job deliveryJob, rawActivity []byte, key httpsig.Key, inserted time.Time, followers *partialFollowers, tasks []chan deliveryTask, events chan<- deliveryEvent) error {
	activityID, err := url.Parse(job.Activity.ID)
	if err != nil {
		return err
	}

	recipients := ap.Audience{}

	// deduplicate recipients or skip if we're forwarding an activity
	if job.Activity.Actor == job.Sender.ID {
		for id := range job.Activity.To.Keys() {
			recipients.Add(id)
		}

		for id := range job.Activity.CC.Keys() {
			recipients.Add(id)
		}
	}

	actorIDs := ap.Audience{}
	wideDelivery := job.Activity.Actor != job.Sender.ID || job.Activity.IsPublic() || recipients.Contains(job.Sender.Followers)

	// list the actor's federated followers if we're forwarding an activity by another actor, or if addressed by actor
	if wideDelivery {
		followers, err := q.DB.QueryContext(ctx, `select distinct follower from follows where followed = ? and follower not like ? and follower not like ? and accepted = 1 and inserted < ?`, job.Sender.ID, fmt.Sprintf("https://%s/%%", q.Domain), fmt.Sprintf("https://%s/%%", activityID.Host), inserted.Unix())
		if err != nil {
			q.Log.Warn("Failed to list followers", "activity", job.Activity.ID, "error", err)
		} else {
			for followers.Next() {
				var follower string
				if err := followers.Scan(&follower); err != nil {
					q.Log.Warn("Skipped a follower", "activity", job.Activity.ID, "error", err)
					continue
				}

				actorIDs.Add(follower)
			}

			followers.Close()
		}
	}

	// assume that all other federated recipients are actors and not collections
	for recipient := range recipients.Keys() {
		actorIDs.Add(recipient)
	}

	var author string
	if obj, ok := job.Activity.Object.(*ap.Object); ok {
		author = obj.AttributedTo
	}

	for actorID := range actorIDs.Keys() {
		if actorID == author || actorID == ap.Public {
			q.Log.Debug("Skipping recipient", "to", actorID, "activity", job.Activity.ID)
			continue
		}

		to, err := q.Resolver.ResolveID(ctx, q.Log, q.DB, key, actorID, ap.Offline)
		if err != nil {
			q.Log.Warn("Failed to resolve a recipient", "to", actorID, "activity", job.Activity.ID, "error", err)
			if !errors.Is(err, ErrActorGone) && !errors.Is(err, ErrBlockedDomain) {
				events <- deliveryEvent{job, false}
			}
			continue
		}

		// if possible, use the recipients's shared inbox and skip other recipients with the same shared inbox
		inbox := to.Inbox
		if wideDelivery {
			if sharedInbox, ok := to.Endpoints["sharedInbox"]; ok && sharedInbox != "" {
				q.Log.Debug("Using shared inbox", "to", actorID, "activity", job.Activity.ID, "shared_inbox", inbox)
				inbox = sharedInbox
			}
		}

		req, err := http.NewRequest(http.MethodPost, inbox, bytes.NewReader(rawActivity))
		if err != nil {
			q.Log.Warn("Failed to create new request", "to", actorID, "activity", job.Activity.ID, "inbox", inbox, "error", err)
			events <- deliveryEvent{job, false}
			continue
		}

		if req.URL.Host == q.Domain {
			q.Log.Debug("Skipping local recipient inbox", "to", actorID, "activity", job.Activity.ID, "inbox", inbox)
			continue
		}

		req.Header.Set("User-Agent", userAgent)
		req.Header.Set("Accept", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)

		if recipients.Contains(job.Sender.Followers) {
			if digest, err := followers.Digest(ctx, q.DB, q.Domain, job.Sender, req.URL.Host); err == nil {
				req.Header.Set("Collection-Synchronization", digest)
			} else {
				q.Log.Warn("Failed to digest followers", "to", actorID, "activity", job.Activity.ID, "inbox", inbox, "error", err)
			}
		}

		q.Log.Info("Queueing activity for delivery", "inbox", inbox, "activity", job.Activity.ID)

		tasks[crc32.ChecksumIEEE([]byte(inbox))%uint32(len(tasks))] <- deliveryTask{
			Job:     job,
			Key:     key,
			Request: req,
			Inbox:   inbox,
		}
	}

	return nil
}
