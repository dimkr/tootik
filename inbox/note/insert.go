/*
Copyright 2023, 2024 Dima Krasner

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

// Package note handles insertion of posts.
package note

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/front/text/plain"
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

// Flatten converts a post into text that can be indexed for search purposes.
func Flatten(note *ap.Object) string {
	content, links := plain.FromHTML(note.Content)
	for _, mention := range note.Tag {
		if mention.Type == ap.MentionMention {
			links.Store(mention.Href, mention.Href)
		}
	}
	for _, attachment := range note.Attachment {
		if attachment.Href != "" {
			links.Store(attachment.Href, attachment.Href)
		} else if attachment.URL != "" {
			links.Store(attachment.URL, attachment.URL)
		}
	}
	var b strings.Builder
	b.WriteString(content)
	b.WriteByte(' ')
	b.WriteString(note.AttributedTo)
	links.Range(func(link, alt string) bool {
		if alt == "" {
			return true
		}
		b.WriteByte(' ')
		b.WriteString(link)
		return true
	})
	return b.String()
}

// Insert inserts a post.
func Insert(ctx context.Context, log *slog.Logger, tx *sql.Tx, note *ap.Object) error {
	body, err := json.Marshal(note)
	if err != nil {
		return fmt.Errorf("failed to marshal note %s: %w", note.ID, err)
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
		`INSERT INTO notes (id, author, object, public, to0, to1, to2, cc0, cc1, cc2) VALUES(?,?,?,?,?,?,?,?,?,?)`,
		note.ID,
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
		return fmt.Errorf("failed to insert note %s: %w", note.ID, err)
	}

	if _, err = tx.ExecContext(
		ctx,
		`INSERT INTO notesfts (id, content) VALUES(?,?)`,
		note.ID,
		Flatten(note),
	); err != nil {
		return fmt.Errorf("failed to insert note %s: %w", note.ID, err)
	}

	for _, hashtag := range hashtags {
		if _, err = tx.ExecContext(ctx, `insert into hashtags (note, hashtag) values(?,?)`, note.ID, hashtag); err != nil {
			log.Warn("Failed to tag post", "hashtag", hashtag, "error", err)
		}
	}

	return nil
}
