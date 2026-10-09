/*
Copyright 2025, 2026 Dima Krasner

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

package migrations

import (
	"database/sql"
	"fmt"
	"os"
	"testing"

	"github.com/dimkr/tootik/ap"
	"github.com/dimkr/tootik/sqlite"
)

const testKey = "z6MkrJVnaZkeFzdQyMZu1cgjg7k1pZZ6pvBQ7XJPt4swbTQ2"

var canonicalCorpus = []string{
	"https://a.example/users/alice",
	"https://a.example/users/alice?x=y#z",
	"https://a.example/users/alice#main-key",
	"https://a.example/.well-known/apgateway/did:key:" + testKey,
	"https://a.example/.well-known/apgateway/did:key:" + testKey + "/actor",
	"https://a.example/.well-known/apgateway/did:key:" + testKey + "/actor#ed25519-key",
	"https://a.example/.well-known/apgateway/did:key:" + testKey + "/actor/inbox?123",
	"https://a.example/.well-known/apgateway/did:key:" + testKey + "/actor/inbox?123#x",
	"https://a.example/.well-known/apgateway/did:key:" + testKey + "/actor#x?y",
	"https://a.example/.well-known/apgateway/did:key:" + testKey + "/x/.well-known/apgateway/did:key:" + testKey + "/actor",
	"https://a.example/x/.well-known/apgateway/did:key:" + testKey + "/actor",
	"ap://did:key:" + testKey + "/actor",
	"ap://did:key:" + testKey + "/actor#main-key",
	"ap://did:key:" + testKey + "/actor?@gateway=https%3A%2F%2Fa.example",
	"ap://did:key:" + testKey + "/actor?@gateway=https%3A%2F%2Fa.example&@gateway=https%3A%2F%2Fb.example#main-key",
	"ap+ef61://did:key:" + testKey + "/actor",
	"ap://did%3Akey%3A" + testKey + "/actor",
	"ap+ef61://did%3akey%3a" + testKey + "/actor?@gateway=https%3A%2F%2Fa.example",
	"ap://did:key%3A" + testKey + "/actor",
}

func TestCanonicalSQL(t *testing.T) {
	f, err := os.CreateTemp("", "tootik-*.sqlite3")
	if err != nil {
		t.Fatalf("Failed to create database: %v", err)
	}
	f.Close()

	path := f.Name()
	defer os.Remove(path)

	db, err := sql.Open(sqlite.DriverName, sqlite.Scheme+path+"?"+sqlite.JournalModeWAL)
	if err != nil {
		t.Fatalf("Failed to open database: %v", err)
	}
	defer db.Close()

	if err := Run(t.Context(), "localhost.localdomain", db); err != nil {
		t.Fatalf("Failed to run migrations: %v", err)
	}

	for i, id := range canonicalCorpus {
		t.Run(id, func(t *testing.T) {
			expected := ap.Canonical(id)

			var direct string
			if err := db.QueryRowContext(t.Context(), `SELECT `+canonicalSQL(`$1`), id).Scan(&direct); err != nil {
				t.Fatalf("Failed to canonicalize %s: %v", id, err)
			}

			if direct != expected {
				t.Fatalf("Expected %s, got %s", expected, direct)
			}

			if _, err := db.ExecContext(
				t.Context(),
				`INSERT INTO notes(slug, id, author, object, public) VALUES ($1, $2, $3, jsonb_object('inReplyTo', $4, 'quote', $4, 'audience', $4, 'context', $4), 1)`,
				fmt.Sprintf("slug%d", i),
				fmt.Sprintf("https://localhost.localdomain/post/%d", i),
				"https://localhost.localdomain/user/alice",
				id,
			); err != nil {
				t.Fatalf("Failed to insert post: %v", err)
			}

			var inReplyTo, quote, audience, context string
			if err := db.QueryRowContext(t.Context(), `SELECT inreplytocid, quotecid, audiencecid, contextcid FROM notes WHERE slug = $1`, fmt.Sprintf("slug%d", i)).Scan(&inReplyTo, &quote, &audience, &context); err != nil {
				t.Fatalf("Failed to fetch post: %v", err)
			}

			if inReplyTo != expected || quote != expected || audience != expected || context != expected {
				t.Fatalf("Expected %s, got %s, %s, %s, %s", expected, inReplyTo, quote, audience, context)
			}
		})
	}
}
