/*
Copyright 2023 - 2025 Dima Krasner

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
	"hash/crc32"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/data"
	"github.com/dimkr/tootik/httpsig"
	"github.com/dimkr/tootik/proof"
)

type Queue struct {
	Domain   string
	Config   *cfg.Config
	DB       *sql.DB
	Resolver *Resolver
}

type deliveryJob struct {
	Activity    *ap.Activity
	RawActivity string
	Sender      *ap.Actor
}

type deliveryTask struct {
	Job     deliveryJob
	Keys    [2]httpsig.Key
	Request *http.Request
	Inbox   string
}

type deliveryEvent struct {
	Job  deliveryJob
	Done bool
}

// Process polls the queue of outgoing activities and delivers them to other servers.
// Delivery happens in batches, with multiple workers, timeout and retries.
// The listing of additional activities and recipients runs in parallel with delivery.
// If possible, wide deliveries (e.g. public posts) are performed using the sharedInbox endpoint, greatly reducing the
// number of outgoing requests when many recipients share the same endpoint.
func (q *Queue) Process(ctx context.Context) error {
	t := time.NewTicker(q.Config.OutboxPollingInterval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil

		case <-t.C:
			if _, err := q.ProcessBatch(ctx); err != nil {
				slog.Error("Failed to deliver posts", "error", err)
			}
		}
	}
}

// ProcessBatch delivers one batch of outgoing activites in the queue.
func (q *Queue) ProcessBatch(ctx context.Context) (int, error) {
	slog.Debug("Polling delivery queue")

	rows, err := q.DB.QueryContext(
		ctx,
		`select outbox.attempts, json(outbox.activity), json(outbox.activity), json(persons.actor), persons.rsaprivkey, persons.ed25519privkey from
		outbox
		join persons
		on
			persons.id = outbox.sender
		where
			outbox.sent = 0 and
			(
				outbox.attempts = 0 or
				(
					outbox.attempts < ? and
					outbox.last <= unixepoch() - ?
				)
			)
		order by
			outbox.attempts asc,
			outbox.last asc
		limit ?`,
		q.Config.MaxDeliveryAttempts,
		q.Config.DeliveryRetryInterval,
		q.Config.DeliveryBatchSize,
	)
	if err != nil {
		return 0, fmt.Errorf("failed to fetch posts to deliver: %w", err)
	}
	defer rows.Close()

	events := make(chan deliveryEvent)
	tasks := make([]chan deliveryTask, 0, q.Config.DeliveryWorkers)
	var wg sync.WaitGroup
	results := make(chan map[deliveryJob]bool)

	// start worker routines, each with its own task queue
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
		r := make(map[deliveryJob]bool, q.Config.DeliveryBatchSize)

		for event := range events {
			r[event.Job] = event.Done
		}

		results <- r
	}()

	followers := partialFollowers{}

	count := 0
	for rows.Next() {
		var activity ap.Activity
		var rawActivity, rsaPrivKeyPem, ed25519PrivKeyPem string
		var actor ap.Actor
		var deliveryAttempts int
		if err := rows.Scan(
			&deliveryAttempts,
			&activity,
			&rawActivity,
			&actor,
			&rsaPrivKeyPem,
			&ed25519PrivKeyPem,
		); err != nil {
			slog.Error("Failed to fetch post to deliver", "error", err)
			continue
		} else if len(actor.AssertionMethod) == 0 {
			slog.Error("Actor has no Ed25519 key", "error", err)
			continue
		}

		count++

		rsaPrivKey, err := data.ParsePrivateKey(rsaPrivKeyPem)
		if err != nil {
			slog.Error("Failed to parse RSA private key", "error", err)
			continue
		}

		ed25519PrivKey, err := data.ParsePrivateKey(ed25519PrivKeyPem)
		if err != nil {
			slog.Error("Failed to parse Ed25519 private key", "error", err)
			continue
		}

		keys := [2]httpsig.Key{
			{ID: actor.PublicKey.ID, PrivateKey: rsaPrivKey},
			{ID: actor.AssertionMethod[0].ID, PrivateKey: ed25519PrivKey},
		}

		if activity.Actor == actor.ID && !q.Config.DisableIntegrityProofs {
			clone := activity

			clone.Context = []string{"https://www.w3.org/ns/activitystreams", "https://w3id.org/security/data-integrity/v1"}
			clone.Actor = ap.Gateway(q.Domain, activity.Actor)

			proof, err := proof.Create(keys[1], time.Now(), &clone, clone.Context)
			if err != nil {
				slog.Error("Failed to add integrity proof", "error", err)
				continue
			}

			clone.Proof = proof

			j, err := json.Marshal(&clone)
			if err != nil {
				slog.Error("Failed to add integrity proof", "error", err)
				continue
			}

			rawActivity = string(j)
		}

		if _, err := q.DB.ExecContext(
			ctx,
			`update outbox set last = unixepoch(), attempts = ? where id = ? and sender = ?`,
			deliveryAttempts+1,
			activity.ID,
			actor.ID,
		); err != nil {
			slog.Error("Failed to save last delivery attempt time", "id", activity.ID, "attempts", deliveryAttempts, "error", err)
			continue
		}

		job := deliveryJob{
			Activity:    &activity,
			RawActivity: rawActivity,
			Sender:      &actor,
		}

		// notify about the new job and mark it as successful until a worker notifies otherwise
		events <- deliveryEvent{job, true}

		// queue tasks for all outgoing requests while workers are busy with previous tasks
		if err := q.queueTasks(
			ctx,
			job,
			keys,
			&followers,
			tasks,
			events,
		); err != nil {
			slog.Warn("Failed to queue activity for delivery", "id", activity.ID, "attempts", deliveryAttempts, "error", err)
		}
	}

	// notify workers that no more tasks will be queued
	for _, ch := range tasks {
		close(ch)
	}

	// wait for all workers to finish their tasks
	wg.Wait()

	// stop collection of job results
	close(events)

	// receive and save job results
	for job, done := range <-results {
		if !done {
			slog.Info("Failed to deliver an activity to at least one recipient", "id", job.Activity.ID)
			continue
		}

		if _, err := q.DB.ExecContext(
			ctx,
			`update outbox set sent = 1 where id = ? and sender = ?`,
			job.Activity.ID,
			job.Sender.ID,
		); err != nil {
			slog.Error("Failed to mark delivery as completed", "id", job.Activity.ID, "error", err)
		} else {
			slog.Info("Successfully delivered an activity to all recipients", "id", job.Activity.ID)
		}
	}

	return count, nil
}

func (q *Queue) deliverWithTimeout(parent context.Context, task deliveryTask) error {
	ctx, cancel := context.WithTimeout(parent, q.Config.DeliveryTimeout)
	defer cancel()

	req := task.Request.WithContext(ctx)

	resp, err := q.Resolver.send(task.Keys, req)
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
				// if we have a duplicate task, skip without querying the deliveries table
				continue
			}
			m[task.Inbox] = struct{}{}
		} else {
			tried[task.Job.Activity.ID] = map[string]struct{}{task.Inbox: {}}
		}

		var delivered int
		if err := q.DB.QueryRowContext(
			ctx,
			`select exists (select 1 from deliveries where activity = ? and inbox = ?)`,
			task.Job.Activity.ID,
			task.Inbox,
		).Scan(&delivered); err != nil {
			slog.Error("Failed to check if delivered already", "to", task.Inbox, "activity", task.Job.Activity.ID, "error", err)
			events <- deliveryEvent{task.Job, false}
			continue
		}

		if delivered == 1 {
			slog.Info("Skipping recipient", "to", task.Inbox, "activity", task.Job.Activity.ID)
			continue
		}

		slog.Info("Delivering activity to recipient", "inbox", task.Inbox, "activity", task.Job.Activity.ID)

		if err := q.deliverWithTimeout(ctx, task); err == nil {
			slog.Info("Successfully sent an activity", "from", task.Job.Sender.ID, "to", task.Inbox, "activity", task.Job.Activity.ID)
		} else {
			slog.Warn("Failed to send an activity", "from", task.Job.Sender.ID, "to", task.Inbox, "activity", task.Job.Activity.ID, "error", err)
			if !errors.Is(err, ErrBlockedDomain) {
				events <- deliveryEvent{task.Job, false}
			}

			continue
		}

		if _, err := q.DB.ExecContext(
			ctx,
			`insert into deliveries(activity, inbox) values (?, ?)`,
			task.Job.Activity.ID,
			task.Inbox,
		); err != nil {
			slog.Error("Failed to record delivery", "activity", task.Job.Activity.ID, "inbox", task.Inbox, "error", err)
			events <- deliveryEvent{task.Job, false}
		}
	}
}

func (q *Queue) queueTasks(
	ctx context.Context,
	job deliveryJob,
	keys [2]httpsig.Key,
	followers *partialFollowers,
	tasks []chan deliveryTask,
	events chan<- deliveryEvent,
) error {
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
		followers, err := q.DB.QueryContext(
			ctx,
			`select distinct follower from follows where followed = ? and follower not like ? and accepted = 1 and not exists (select 1 from persons where persons.id = follows.follower and ed25519privkey is not null)`,
			job.Sender.ID,
			fmt.Sprintf("https://%s/%%", q.Domain),
		)
		if err != nil {
			slog.Warn("Failed to list followers", "activity", job.Activity.ID, "error", err)
		} else {
			for followers.Next() {
				var follower string
				if err := followers.Scan(&follower); err != nil {
					slog.Warn("Skipped a follower", "activity", job.Activity.ID, "error", err)
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
		author = ap.Canonicalize(obj.AttributedTo)
	}

	contentLength := strconv.Itoa(len(job.RawActivity))

	for actorID := range actorIDs.Keys() {
		if actorID == author || actorID == ap.Public {
			slog.Debug("Skipping recipient", "to", actorID, "activity", job.Activity.ID)
			continue
		}

		to, err := q.Resolver.ResolveID(ctx, keys, actorID, ap.Offline)
		if err != nil {
			slog.Warn("Failed to resolve a recipient", "to", actorID, "activity", job.Activity.ID, "error", err)
			if !errors.Is(err, ErrActorGone) && !errors.Is(err, ErrBlockedDomain) {
				events <- deliveryEvent{job, false}
			}
			continue
		}

		var inboxes []string
		// if inbox is a portable object, deliver to all gateways
		if ap.IsPortable(to.Inbox) {
			inboxes = make([]string, 0, len(to.Gateways))

			for _, gw := range to.Gateways {
				if strings.HasPrefix(gw, "https://") {
					inboxes = append(inboxes, ap.Gateway(gw[8:], to.Inbox[5:]))
				}
			}
		} else if wideDelivery {
			// if possible, use the recipient's shared inbox and skip other recipients with the same shared inbox
			if sharedInbox, ok := to.Endpoints["sharedInbox"]; ok && sharedInbox != "" {
				slog.Debug("Using shared inbox", "to", actorID, "activity", job.Activity.ID, "shared_inbox", sharedInbox)
				inboxes = []string{sharedInbox}
			}
		}

		if len(inboxes) == 0 {
			inboxes = []string{to.Inbox}
		}

		for _, inbox := range inboxes {
			req, err := http.NewRequest(http.MethodPost, inbox, strings.NewReader(job.RawActivity))
			if err != nil {
				slog.Warn("Failed to create new request", "to", actorID, "activity", job.Activity.ID, "inbox", inbox, "error", err)
				events <- deliveryEvent{job, false}
				continue
			}

			if req.URL.Host == q.Domain {
				slog.Debug("Skipping local recipient inbox", "to", actorID, "activity", job.Activity.ID, "inbox", inbox)
				continue
			}

			req.Header.Set("Accept", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)
			req.Header.Set("Content-Length", contentLength)

			if recipients.Contains(job.Sender.Followers) {
				if digest, err := followers.Digest(ctx, q.DB, q.Domain, job.Sender, req.URL.Host); err == nil {
					req.Header.Set("Collection-Synchronization", digest)
				} else {
					slog.Warn("Failed to digest followers", "to", actorID, "activity", job.Activity.ID, "inbox", inbox, "error", err)
				}
			}

			slog.Info("Queueing activity for delivery", "inbox", inbox, "activity", job.Activity.ID)

			// assign a task to a random worker but use one worker per inbox, so activities are delivered once per inbox
			tasks[crc32.ChecksumIEEE([]byte(inbox))%uint32(len(tasks))] <- deliveryTask{
				Job:     job,
				Keys:    keys,
				Request: req,
				Inbox:   inbox,
			}
		}
	}

	return nil
}
