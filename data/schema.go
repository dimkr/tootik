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
package data

import (
	"context"
	"database/sql"
)

func Migrate(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS persons(id STRING NOT NULL PRIMARY KEY, hash STRING NOT NULL, actor JSON NOT NULL, inserted INTEGER DEFAULT (UNIXEPOCH()), updated INTEGER DEFAULT (UNIXEPOCH()))`); err != nil {
		return err
	}

	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS notes(id STRING NOT NULL PRIMARY KEY, hash STRING NOT NULL, author STRING NOT NULL, object JSON NOT NULL, to0 STRING, to1 STRING, to2 STRING, cc0 STRING, cc1 STRING, cc2 STRING, inserted INTEGER DEFAULT (UNIXEPOCH()))`); err != nil {
		return err
	}

	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS follows(id STRING NOT NULL PRIMARY KEY, follower STRING NOT NULL, followed JSON NOT NULL, inserted INTEGER DEFAULT (UNIXEPOCH()))`); err != nil {
		return err
	}

	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS icons(id STRING NOT NULL PRIMARY KEY, buf BLOB NOT NULL)`); err != nil {
		return err
	}

	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS deliveries(id STRING NOT NULL PRIMARY KEY, inserted INTEGER DEFAULT (UNIXEPOCH()), attempts INTEGER DEFAULT 0, last INTEGER DEFAULT (UNIXEPOCH()))`); err != nil {
		return err
	}

	if _, err := db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS hashtags(note id STRING NOT NULL, hashtag STRING COLLATE NOCASE NOT NULL)`); err != nil {
		return err
	}

	if _, err := db.ExecContext(ctx, `CREATE UNIQUE INDEX IF NOT EXISTS personsidhash ON persons(id, hash)`); err != nil {
		return err
	}

	if _, err := db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS notesauthor ON notes(author)`); err != nil {
		return err
	}

	if _, err := db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS notesinreplyto ON notes(object->>'inReplyTo')`); err != nil {
		return err
	}

	if _, err := db.ExecContext(ctx, `CREATE UNIQUE INDEX IF NOT EXISTS notesidhash ON notes(id, hash)`); err != nil {
		return err
	}

	if _, err := db.ExecContext(ctx, `CREATE INDEX IF NOT EXISTS hashtagshashtag ON hashtags(hashtag)`); err != nil {
		return err
	}

	return nil
}
