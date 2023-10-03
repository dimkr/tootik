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
	"database/sql"
	"fmt"
	"github.com/dimkr/tootik/cfg"
	"github.com/dimkr/tootik/front/text"
)

func scanHashtags(r *request, rows *sql.Rows) []string {
	tags := make([]string, 0, 30)

	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			r.Log.Warn("Failed to scan hashtag", "error", err)
			continue
		}

		tags = append(tags, tag)
	}

	return tags
}

func printHashtags(w text.Writer, r *request, title string, tags []string) {
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

func hashtags(w text.Writer, r *request) {
	rows, err := r.Query(`select hashtag from (select hashtag, max(inserted)/86400 as last, count(distinct author) as users, count(*) as posts from (select hashtags.hashtag, notes.author, notes.inserted from hashtags join notes on notes.id = hashtags.note join follows on follows.followed = notes.author where follows.accepted = 1 and follows.follower like ? and notes.inserted > unixepoch()-60*60*24*7) group by hashtag) where users > 1 order by users desc, posts desc, last desc limit 30`, fmt.Sprintf("https://%s/%%", cfg.Domain))
	if err != nil {
		r.Log.Warn("Failed to list hashtags", "error", err)
		w.Error()
		return
	}

	followed := scanHashtags(r, rows)
	rows.Close()

	rows, err = r.Query(`select hashtag from (select hashtag, max(inserted)/86400 as last, count(distinct author) as users, count(*) as posts from (select hashtags.hashtag, notes.author, notes.inserted from hashtags join notes on notes.id = hashtags.note where inserted > unixepoch()-60*60*24*7) group by hashtag) where users > 1 order by users desc, posts desc, last desc limit 30`)
	if err != nil {
		r.Log.Warn("Failed to list hashtags", "error", err)
		w.Error()
		return
	}

	all := scanHashtags(r, rows)
	rows.Close()

	w.OK()
	w.Title("🔥 Hashtags")

	if len(followed) > 0 {
		printHashtags(w, r, "Most popular hashtags used by users with local followers in the last week:", followed)
	}

	if len(all) > 0 {
		if len(followed) > 0 {
			w.Empty()
		}
		printHashtags(w, r, "Most popular hashtags used by any user in the last week:", all)
	}

	if len(followed) > 0 || len(all) > 0 {
		w.Separator()
	}

	if r.User == nil {
		w.Link("/search", "🔎 Posts by hashtag")
	} else {
		w.Link("/users/search", "🔎 Posts by hashtag")
	}
}
