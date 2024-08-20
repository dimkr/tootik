/*
Copyright 2024 Dima Krasner

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

package inbox

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/dimkr/tootik/cfg"
)

type FeedUpdater struct {
	Domain string
	Config *cfg.Config
	DB     *sql.DB
}

func (u FeedUpdater) Run(ctx context.Context) error {
	since := int64(0)
	var ts sql.NullInt64
	if err := u.DB.QueryRowContext(ctx, `select max(inserted) from feed`).Scan(&ts); err != nil {
		return err
	} else if ts.Valid {
		since = ts.Int64
	}

	if _, err := u.DB.ExecContext(
		ctx,
		`
			insert into feed(follower, note, author, sharer, inserted)
			select follows.follower, notes.object as note, persons.actor as author, null as sharer, notes.inserted from
			follows
			join
			persons
			on
				persons.id = follows.followed
			join
			notes
			on
				notes.author = follows.followed and
				(
					notes.public = 1 or
					persons.actor->>'$.followers' in (notes.cc0, notes.to0, notes.cc1, notes.to1, notes.cc2, notes.to2) or
					follows.follower in (notes.cc0, notes.to0, notes.cc1, notes.to1, notes.cc2, notes.to2) or
					(notes.to2 is not null and exists (select 1 from json_each(notes.object->'$.to') where value = persons.actor->>'$.followers' or value = follows.follower)) or
					(notes.cc2 is not null and exists (select 1 from json_each(notes.object->'$.cc') where value = persons.actor->>'$.followers' or value = follows.follower))
				)
			where
				follows.follower like $1 and
				notes.inserted >= $2 and
				not exists (select 1 from feed where feed.follower = follows.follower and feed.note->>'$.id' = notes.id and feed.sharer is null)
			union
			select myposts.author as follower, notes.object as note, authors.actor as author, null as sharer, notes.inserted from
			notes myposts
			join
			notes
			on
				notes.object->>'$.inReplyTo' = myposts.id
			join
			persons authors
			on
				authors.id = notes.author
			where
				notes.author != myposts.author and
				notes.inserted >= $2 and
				myposts.author like $1 and
				not exists (select 1 from feed where feed.follower = notes.author and feed.note->>'$.id' = notes.id and feed.sharer is null)
			union all
			select follows.follower, notes.object as note, authors.actor as author, sharers.actor as sharer, shares.inserted from
			follows
			join
			shares
			on
				shares.by = follows.followed
			join
			notes
			on
				notes.id = shares.note
			join
			persons authors
			on
				authors.id = notes.author
			join
			persons sharers
			on
				sharers.id = follows.followed
			where
				notes.public = 1 and
				shares.inserted >= $2 and
				follows.follower like $1 and
				not exists (select 1 from feed where feed.follower = follows.follower and feed.note->>'$.id' = notes.id and feed.sharer->>'$.id' = sharers.id)
		`,
		fmt.Sprintf("https://%s/%%", u.Domain),
		since,
	); err != nil {
		return err
	}

	return nil
}
