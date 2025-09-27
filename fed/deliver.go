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
	"errors"
	"fmt"
	"hash/crc32"
	"log/slog"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/data"
	"github.com/dimkr/tootik/httpsig"
	"github.com/dimkr/tootik/logcontext"
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
// During wide deliveries (e.g. public posts), additional requests to the same server are skipped, greatly reducing the
// number of outgoing requests when many recipients reside on the same server.
func (q *Queue) Process(ctx context.Context) error {
	t := time.NewTicker(q.Config.OutboxPollingInterval)
	defer t.Stop()

	for {
		select {
		case <-ctx.Done():
			return nil

		case <-t.C:
			if _, err := q.ProcessBatch(ctx); err != nil {
				slog.ErrorContext(ctx, "Failed to deliver posts", "error", err)
			}
		}
	}
}

// ProcessBatch delivers one batch of outgoing activites in the queue.
func (q *Queue) ProcessBatch(ctx context.Context) (int, error) {
	slog.DebugContext(ctx, "Polling delivery queue")

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
		var rawActivity, rsaPrivKeyPem, ed25519PrivKeyMultibase string
		var actor ap.Actor
		var deliveryAttempts int
		if err := rows.Scan(
			&deliveryAttempts,
			&activity,
			&rawActivity,
			&actor,
			&rsaPrivKeyPem,
			&ed25519PrivKeyMultibase,
		); err != nil {
			slog.ErrorContext(ctx, "Failed to fetch post to deliver", "error", err)
			continue
		} else if len(actor.AssertionMethod) == 0 {
			slog.ErrorContext(ctx, "Actor has no Ed25519 key", "error", err)
			continue
		}

		count++

		rsaPrivKey, err := data.ParseRSAPrivateKey(rsaPrivKeyPem)
		if err != nil {
			slog.ErrorContext(ctx, "Failed to parse RSA private key", "error", err)
			continue
		}

		ed25519PrivKey, err := data.DecodeEd25519PrivateKey(ed25519PrivKeyMultibase)
		if err != nil {
			slog.ErrorContext(ctx, "Failed to decode Ed25519 private key", "error", err)
			continue
		}

		keys := [2]httpsig.Key{
			{ID: actor.PublicKey.ID, PrivateKey: rsaPrivKey},
			{ID: actor.AssertionMethod[0].ID, PrivateKey: ed25519PrivKey},
		}

		if _, err := q.DB.ExecContext(
			ctx,
			`update outbox set last = unixepoch(), attempts = ? where cid = ? and sender = ?`,
			deliveryAttempts+1,
			ap.Canonical(activity.ID),
			actor.ID,
		); err != nil {
			slog.ErrorContext(ctx, "Failed to save last delivery attempt time", "id", activity.ID, "attempts", deliveryAttempts, "error", err)
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
			slog.WarnContext(ctx, "Failed to queue activity for delivery", "id", activity.ID, "attempts", deliveryAttempts, "error", err)
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
			slog.InfoContext(ctx, "Failed to deliver an activity to at least one recipient", "id", job.Activity.ID)
			continue
		}

		if _, err := q.DB.ExecContext(
			ctx,
			`update outbox set sent = 1 where cid = ? and sender = ?`,
			ap.Canonical(job.Activity.ID),
			job.Sender.ID,
		); err != nil {
			slog.ErrorContext(ctx, "Failed to mark delivery as completed", "id", job.Activity.ID, "error", err)
		} else {
			slog.InfoContext(ctx, "Successfully delivered an activity to all recipients", "id", job.Activity.ID)
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

func (q *Queue) consume(parent context.Context, requests <-chan deliveryTask, events chan<- deliveryEvent) {
	tried := map[string]map[string]struct{}{}

	for task := range requests {
		if m, ok := tried[task.Job.Activity.ID]; ok {
			if _, ok := m[task.Request.Host]; ok {
				// if we have a duplicate task, skip without querying the deliveries table
				continue
			}
			m[task.Request.Host] = struct{}{}
		} else {
			tried[task.Job.Activity.ID] = map[string]struct{}{task.Request.Host: {}}
		}

		ctx := logcontext.Add(parent, slog.Group("delivery", "sender", task.Job.Sender.ID, "inbox", task.Inbox, "activity", task.Job.Activity.ID))

		var delivered int
		if err := q.DB.QueryRowContext(
			ctx,
			`select exists (select 1 from deliveries where activity = ? and host = ?)`,
			ap.Canonical(task.Job.Activity.ID),
			task.Request.Host,
		).Scan(&delivered); err != nil {
			slog.ErrorContext(ctx, "Failed to check if delivered already", "error", err)
			events <- deliveryEvent{task.Job, false}
			continue
		}

		if delivered == 1 {
			slog.InfoContext(ctx, "Skipping recipient")
			continue
		}

		slog.InfoContext(ctx, "Delivering activity to recipient")

		if err := q.deliverWithTimeout(ctx, task); err == nil {
			slog.InfoContext(ctx, "Successfully sent an activity")
		} else {
			slog.WarnContext(ctx, "Failed to send an activity", "error", err)
			if !errors.Is(err, ErrBlockedDomain) {
				events <- deliveryEvent{task.Job, false}
			}

			continue
		}

		if _, err := q.DB.ExecContext(
			ctx,
			`insert into deliveries(activity, host) values (?, ?)`,
			ap.Canonical(task.Job.Activity.ID),
			task.Request.Host,
		); err != nil {
			slog.ErrorContext(ctx, "Failed to record delivery", "to", task.Inbox, "error", err)
			events <- deliveryEvent{task.Job, false}
		}
	}
}

func (q *Queue) queueTask(
	ctx context.Context,
	job deliveryJob,
	keys [2]httpsig.Key,
	inbox, contentLength string,
	followers *partialFollowers,
	tasks []chan deliveryTask,
	events chan<- deliveryEvent,
) {
	ctx = logcontext.Add(ctx, slog.Group("task", "activity", job.Activity.ID, "inbox", inbox))

	req, err := http.NewRequest(http.MethodPost, inbox, strings.NewReader(job.RawActivity))
	if err != nil {
		slog.WarnContext(ctx, "Failed to create new request", "error", err)
		events <- deliveryEvent{job, false}
		return
	}

	if req.URL.Host == q.Domain {
		slog.DebugContext(ctx, "Skipping local recipient inbox")
		return
	}

	req.Header.Set("Content-Type", `application/ld+json; profile="https://www.w3.org/ns/activitystreams"`)
	req.Header.Set("Content-Length", contentLength)

	if followers != nil {
		if digest, err := followers.Digest(ctx, q.DB, q.Domain, job.Sender, req.URL.Host); err == nil {
			req.Header.Set("Collection-Synchronization", digest)
		} else {
			slog.WarnContext(ctx, "Failed to digest followers", "error", err)
		}
	}

	slog.InfoContext(ctx, "Queueing activity for delivery")

	// assign a task to a random worker but use one worker per inbox, so activities are delivered once per inbox
	tasks[crc32.ChecksumIEEE([]byte(inbox))%uint32(len(tasks))] <- deliveryTask{
		Job:     job,
		Keys:    keys,
		Request: req,
		Inbox:   inbox,
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
	activityID, err := url.Parse(job.Activity.ID)
	if err != nil {
		return err
	}

	ctx = logcontext.Add(ctx, slog.Group("job", "activity", job.Activity.ID, "sender", job.Sender.ID))

	recipients := ap.Audience{}

	// deduplicate recipients or skip if we're forwarding an activity
	if ap.Canonical(job.Activity.Actor) == ap.Canonical(job.Sender.ID) {
		for id := range job.Activity.To.Keys() {
			recipients.Add(id)
		}

		for id := range job.Activity.CC.Keys() {
			recipients.Add(id)
		}
	}

	wideDelivery := recipients.Contains(job.Sender.Followers)

	// disable followers synchronization if not sending to followers
	if !wideDelivery {
		followers = nil

		wideDelivery = ap.Canonical(job.Activity.Actor) != ap.Canonical(job.Sender.ID) || job.Activity.IsPublic()
	}

	contentLength := strconv.Itoa(len(job.RawActivity))

	// list the actor's federated followers if we're forwarding an activity by another actor, or if addressed by actor
	if wideDelivery {
		inboxes, err := q.DB.QueryContext(
			ctx,
			`select distinct coalesce(persons.actor->>'$.endpoints.sharedInbox', persons.actor->>'$.inbox') from persons join follows on follows.follower = persons.id where follows.followed = ? and follows.accepted = 1 and follows.follower not like ? and persons.ed25519privkey is null`,
			job.Sender.ID,
			fmt.Sprintf("https://%s/%%", activityID.Host),
		)
		if err != nil {
			slog.WarnContext(ctx, "Failed to list followers", "error", err)
		} else {
			for inboxes.Next() {
				var inbox string
				if err := inboxes.Scan(&inbox); err != nil {
					slog.WarnContext(ctx, "Skipped an inbox", "error", err)
					continue
				}

				q.queueTask(
					ctx,
					job,
					keys,
					inbox,
					contentLength,
					followers,
					tasks,
					events,
				)
			}

			inboxes.Close()
		}
	}

	var author string
	if obj, ok := job.Activity.Object.(*ap.Object); ok {
		author = ap.Canonical(obj.AttributedTo)
	}

	// assume that all other federated recipients are actors and not collections
	for actorID := range recipients.Keys() {
		if ap.Canonical(actorID) == author || actorID == ap.Public || actorID == job.Sender.Followers {
			slog.DebugContext(ctx, "Skipping recipient", "to", actorID)
			continue
		}

		to, err := q.Resolver.ResolveID(ctx, keys, actorID, ap.Offline)
		if err != nil {
			slog.WarnContext(ctx, "Failed to resolve a recipient", "to", actorID, "error", err)
			if !errors.Is(err, ErrActorGone) && !errors.Is(err, ErrBlockedDomain) {
				events <- deliveryEvent{job, false}
			}
			continue
		}

		// if possible, use the recipient's shared inbox
		inbox := to.Inbox
		if wideDelivery {
			if sharedInbox, ok := to.Endpoints["sharedInbox"]; ok && sharedInbox != "" {
				slog.DebugContext(ctx, "Using shared inbox", "to", actorID, "shared_inbox", inbox)
				inbox = sharedInbox
			}
		}

		q.queueTask(
			ctx,
			job,
			keys,
			inbox,
			contentLength,
			followers,
			tasks,
			events,
		)
	}

	// if this is an activity by a portable actor, forward it to all gateways
	if ap.IsPortable(job.Sender.ID) && len(job.Sender.Gateways) > 1 {
		for _, gw := range job.Sender.Gateways[1:] {
			slog.InfoContext(ctx, "Forwarding activity to gateway", "gateway", gw)

			q.queueTask(
				ctx,
				job,
				keys,
				ap.Gateway(gw, job.Sender.Inbox),
				contentLength,
				followers,
				tasks,
				events,
			)
		}
	}

	return nil
}
