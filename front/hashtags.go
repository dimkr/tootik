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
	"fmt"

	"github.com/dimkr/tootik/data"
	"github.com/dimkr/tootik/front/graph"
	"github.com/dimkr/tootik/front/text"
)

func printHashtags(w text.Writer, r *Request, title string, tags []string) {
	w.Text(title)
	w.Empty()

	for _, tag := range tags {
		if r.User == nil {
			w.Link("/hashtag/"+tag, "#"+tag)
		} else {
			w.Link("/users/hashtag/"+tag, "#"+tag)
		}
	}
}

func (h *Handler) hashtags(w text.Writer, r *Request, args ...string) {
	followed, err := data.CollectRowsCountIgnore[string](
		r.Context,
		h.DB,
		30,
		func(err error) bool {
			r.Log.Warn("Failed to scan hashtag", "error", err)
			return true
		},
		`
			select hashtag from (
				select hashtag, max(inserted)/86400 as last, count(distinct author) as authors, count(distinct follower) as followers, count(distinct id) as posts from (
					select hashtags.hashtag, notes.id, notes.author, notes.inserted, follows.follower from
					hashtags
					join notes
					on
						notes.id = hashtags.note
					join follows
					on
						follows.followed = notes.author
					where
						follows.accepted = 1 and
						follows.follower like ? and
						notes.inserted > unixepoch()-60*60*24*7
				)
				group by
					hashtag
			)
			where
				authors > 1 and
				followers > 1
			order by
				followers desc,
				authors desc,
				posts desc,
				last desc
			limit 30
		`,
		fmt.Sprintf("https://%s/%%", h.Domain),
	)
	if err != nil {
		r.Log.Warn("Failed to list hashtags", "error", err)
		w.Error()
		return
	}

	all, err := data.CollectRowsCountIgnore[string](
		r.Context,
		h.DB,
		30,
		func(err error) bool {
			r.Log.Warn("Failed to scan hashtag", "error", err)
			return true
		},
		`
			select hashtag from (
				select hashtag, max(inserted)/86400 as last, count(distinct author) as authors, count(*) as posts from (
					select hashtags.hashtag, notes.author, notes.inserted from
					hashtags
					join notes
					on
						notes.id = hashtags.note
					where
						inserted > unixepoch()-60*60*24*7
				)
				group by
					hashtag
			)
			where
				authors > 1
			order by
				authors desc,
				posts desc,
				last desc limit 30
		`)
	if err != nil {
		r.Log.Warn("Failed to list hashtags", "error", err)
		w.Error()
		return
	}

	labels := make([]string, 0, 7)
	values := make([]int64, 0, 7)

	if err := data.QueryScanRows(
		r.Context,
		func(row struct {
			Label string
			Value int64
		}) bool {
			labels = append(labels, row.Label)
			values = append(values, row.Value)
			return true
		},
		func(err error) bool {
			r.Log.Warn("Failed to scan hashtag", "error", err)
			return true
		},
		h.DB,
		`
		select strftime('%Y-%m-%d', datetime(day*86400, 'unixepoch')) || ' #' || hashtag, authors from (
			select notes.inserted/86400 as day, hashtags.hashtag, count(distinct notes.author) authors from
			hashtags
			join notes
			on
				notes.id = hashtags.note
			where
				inserted > (unixepoch()/86400-6)*86400
			group by
				day,
				hashtag
			order by
				day,
				authors desc
		)
		group by
			day
		order by
			day desc
		`,
	); err != nil {
		r.Log.Warn("Failed to list hashtags", "error", err)
		w.Error()
		return
	}

	w.OK()
	w.Title("🔥 Hashtags")

	if len(followed) > 0 {
		printHashtags(w, r, "Most popular hashtags used by users with at least 2 local followers, in the last week:", followed)
	}

	if len(labels) > 0 {
		if len(followed) > 0 {
			w.Empty()
		}

		w.Text("Most popular hashtag used by any user, by day:")
		w.Empty()
		w.Raw("Top daily hashtag graph", graph.Bars(labels, values))
	}

	if len(all) > 0 {
		if len(followed) > 0 || len(labels) > 0 {
			w.Empty()
		}
		printHashtags(w, r, "Most popular hashtags used by any user in the last week:", all)
	}

	if len(followed) > 0 || len(labels) > 0 || len(all) > 0 {
		w.Separator()
	}

	if r.User == nil {
		w.Link("/search", "🔎 Posts by hashtag")
	} else {
		w.Link("/users/search", "🔎 Posts by hashtag")
	}
}
