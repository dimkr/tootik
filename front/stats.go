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

package front

import (
	"fmt"
	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/graph"
	"github.com/dimkr/tootik/text"
	"regexp"
	"time"
)

func init() {
	handlers[regexp.MustCompile(`^/stats$`)] = withCache(withUserMenu(stats), time.Minute*5)
	handlers[regexp.MustCompile(`^/users/stats$`)] = withCache(withUserMenu(stats), time.Minute*5)
}

func getGraph(r *request, query string, keys []string, values []int64) string {
	rows, err := r.Query(query)
	if err != nil {
		r.Log.WithField("query", query).WithError(err).Warn("Failed to data points")
		return ""
	}
	defer rows.Close()

	i := 0
	for rows.Next() {
		if err := rows.Scan(&keys[i], &values[i]); err != nil {
			r.Log.WithError(err).Warn("Failed to data point")
			i++
			continue
		}
		i++
	}

	return graph.Bars(keys, values)
}

func getDailyPostsGraph(r *request) string {
	keys := make([]string, 24)
	values := make([]int64, 24)
	return getGraph(r, `select strftime('%Y-%m-%d %H:%M', datetime(inserted*60*60, 'unixepoch')), count(*) from (select inserted/(60*60) as inserted from notes where inserted>unixepoch()-60*60*24 and inserted<unixepoch()/(60*60)*60*60) group by inserted order by inserted`, keys, values)
}

func getWeeklyPostsGraph(r *request) string {
	keys := make([]string, 7)
	values := make([]int64, 7)
	return getGraph(r, `select strftime('%Y-%m-%d', datetime(inserted*60*60*24, 'unixepoch')), count(*) from (select inserted/(60*60*24) as inserted from notes where inserted>unixepoch()-60*60*24*7 and inserted<unixepoch()/(60*60*24)*(60*60*24)) group by inserted order by inserted`, keys, values)
}

func getUsersGraph(r *request) string {
	keys := make([]string, 24)
	values := make([]int64, 24)
	return getGraph(r, `select strftime('%Y-%m-%d %H:%M', datetime(inserted*60*60, 'unixepoch')), count(*) from (select inserted/(60*60) as inserted from persons where inserted>unixepoch()-60*60*24 and inserted<unixepoch()/(60*60)*60*60) group by inserted order by inserted`, keys, values)
}

func stats(w text.Writer, r *request) {
	prefix := fmt.Sprintf("https://%s/%%", cfg.Domain)

	var usersCount, postsCount, postsToday, federatedPostsCount, federatedPostsToday, lastPost, lastFederatedPost, lastRegister, lastFederatedUser int64
	var deliveriesQueueSize, activitiesQueueSize int

	if err := r.QueryRow(`select count(*) from persons where id like ?`, prefix).Scan(&usersCount); err != nil {
		r.Log.WithError(err).Info("Failed to get users count")
		w.Error()
		return
	}

	if err := r.QueryRow(`select count(*) from notes where id like ?`, prefix).Scan(&postsCount); err != nil {
		r.Log.WithError(err).Info("Failed to get posts count")
		w.Error()
		return
	}

	if err := r.QueryRow(`select count(*) from notes where id like ? and inserted >= unixepoch() - 24*60*60`, prefix).Scan(&postsToday); err != nil {
		r.Log.WithError(err).Info("Failed to get daily posts count")
		w.Error()
		return
	}

	if err := r.QueryRow(`select count(*) from notes where id not like ?`, prefix).Scan(&federatedPostsCount); err != nil {
		r.Log.WithError(err).Info("Failed to get federated posts count")
		w.Error()
		return
	}

	if err := r.QueryRow(`select count(*) from notes where id not like ? and inserted >= unixepoch() - 24*60*60`, prefix).Scan(&federatedPostsToday); err != nil {
		r.Log.WithError(err).Info("Failed to get daily federated posts count")
		w.Error()
		return
	}

	if err := r.QueryRow(`select max(inserted) from notes where id like ?`, prefix).Scan(&lastPost); err != nil {
		r.Log.WithError(err).Info("Failed to get last post time")
		w.Error()
		return
	}

	if err := r.QueryRow(`select max(inserted) from notes where id not like ?`, prefix).Scan(&lastFederatedPost); err != nil {
		r.Log.WithError(err).Info("Failed to get last federated post time")
		w.Error()
		return
	}

	if err := r.QueryRow(`select max(inserted) from persons where id like ?`, prefix).Scan(&lastRegister); err != nil {
		r.Log.WithError(err).Info("Failed to get last post time")
		w.Error()
		return
	}

	if err := r.QueryRow(`select max(max(inserted), max(updated)) from persons where id not like ?`, prefix).Scan(&lastFederatedUser); err != nil {
		r.Log.WithError(err).Info("Failed to get last post time")
		w.Error()
		return
	}

	if err := r.QueryRow(`select count(*) from activities`).Scan(&activitiesQueueSize); err != nil {
		r.Log.WithError(err).Info("Failed to get activities queue size")
		w.Error()
		return
	}

	if err := r.QueryRow(`select count(*) from deliveries`).Scan(&deliveriesQueueSize); err != nil {
		r.Log.WithError(err).Info("Failed to get delivery queue size")
		w.Error()
		return
	}

	dailyPostsGraph := getDailyPostsGraph(r)
	weeklyPostsGraph := getWeeklyPostsGraph(r)
	usersGraph := getUsersGraph(r)

	w.OK()

	w.Title("📊 Statistics")

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

	if usersGraph != "" {
		w.Subtitle("New Federated Users Per Hour")
		w.Raw("Users graph", usersGraph)
		w.Empty()
	}

	w.Subtitle("Other Statistics")
	w.Itemf("Latest local post: %s", time.Unix(lastPost, 0).Format(time.UnixDate))
	w.Itemf("Latest federated post: %s", time.Unix(lastFederatedPost, 0).Format(time.UnixDate))
	w.Itemf("Local posts today: %d", postsToday)
	w.Itemf("Federated posts today: %d", federatedPostsToday)
	w.Itemf("Local users: %d", usersCount)
	w.Itemf("Local posts: %d", postsCount)
	w.Itemf("Federated posts: %d", federatedPostsCount)
	w.Itemf("Newest user: %s", time.Unix(lastRegister, 0).Format(time.UnixDate))
	w.Itemf("Latest federated user update: %s", time.Unix(lastFederatedUser, 0).Format(time.UnixDate))
	w.Itemf("Incoming posts queue size: %d", activitiesQueueSize)
	w.Itemf("Outgoing posts queue size: %d", deliveriesQueueSize)
}
