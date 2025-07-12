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

package front

import (
	"database/sql"
	"time"

	"github.com/dimkr/tootik/front/graph"
	"github.com/dimkr/tootik/front/text"
)

func (h *Handler) getGraph(r *Request, query string, keys []string, values []int64) string {
	rows, err := h.DB.QueryContext(r.Context, query)
	if err != nil {
		r.Log.Warn("Failed to data points", "query", query, "error", err)
		return ""
	}
	defer rows.Close()

	i := 0
	for rows.Next() {
		if err := rows.Scan(&keys[i], &values[i]); err != nil {
			r.Log.Warn("Failed to data point", "error", err)
			i++
			continue
		}
		i++
	}

	return graph.Bars(keys, values)
}

func (h *Handler) getDailyPostsGraph(r *Request) string {
	keys := make([]string, 24)
	values := make([]int64, 24)
	return h.getGraph(r, `select strftime('%Y-%m-%d %H:%M', datetime(inserted*60*60, 'unixepoch')), count(*) from (select inserted/(60*60) as inserted from notes where inserted>unixepoch()-60*60*24 and inserted<unixepoch()/(60*60)*60*60) group by inserted order by inserted`, keys, values)
}

func (h *Handler) getWeeklyPostsGraph(r *Request) string {
	keys := make([]string, 7)
	values := make([]int64, 7)
	return h.getGraph(r, `select strftime('%Y-%m-%d', datetime(inserted*60*60*24, 'unixepoch')), count(*) from (select inserted/(60*60*24) as inserted from notes where inserted>unixepoch()-60*60*24*7 and inserted<unixepoch()/(60*60*24)*(60*60*24)) group by inserted order by inserted`, keys, values)
}

func (h *Handler) getWeeklyFailedDeliveriesGraph(r *Request) string {
	keys := make([]string, 7)
	values := make([]int64, 7)
	return h.getGraph(r, `select strftime('%Y-%m-%d %H:%M', datetime(day*60*60*24, 'unixepoch')), count(*) from (select inserted/(60*60*24) as day from outbox where sent = 0 and inserted>unixepoch()-60*60*24*7 and inserted<unixepoch()/(60*60*24)*60*60*24) group by day order by day`, keys, values)
}

func (h *Handler) getUsersGraph(r *Request) string {
	keys := make([]string, 24)
	values := make([]int64, 24)
	return h.getGraph(r, `select strftime('%Y-%m-%d %H:%M', datetime(inserted*60*60, 'unixepoch')), count(*) from (select inserted/(60*60) as inserted from persons where inserted>unixepoch()-60*60*24 and inserted<unixepoch()/(60*60)*60*60) group by inserted order by inserted`, keys, values)
}

func (h *Handler) getKnownInstancesGraph(r *Request) string {
	keys := make([]string, 7)
	values := make([]int64, 7)
	return h.getGraph(r, `select strftime('%Y-%m-%d', datetime(days.day, 'unixepoch')), count(*) from (select host, min(inserted/(60*60*24)*60*60*24) as day from persons group by host) hosts join (select distinct inserted/(60*60*24)*60*60*24 as day from persons where inserted>unixepoch()-60*60*24*7 and inserted<unixepoch()/(60*60*24)*60*60*24) days on hosts.day < days.day group by days.day`, keys, values)
}

func (h *Handler) getActiveInstancesGraph(r *Request) string {
	keys := make([]string, 10)
	values := make([]int64, 10)
	return h.getGraph(r, `select host, (cast(round(avg(posts)) as int)) as daily from (select host, day, count(*) as posts from (select host, inserted/(60*60*24) as day from notes where inserted > unixepoch()-60*60*24*7) group by host, day) group by host order by daily desc limit 10`, keys, values)
}

func (h *Handler) getActiveUsersGraph(r *Request) string {
	keys := make([]string, 7)
	values := make([]int64, 7)
	return h.getGraph(r, `select strftime('%Y-%m-%d', datetime(day, 'unixepoch')), count(distinct author) from (select notes.inserted/(60*60*24)*60*60*24 as day, persons.id as author from notes join persons on persons.id = notes.author where notes.inserted>unixepoch()-60*60*24*7 and notes.inserted<unixepoch()/(60*60*24)*60*60*24) group by day`, keys, values)
}

func (h *Handler) getInstanceCapabilitiesGraph(r *Request) string {
	keys := make([]string, 3)
	values := make([]int64, 3)
	return h.getGraph(
		r,
		`
		select 'RFC9421+Ed25519', (select count(*) from servers where capabilities & 0x1001 = 0x1001)
		union all
		select 'RFC9421', (select count(*) from servers where capabilities & 0x1000 = 0x1000)
		union all
		select 'draft-cavage-http-signatures', (select count(*) from servers where capabilities & 0x1000 = 0)
		`,
		keys,
		values,
	)
}

