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
	"errors"
	"time"

	"github.com/dimkr/tootik/dbx"
	"github.com/dimkr/tootik/front/graph"
	"github.com/dimkr/tootik/front/text"
	"github.com/dimkr/tootik/lake"
)

func (h *Handler) getGraph(r *Request, name string, size int) string {
	keys := make([]string, size)
	values := make([]int64, size)

	if err := dbx.QueryScan(
		r.Context,
		func(row struct {
			Position int
			Label    string
			Value    int64
		}) {
			if row.Position < size {
				keys[row.Position] = row.Label
				values[row.Position] = row.Value
			}
		},
		func(err error) bool {
			return false
		},
		h.DB,
		`select position, label, value from statusgraphs where graph = $1 order by position`,
		name,
	); err != nil {
		r.Log.Warn("Failed to query data points", "graph", name, "error", err)
		return ""
	}

	return graph.Bars(keys, values)
}

func (h *Handler) status(w text.Writer, r *Request, args ...string) {
	var usersCount, postsCount, postsToday, federatedPostsCount, federatedPostsToday int64
	var lastPost, lastFederatedPost, lastRegister, lastFederatedUser, exported sql.NullTime
	var exportDuration sql.NullFloat64
	var exportedRows sql.NullInt64
	var outboxSize, inboxSize int

	computed := true
	if err := h.DB.QueryRowContext(
		r.Context,
		`select users, posts, poststoday, federatedposts, federatedpoststoday, lastpost, lastfederatedpost, lastregister, lastfederateduser, exported, extract(epoch from exportduration)::float8, exportedrows from status`,
	).Scan(
		&usersCount,
		&postsCount,
		&postsToday,
		&federatedPostsCount,
		&federatedPostsToday,
		&lastPost,
		&lastFederatedPost,
		&lastRegister,
		&lastFederatedUser,
		&exported,
		&exportDuration,
		&exportedRows,
	); errors.Is(err, sql.ErrNoRows) {
		computed = false
	} else if err != nil {
		r.Log.Info("Failed to get statistics", "error", err)
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

	var dailyPostsGraph, weeklyPostsGraph, weeklyFailedDeliveriesGraph, usersGraph, activeUsersGraph, knownInstancesGraph, activeInstancesGraph, instanceCapabilitiesGraph string
	if computed {
		dailyPostsGraph = h.getGraph(r, lake.GraphDailyPosts, 24)
		weeklyPostsGraph = h.getGraph(r, lake.GraphWeeklyPosts, 7)
		weeklyFailedDeliveriesGraph = h.getGraph(r, lake.GraphWeeklyFailedDeliveries, 7)
		usersGraph = h.getGraph(r, lake.GraphUsers, 24)
		activeUsersGraph = h.getGraph(r, lake.GraphActiveUsers, 7)
		knownInstancesGraph = h.getGraph(r, lake.GraphKnownInstances, 7)
		activeInstancesGraph = h.getGraph(r, lake.GraphActiveInstances, 10)
		instanceCapabilitiesGraph = h.getGraph(r, lake.GraphInstanceCapabilities, 7)
	}

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
	if computed {
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
		if exported.Valid && exportDuration.Valid && exportedRows.Valid {
			w.Itemf("Latest Iceberg export: %s", exported.Time.Format(time.UnixDate))
			w.Itemf("Latest Iceberg export duration: %s", (time.Duration(exportDuration.Float64 * float64(time.Second))).Round(time.Millisecond))
			w.Itemf("Latest Iceberg export changed rows: %d", exportedRows.Int64)
		}
	} else {
		w.Item("Statistics are not available yet")
	}
	w.Itemf("Incoming activities queue size: %d", inboxSize)
	w.Itemf("Outgoing activities queue size: %d", outboxSize)
}
