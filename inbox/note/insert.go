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

package note

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dimkr/tootik/ap"
	"log/slog"
)

func expand(aud ap.Audience, arr *[3]sql.NullString) {
	have := 0

	for _, id := range aud.Keys() {
		if id == ap.Public {
			continue
		}

		if !arr[have].Valid {
			arr[have].Valid = true
			arr[have].String = id
			have++
			if have == len(arr) {
				break
			}
		}
	}
}

func Insert(ctx context.Context, log *slog.Logger, tx *sql.Tx, note *ap.Object) error {
	body, err := json.Marshal(note)
	if err != nil {
		return fmt.Errorf("Failed to marshal note %s: %w", note.ID, err)
	}

	var to, cc [3]sql.NullString

	expand(note.To, &to)
	expand(note.CC, &cc)

	hashtags := map[string]string{}

	for _, tag := range note.Tag {
		if tag.Type != ap.HashtagMention {
			continue
		}

		if tag.Name == "" {
			continue
		}

		if tag.Name[0] == '#' {
			hashtags[strings.ToLower(tag.Name[1:])] = tag.Name[1:]
		} else {
			hashtags[strings.ToLower(tag.Name)] = tag.Name
		}
	}

	public := 0
	if note.IsPublic() {
		public = 1
	}

	if _, err = tx.ExecContext(
		ctx,
		`INSERT INTO notes (id, hash, author, object, public, to0, to1, to2, cc0, cc1, cc2) VALUES(?,?,?,?,?,?,?,?,?,?,?)`,
		note.ID,
		fmt.Sprintf("%x", sha256.Sum256([]byte(note.ID))),
		note.AttributedTo,
		string(body),
		public,
		to[0],
		to[1],
		to[2],
		cc[0],
		cc[1],
		cc[2],
	); err != nil {
		return fmt.Errorf("Failed to insert note %s: %w", note.ID, err)
	}

	if _, err = tx.ExecContext(ctx, `update notes SET groupid = (select id from persons where actor->>'type' = 'Group' and (id in (notes.cc0, notes.to0, notes.cc1, notes.to1, notes.cc2, notes.to2)) or (notes.cc2 is not null and exists (select 1 from json_each(notes.object->'cc') where value = persons.id)) or (notes.to2 is not null and exists (select 1 from json_each(notes.object->'to') where value = persons.id)) limit 1) where id = ?`, note.ID); err != nil {
		log.Warn("Failed to set post group", "error", err)
	}

	for _, hashtag := range hashtags {
		if _, err = tx.ExecContext(ctx, `insert into hashtags (note, hashtag) values(?,?)`, note.ID, hashtag); err != nil {
			log.Warn("Failed to tag post", "hashtag", hashtag, "error", err)
		}
	}

	return nil
}
