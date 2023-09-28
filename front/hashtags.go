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
	"github.com/dimkr/tootik/text"
)

func hashtags(w text.Writer, r *request) {
	rows, err := r.Query(`select hashtag from (select hashtag, max(inserted)/86400 as last, count(distinct author) as users, count(*) as posts from (select hashtags.hashtag, notes.author, notes.inserted from hashtags join notes on notes.id = hashtags.note where inserted > unixepoch()-60*60*24*7) group by hashtag) where users > 1 order by users desc, posts desc, last desc limit 100`)
	if err != nil {
		r.Log.Warn("Failed to list hashtags", "error", err)
		w.Error()
		return
	}

	var tags []string

	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			r.Log.Warn("Failed to scan hashtag", "error", err)
			continue
		}

		tags = append(tags, tag)
	}
	rows.Close()

	w.OK()
	w.Title("🔥 Hashtags")

	if len(tags) > 0 {
		w.Text("Most popular hashtags in the last week:")
		w.Empty()

		for _, tag := range tags {
			if r.User == nil {
				w.Link("/hashtag/"+tag, "#"+tag)
			} else {
				w.Link("/users/hashtag/"+tag, "#"+tag)
			}
		}

		w.Separator()
	}

	if r.User == nil {
		w.Link("/search", "🔎 Posts by hashtag")
	} else {
		w.Link("/users/search", "🔎 Posts by hashtag")
	}
}
