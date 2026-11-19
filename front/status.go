/*
Copyright 2023 - 2026 Dima Krasner

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

package front

import (
	"database/sql"
	"time"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/dbx"
	"github.com/dimkr/tootik/front/graph"
	"github.com/dimkr/tootik/front/text"
)

func (h *Handler) getGraph(r *Request, keys []string, values []int64, query string, args ...any) string {
	i := 0
	if err := dbx.QueryScan(
		r.Context,
		func(row struct {
			Key   string
			Value int64
		}) {
			keys[i] = row.Key
			values[i] = row.Value
			i++
		},
		func(err error) bool {
			return false
		},
		h.DB,
		query,
		args...,
	); err != nil {
		r.Log.Warn("Failed to query data points", "error", err)
		return ""
	}

	return graph.Bars(keys, values)
}

func (h *Handler) getDailyPostsGraph(r *Request) string {
	keys := make([]string, 24)
	values := make([]int64, 24)
	return h.getGraph(r, keys, values, `select to_char(hour at time zone 'UTC', 'YYYY-MM-DD HH24:MI'), count(*) from (select date_trunc('hour', inserted, 'UTC') as hour from notes where inserted > now() - interval '1 day' and inserted < date_trunc('hour', now(), 'UTC')) hours group by hour order by hour`)
}

func (h *Handler) getWeeklyPostsGraph(r *Request) string {
	keys := make([]string, 7)
	values := make([]int64, 7)
	return h.getGraph(r, keys, values, `select to_char(day at time zone 'UTC', 'YYYY-MM-DD'), count(*) from (select date_trunc('day', inserted, 'UTC') as day from notes where inserted > now() - interval '7 days' and inserted < date_trunc('day', now(), 'UTC')) days group by day order by day`)
}

func (h *Handler) getWeeklyFailedDeliveriesGraph(r *Request) string {
	keys := make([]string, 7)
	values := make([]int64, 7)
	return h.getGraph(r, keys, values, `select to_char(day at time zone 'UTC', 'YYYY-MM-DD HH24:MI'), count(*) from (select date_trunc('day', inserted, 'UTC') as day from outbox where not sent and inserted > now() - interval '7 days' and inserted < date_trunc('day', now(), 'UTC')) days group by day order by day`)
}

func (h *Handler) getUsersGraph(r *Request) string {
	keys := make([]string, 24)
	values := make([]int64, 24)
	return h.getGraph(r, keys, values, `select to_char(hour at time zone 'UTC', 'YYYY-MM-DD HH24:MI'), count(*) from (select date_trunc('hour', inserted, 'UTC') as hour from persons where inserted > now() - interval '1 day' and inserted < date_trunc('hour', now(), 'UTC')) hours group by hour order by hour`)
}

func (h *Handler) getKnownInstancesGraph(r *Request) string {
	keys := make([]string, 7)
	values := make([]int64, 7)
	return h.getGraph(r, keys, values, `select to_char(days.day at time zone 'UTC', 'YYYY-MM-DD'), count(*) from (select host, min(date_trunc('day', inserted, 'UTC')) as day from persons group by host) hosts join (select distinct date_trunc('day', inserted, 'UTC') as day from persons where inserted > now() - interval '7 days' and inserted < date_trunc('day', now(), 'UTC')) days on hosts.day < days.day group by days.day order by days.day`)
}

func (h *Handler) getActiveInstancesGraph(r *Request) string {
	keys := make([]string, 10)
	values := make([]int64, 10)
	return h.getGraph(r, keys, values, `select host, (cast(round(avg(posts)) as int)) as daily from (select host, day, count(*) as posts from (select host, date_trunc('day', inserted, 'UTC') as day from notes where inserted > now() - interval '7 days') days group by host, day) posts group by host order by daily desc limit 10`)
}

func (h *Handler) getActiveUsersGraph(r *Request) string {
	keys := make([]string, 7)
	values := make([]int64, 7)
	return h.getGraph(r, keys, values, `select to_char(day at time zone 'UTC', 'YYYY-MM-DD'), count(distinct author) from (select date_trunc('day', notes.inserted, 'UTC') as day, persons.id as author from notes join persons on persons.id = notes.author where notes.inserted > now() - interval '7 days' and notes.inserted < date_trunc('day', now(), 'UTC')) days group by day order by day`)
}

func (h *Handler) getInstanceCapabilitiesGraph(r *Request) string {
	keys := make([]string, 7)
	values := make([]int64, 7)
	return h.getGraph(
		r,
		keys,
		values,
		`
		select 'RFC9421 with ML-DSA-44', (select count(*) from servers where capabilities & $1 > 0)
		union all
		select 'RFC9421 with Ed25519', (select count(*) from servers where capabilities & $2 > 0)
		union all
		select 'RFC9421 with RSA but without Ed25519 or ML-DSA-44', (select count(*) from servers where capabilities & ($1 | $2 | $3) = $3)
		union all
		select 'RFC9421 without draft-cavage-http-signatures', (select count(*) from servers where capabilities & $4 = 0 and capabilities & ($1 | $2 | $3) > 0)
		union all
		select 'draft-cavage-http-signatures without RFC9421', (select count(*) from servers where capabilities & $4 > 0 and capabilities & ($1 | $2 | $3) = 0)
		union all
		select 'draft-cavage-http-signatures', (select count(*) from servers where capabilities & $4 > 0)
		union all
		select 'Total', (select count(*) from servers)
		`,
		ap.RFC9421MLDSA44Signatures,
		ap.RFC9421Ed25519Signatures,
		ap.RFC9421RSASignatures,
		ap.CavageDraftSignatures,
	)
}

func (h *Handler) status(w text.Writer, r *Request, args ...string) {
	var usersCount, postsCount, postsToday, federatedPostsCount, federatedPostsToday int64
	var lastPost, lastFederatedPost, lastRegister, lastFederatedUser sql.NullTime
	var outboxSize, inboxSize int

	if err := h.DB.QueryRowContext(r.Context, `select count(*) from persons where host = $1`, h.Domain).Scan(&usersCount); err != nil {
		r.Log.Info("Failed to get users count", "error", err)
		w.Error()
		return
	}

	if err := h.DB.QueryRowContext(r.Context, `select count(*) from notes where host = $1`, h.Domain).Scan(&postsCount); err != nil {
		r.Log.Info("Failed to get posts count", "error", err)
		w.Error()
		return
	}

	if err := h.DB.QueryRowContext(r.Context, `select count(*) from notes where host = $1 and inserted >= now() - interval '1 day'`, h.Domain).Scan(&postsToday); err != nil {
		r.Log.Info("Failed to get daily posts count", "error", err)
		w.Error()
		return
	}

	if err := h.DB.QueryRowContext(r.Context, `select count(*) from notes where host != $1`, h.Domain).Scan(&federatedPostsCount); err != nil {
		r.Log.Info("Failed to get federated posts count", "error", err)
		w.Error()
		return
	}

	if err := h.DB.QueryRowContext(r.Context, `select count(*) from notes where host != $1 and inserted >= now() - interval '1 day'`, h.Domain).Scan(&federatedPostsToday); err != nil {
		r.Log.Info("Failed to get daily federated posts count", "error", err)
		w.Error()
		return
	}

	if err := h.DB.QueryRowContext(r.Context, `select max(inserted) from notes where host = $1`, h.Domain).Scan(&lastPost); err != nil {
		r.Log.Info("Failed to get last post time", "error", err)
		w.Error()
		return
	}

	if err := h.DB.QueryRowContext(r.Context, `select max(inserted) from notes where host != $1`, h.Domain).Scan(&lastFederatedPost); err != nil {
		r.Log.Info("Failed to get last federated post time", "error", err)
		w.Error()
		return
	}

	if err := h.DB.QueryRowContext(r.Context, `select max(inserted) from persons where host = $1`, h.Domain).Scan(&lastRegister); err != nil {
		r.Log.Info("Failed to get last post time", "error", err)
		w.Error()
		return
	}

	if err := h.DB.QueryRowContext(r.Context, `select greatest(max(inserted), max(updated)) from persons where host != $1`, h.Domain).Scan(&lastFederatedUser); err != nil {
		r.Log.Info("Failed to get last post time", "error", err)
		w.Error()
		return
	}

	if err := h.DB.QueryRowContext(r.Context, `select count(*) from inbox`).Scan(&inboxSize); err != nil {
		r.Log.Info("Failed to get activities queue size", "error", err)
		w.Error()
		return
	}

	if err := h.DB.QueryRowContext(r.Context, `select count(distinct cid) from outbox where not sent and attempts < $1`, h.Config.MaxDeliveryAttempts).Scan(&outboxSize); err != nil {
		r.Log.Info("Failed to get delivery queue size", "error", err)
		w.Error()
		return
	}

	dailyPostsGraph := h.getDailyPostsGraph(r)
	weeklyPostsGraph := h.getWeeklyPostsGraph(r)
	weeklyFailedDeliveriesGraph := h.getWeeklyFailedDeliveriesGraph(r)
	usersGraph := h.getUsersGraph(r)
	activeUsersGraph := h.getActiveUsersGraph(r)
	knownInstancesGraph := h.getKnownInstancesGraph(r)
	activeInstancesGraph := h.getActiveInstancesGraph(r)
	instanceCapabilitiesGraph := h.getInstanceCapabilitiesGraph(r)

	w.OK()

	w.Title("📊 Status")

	if dailyPostsGraph != "" {
		w.Subtitle("Posts Per Hour")
		w.Raw("Daily posts graph", dailyPostsGraph)
		w.Empty()
	}

	if weeklyPostsGraph != "" {
		w.Subtitle("Posts Per Day")
		w.Raw("Weekly posts graph", weeklyPostsGraph)
		w.Empty()
	}

	if weeklyFailedDeliveriesGraph != "" {
		w.Subtitle("Failed Outgoing Post Deliveries Per Day")
		w.Raw("Failed outgoing post deliveries graph", weeklyFailedDeliveriesGraph)
		w.Empty()
	}

	if usersGraph != "" {
		w.Subtitle("New Federated Users Per Hour")
		w.Raw("Users graph", usersGraph)
		w.Empty()
	}

	if activeUsersGraph != "" {
		w.Subtitle("Active Users Per Day")
		w.Raw("Weekly active users graph", activeUsersGraph)
		w.Empty()
	}

	if knownInstancesGraph != "" {
		w.Subtitle("Connected Instances Per Day")
		w.Raw("Weekly connected instances graph", knownInstancesGraph)
		w.Empty()
	}

	if activeInstancesGraph != "" {
		w.Subtitle("Average Daily Posts By Instance")
		w.Raw("Average daily posts graph", activeInstancesGraph)
		w.Empty()
	}

	if instanceCapabilitiesGraph != "" {
		w.Subtitle("Discovered Instance Capabilities")
		w.Raw("Discovered instance capabilities graph", instanceCapabilitiesGraph)
		w.Empty()
	}

	w.Subtitle("Other Statistics")
	if lastPost.Valid {
		w.Itemf("Latest local post: %s", lastPost.Time.Format(time.UnixDate))
	}
	if lastFederatedPost.Valid {
		w.Itemf("Latest federated post: %s", lastFederatedPost.Time.Format(time.UnixDate))
	}
	w.Itemf("Local posts today: %d", postsToday)
	w.Itemf("Federated posts today: %d", federatedPostsToday)
	w.Itemf("Local users: %d", usersCount)
	w.Itemf("Local posts: %d", postsCount)
	w.Itemf("Federated posts: %d", federatedPostsCount)
	if lastRegister.Valid {
		w.Itemf("Newest user: %s", lastRegister.Time.Format(time.UnixDate))
	}
	if lastFederatedUser.Valid {
		w.Itemf("Latest federated user update: %s", lastFederatedUser.Time.Format(time.UnixDate))
	}
	w.Itemf("Incoming activities queue size: %d", inboxSize)
	w.Itemf("Outgoing activities queue size: %d", outboxSize)
}
