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

func Insert(ctx context.Context, db *sql.DB, note *ap.Object, logger *log.Logger) error {
	body, err := json.Marshal(note)
	if err != nil {
		return fmt.Errorf("Failed to marshal note %s: %w", note.ID, err)
	}

	var to0, to1, to2, cc0, cc1, cc2 sql.NullString

	to := note.To.Keys()

	if len(to) > 0 {
		to0.Valid = true
		to0.String = to[0]
	}
	if len(to) > 1 {
		to1.Valid = true
		to1.String = to[1]
	}
	if len(to) > 2 {
		to2.Valid = true
		to2.String = to[2]
	}

	cc := note.CC.Keys()

	if len(cc) > 0 {
		cc0.Valid = true
		cc0.String = cc[0]
	}
	if len(cc) > 1 {
		cc1.Valid = true
		cc1.String = cc[1]
	}
	if len(cc) > 2 {
		cc2.Valid = true
		cc2.String = cc[2]
	}

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

	if _, err = db.ExecContext(
		ctx,
		`INSERT INTO notes (id, hash, author, object, to0, to1, to2, cc0, cc1, cc2) VALUES(?,?,?,?,?,?,?,?,?,?)`,
		note.ID,
		fmt.Sprintf("%x", sha256.Sum256([]byte(note.ID))),
		note.AttributedTo,
		string(body),
		to0,
		to1,
		to2,
		cc0,
		cc1,
		cc2,
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
