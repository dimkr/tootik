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

package test

import (
	"database/sql"
	"fmt"
	"strings"
	"testing"
	"time"

	"github.com/dimkr/tootik/lake"
	"github.com/dimkr/tootik/test/deps"
	"github.com/stretchr/testify/assert"
)

const lakeRetryInterval = 100 * time.Millisecond

// waitForLake waits until the pg_lake replica has as many rows as the primary in every replicated table.
func waitForLake(t *testing.T, server *server, lakeDB *sql.DB) {
	t.Helper()

	ticker := time.NewTicker(lakeRetryInterval)
	defer ticker.Stop()

	for {
		synced := true

		for _, table := range []string{"persons", "notes", "outbox", "servers"} {
			var primary, replica int64
			if err := server.dbw.QueryRowContext(t.Context(), `select count(*) from `+table).Scan(&primary); err != nil {
				t.Fatalf("Failed to count %s on the primary: %v", table, err)
			}

			if err := lakeDB.QueryRowContext(t.Context(), `select count(*) from public.`+table).Scan(&replica); err != nil {
				t.Fatalf("Failed to count %s on pg_lake: %v", table, err)
			}

			if primary != replica {
				synced = false
				break
			}
		}

		if synced {
			return
		}

		select {
		case <-t.Context().Done():
			t.Fatalf("Failed to wait for pg_lake: %v", t.Context().Err())

		case <-ticker.C:
		}
	}
}

func countRows(t *testing.T, db *sql.DB, query string, args ...any) int64 {
	t.Helper()

	var count int64
	if err := db.QueryRowContext(t.Context(), query, args...).Scan(&count); err != nil {
		t.Fatalf("Failed to run %s: %v", query, err)
	}

	return count
}

func updateStatus(t *testing.T, server *server, lakeDB *sql.DB) {
	t.Helper()

	if err := (&lake.Exporter{Lake: lakeDB}).Run(t.Context()); err != nil {
		t.Fatalf("Failed to export: %v", err)
	}

	if err := (&lake.StatusUpdater{Domain: domain, Lake: lakeDB, DBW: server.dbw}).Run(t.Context()); err != nil {
		t.Fatalf("Failed to update status: %v", err)
	}
}

func TestStatus_NewInstance(t *testing.T) {
	server := newTestServer(t)

	assert := assert.New(t)

	status := server.Handle("/status", server.Alice)
	assert.Regexp("^20 text/gemini\r\n", status)

	lines := strings.Split(status, "\n")
	assert.Contains(lines, "* Statistics are not available yet")
	assert.Contains(lines, "* Incoming activities queue size: 0")
	assert.Contains(lines, "* Outgoing activities queue size: 0")
}

func TestStatus_WithPosts(t *testing.T) {
	server := newTestServer(t)

	assert := assert.New(t)

	lakeDB := deps.NewLake(t, server.database)

	say := server.Handle("/users/say?Hello%20world", server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, say)

	say = server.Handle("/users/say?Hello%20from%20Bob", server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, say)

	id := say[15 : len(say)-2]
	delete := server.Handle("/users/delete/"+id, server.Bob)
	assert.Equal(fmt.Sprintf("30 /users/view/%s\r\n", id), delete)

	users := countRows(t, server.dbw, `select count(*) from persons where host = $1`, domain)
	posts := countRows(t, server.dbw, `select count(*) from notes where host = $1`, domain)
	postsToday := countRows(t, server.dbw, `select count(*) from notes where host = $1 and inserted >= now() - interval '1 day'`, domain)
	exportedRows := countRows(t, server.dbw, `select (select count(*) from persons) + (select count(*) from notes) + (select count(*) from outbox) + (select count(*) from servers)`)

	waitForLake(t, server, lakeDB)
	updateStatus(t, server, lakeDB)

	status := server.Handle("/status", server.Alice)
	assert.Regexp("^20 text/gemini\r\n", status)

	lines := strings.Split(status, "\n")
	assert.Contains(lines, fmt.Sprintf("* Local users: %d", users))
	assert.Contains(lines, fmt.Sprintf("* Local posts: %d", posts))
	assert.Contains(lines, fmt.Sprintf("* Local posts today: %d", postsToday))
	assert.Contains(lines, "* Federated posts: 0")
	assert.Contains(lines, fmt.Sprintf("* Latest Iceberg export changed rows: %d", exportedRows))
	assert.Condition(func() bool {
		for _, line := range lines {
			if strings.HasPrefix(line, "* Latest Iceberg export: ") {
				return true
			}
		}
		return false
	}, "no export time in %s", status)
	assert.NotContains(lines, "* Statistics are not available yet")
}

