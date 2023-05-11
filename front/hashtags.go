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
	"regexp"
	"time"
)

func init() {
	handlers[regexp.MustCompile(`^/hashtags$`)] = withCache(withUserMenu(hashtags), time.Minute*30)
	handlers[regexp.MustCompile(`^/users/hashtags$`)] = withCache(withUserMenu(hashtags), time.Minute*30)
}

func hashtags(w text.Writer, r *request) {
	rows, err := r.Query(`select hashtag from (select distinct hashtags.hashtag, notes.author from hashtags join notes where notes.id = hashtags.note) group by hashtag order by count(*) desc limit 30`)
	if err != nil {
		r.Log.WithError(err).Warn("Failed to list hashtags")
		w.Error()
		return
	}

	var tags []string

	for rows.Next() {
		var tag string
		if err := rows.Scan(&tag); err != nil {
			r.Log.WithError(err).Warn("Failed to scan hashtag")
			continue
		}

		tags = append(tags, tag)
	}
	rows.Close()

	w.OK()
	w.Title("🔥 Hashtags")

	for _, tag := range tags {
		if r.User == nil {
			w.Link("/hashtag/"+tag, "#"+tag)
		} else {
			w.Link("/users/hashtag/"+tag, "#"+tag)
		}
	}
}
