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
	log "github.com/sirupsen/logrus"
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

func Insert(ctx context.Context, db *sql.DB, note *ap.Object, logger *log.Logger) error {
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

	if _, err = db.ExecContext(
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

	for _, hashtag := range hashtags {
		if _, err = db.ExecContext(ctx, `insert into hashtags (note, hashtag) values(?,?)`, note.ID, hashtag); err != nil {
			log.WithFields(log.Fields{"note": note.ID, "hashtag": hashtag}).WithError(err).Warn("Failed to tag post")
		}
	}

	return nil
}