func TestStatus_KeepsResultsOnEmptyLake(t *testing.T) {
	server := newTestServer(t)

	assert := assert.New(t)

	lakeDB := deps.NewLake(t, server.database)

	say := server.Handle("/users/say?Hello%20world", server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, say)

	users := countRows(t, server.dbw, `select count(*) from persons where host = $1`, domain)
	posts := countRows(t, server.dbw, `select count(*) from notes where host = $1`, domain)

	waitForLake(t, server, lakeDB)
	updateStatus(t, server, lakeDB)

	if _, err := lakeDB.ExecContext(t.Context(), `delete from lake.persons`); err != nil {
		t.Fatalf("Failed to delete persons from Iceberg: %v", err)
	}

	if err := (&lake.StatusUpdater{Domain: domain, Lake: lakeDB, DBW: server.dbw}).Run(t.Context()); err != nil {
		t.Fatalf("Failed to update status: %v", err)
	}

	status := server.Handle("/status", server.Alice)
	lines := strings.Split(status, "\n")
	assert.Contains(lines, fmt.Sprintf("* Local users: %d", users))
	assert.Contains(lines, fmt.Sprintf("* Local posts: %d", posts))
}

// publishedRows returns the columns of persons, notes, outbox and servers that the primary publishes to pg_lake, by
// table and key.
func publishedRows(t *testing.T, server *server) map[string]string {
	t.Helper()

	published := map[string]string{}

	for _, query := range []string{
		`select 'persons:' || pk, row(id, host, inserted, updated)::text from persons`,
		`select 'notes:' || pk, row(author, host, inserted)::text from notes`,
		`select 'outbox:' || pk, row(inserted, sent)::text from outbox`,
		`select 'servers:' || host, row(capabilities)::text from servers`,
	} {
		rows, err := server.dbw.QueryContext(t.Context(), query)
		if err != nil {
			t.Fatalf("Failed to run %s: %v", query, err)
		}

		for rows.Next() {
			var key, row string
			if err := rows.Scan(&key, &row); err != nil {
				rows.Close()
				t.Fatalf("Failed to scan %s: %v", query, err)
			}
			published[key] = row
		}

		if err := rows.Err(); err != nil {
			rows.Close()
			t.Fatalf("Failed to run %s: %v", query, err)
		}

		rows.Close()
	}

	return published
}

// countChangedKeys returns the number of keys that were added, removed or changed between two results of
// publishedRows.
func countChangedKeys(before, after map[string]string) int {
	changed := 0

	for key, row := range after {
		if prev, ok := before[key]; !ok || prev != row {
			changed++
		}
	}

	for key := range before {
		if _, ok := after[key]; !ok {
			changed++
		}
	}

	return changed
}

// waitForReplica waits until a query that returns one text value returns the same value on the primary and on the
// pg_lake replica.
func waitForReplica(t *testing.T, server *server, lakeDB *sql.DB, query string, args ...any) {
	t.Helper()

	ticker := time.NewTicker(lakeRetryInterval)
	defer ticker.Stop()

	for {
		var primary, replica string
		if err := server.dbw.QueryRowContext(t.Context(), query, args...).Scan(&primary); err != nil {
			t.Fatalf("Failed to run %s on the primary: %v", query, err)
		}

		if err := lakeDB.QueryRowContext(t.Context(), query, args...).Scan(&replica); err != nil {
			t.Fatalf("Failed to run %s on pg_lake: %v", query, err)
		}

		if primary == replica {
			return
		}

		select {
		case <-t.Context().Done():
			t.Fatalf("Failed to wait for pg_lake: %v", t.Context().Err())

		case <-ticker.C:
		}
	}
}