func (h *Handler) status(w text.Writer, r *Request, args ...string) {
	var usersCount, postsCount, postsToday, federatedPostsCount, federatedPostsToday int64
	var lastPost, lastFederatedPost, lastRegister, lastFederatedUser sql.NullInt64
	var outboxSize, inboxSize int

	if err := h.DB.QueryRowContext(r.Context, `select count(*) from persons where host = ?`, h.Domain).Scan(&usersCount); err != nil {
		r.Log.Info("Failed to get users count", "error", err)
		w.Error()
		return
	}

	if err := h.DB.QueryRowContext(r.Context, `select count(*) from notes where host = ?`, h.Domain).Scan(&postsCount); err != nil {
		r.Log.Info("Failed to get posts count", "error", err)
		w.Error()
		return
	}

	if err := h.DB.QueryRowContext(r.Context, `select count(*) from notes where host = ? and inserted >= unixepoch() - 24*60*60`, h.Domain).Scan(&postsToday); err != nil {
		r.Log.Info("Failed to get daily posts count", "error", err)
		w.Error()
		return
	}

	if err := h.DB.QueryRowContext(r.Context, `select count(*) from notes where host != ?`, h.Domain).Scan(&federatedPostsCount); err != nil {
		r.Log.Info("Failed to get federated posts count", "error", err)
		w.Error()
		return
	}

	if err := h.DB.QueryRowContext(r.Context, `select count(*) from notes where host != ? and inserted >= unixepoch() - 24*60*60`, h.Domain).Scan(&federatedPostsToday); err != nil {
		r.Log.Info("Failed to get daily federated posts count", "error", err)
		w.Error()
		return
	}

	if err := h.DB.QueryRowContext(r.Context, `select max(inserted) from notes where host = ?`, h.Domain).Scan(&lastPost); err != nil {
		r.Log.Info("Failed to get last post time", "error", err)
		w.Error()
		return
	}

	if err := h.DB.QueryRowContext(r.Context, `select max(inserted) from notes where host != ?`, h.Domain).Scan(&lastFederatedPost); err != nil {
		r.Log.Info("Failed to get last federated post time", "error", err)
		w.Error()
		return
	}

	if err := h.DB.QueryRowContext(r.Context, `select max(inserted) from persons where host = ?`, h.Domain).Scan(&lastRegister); err != nil {
		r.Log.Info("Failed to get last post time", "error", err)
		w.Error()
		return
	}

	if err := h.DB.QueryRowContext(r.Context, `select max(max(inserted), max(updated)) from persons where host != ?`, h.Domain).Scan(&lastFederatedUser); err != nil {
		r.Log.Info("Failed to get last post time", "error", err)
		w.Error()
		return
	}

	if err := h.DB.QueryRowContext(r.Context, `select count(*) from inbox`).Scan(&inboxSize); err != nil {
		r.Log.Info("Failed to get activities queue size", "error", err)
		w.Error()
		return
	}

	if err := h.DB.QueryRowContext(r.Context, `select count(distinct activity->>'$.id') from outbox where sent = 0 and attempts < ?`, h.Config.MaxDeliveryAttempts).Scan(&outboxSize); err != nil {
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
		w.Subtitle("Instance Capabilities")
		w.Raw("Instance capabilities graph", instanceCapabilitiesGraph)
		w.Empty()
	}

	w.Subtitle("Other Statistics")
	if lastPost.Valid {
		w.Itemf("Latest local post: %s", time.Unix(lastPost.Int64, 0).Format(time.UnixDate))
	}
	if lastFederatedPost.Valid {
		w.Itemf("Latest federated post: %s", time.Unix(lastFederatedPost.Int64, 0).Format(time.UnixDate))
	}
	w.Itemf("Local posts today: %d", postsToday)
	w.Itemf("Federated posts today: %d", federatedPostsToday)
	w.Itemf("Local users: %d", usersCount)
	w.Itemf("Local posts: %d", postsCount)
	w.Itemf("Federated posts: %d", federatedPostsCount)
	if lastRegister.Valid {
		w.Itemf("Newest user: %s", time.Unix(lastRegister.Int64, 0).Format(time.UnixDate))
	}
	if lastFederatedUser.Valid {
		w.Itemf("Latest federated user update: %s", time.Unix(lastFederatedUser.Int64, 0).Format(time.UnixDate))
	}
	w.Itemf("Incoming activities queue size: %d", inboxSize)
	w.Itemf("Outgoing activities queue size: %d", outboxSize)
}