func TestStatus_IncrementalExport(t *testing.T) {
	server := newTestServer(t)

	assert := assert.New(t)

	lakeDB := deps.NewLake(t, server.database)

	say := server.Handle("/users/say?Hello%20world", server.Alice)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, say)

	waitForLake(t, server, lakeDB)

	for _, table := range []struct{ Name, Key string }{{"persons", "pk"}, {"notes", "pk"}, {"outbox", "pk"}, {"servers", "host"}} {
		t.Logf(
			"%s: %d logged keys, %d replicated rows",
			table.Name,
			countRows(t, lakeDB, fmt.Sprintf(`select count(distinct %s) from lake.%s_changes`, table.Key, table.Name)),
			countRows(t, lakeDB, `select count(*) from public.`+table.Name),
		)
	}

	first := publishedRows(t, server)
	updateStatus(t, server, lakeDB)

	lines := strings.Split(server.Handle("/status", server.Alice), "\n")
	assert.Contains(lines, fmt.Sprintf("* Local posts: %d", countRows(t, server.dbw, `select count(*) from notes where host = $1`, domain)))
	assert.Contains(lines, fmt.Sprintf("* Latest Iceberg export changed rows: %d", countChangedKeys(nil, first)))

	res, err := server.dbw.ExecContext(t.Context(), `delete from notes where author = $1`, server.Alice.ID)
	if err != nil {
		t.Fatalf("Failed to delete posts: %v", err)
	}

	deleted, err := res.RowsAffected()
	if err != nil {
		t.Fatalf("Failed to count deleted posts: %v", err)
	}
	assert.EqualValues(1, deleted)

	say = server.Handle("/users/say?Hello%20from%20Bob", server.Bob)
	assert.Regexp(`^30 /users/view/\S+\r\n$`, say)

	waitForLake(t, server, lakeDB)
	second := publishedRows(t, server)
	updateStatus(t, server, lakeDB)

	assert.Equal(countRows(t, server.dbw, `select count(*) from notes where host = $1`, domain), countRows(t, server.dbw, `select posts from status`))
	assert.EqualValues(countChangedKeys(first, second), countRows(t, server.dbw, `select exportedrows from status`))
	assert.Equal(countRows(t, server.dbw, `select count(*) from notes`), countRows(t, lakeDB, `select count(*) from lake.notes`))

	if _, err := server.dbw.ExecContext(t.Context(), `update notes set nreplies = nreplies + 1`); err != nil {
		t.Fatalf("Failed to update replies count: %v", err)
	}

	if _, err := server.dbw.ExecContext(t.Context(), `update persons set updated = now() where id = $1`, server.Alice.ID); err != nil {
		t.Fatalf("Failed to update user: %v", err)
	}

	waitForReplica(t, server, lakeDB, `select updated::text from persons where id = $1`, server.Alice.ID)
	third := publishedRows(t, server)
	updateStatus(t, server, lakeDB)

	assert.EqualValues(countChangedKeys(second, third), countRows(t, server.dbw, `select exportedrows from status`))

	var updated, exported time.Time
	if err := server.dbw.QueryRowContext(t.Context(), `select updated from persons where id = $1`, server.Alice.ID).Scan(&updated); err != nil {
		t.Fatalf("Failed to get user: %v", err)
	}

	if err := lakeDB.QueryRowContext(t.Context(), `select updated from lake.persons where id = $1`, server.Alice.ID).Scan(&exported); err != nil {
		t.Fatalf("Failed to get exported user: %v", err)
	}
	assert.True(updated.Equal(exported), "exported %s instead of %s", exported, updated)
}
