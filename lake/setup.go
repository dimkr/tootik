/*
Copyright 2026 Dima Krasner

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

// Package lake maintains a pg_lake replica of the primary and the statistics computed from it.
//
// The pg_lake database holds a logical replica of the columns of persons, notes, outbox and servers that statistics
// need, in heap tables with the same names in the public schema. Triggers on these tables record the key of every
// inserted, updated or deleted row in a change log. [Exporter] applies the logged changes to Iceberg tables in the lake
// schema and records the last export in lake.exports, and [StatusUpdater] computes statistics from these Iceberg
// tables and writes them to the primary.
package lake

import (
	"context"
	"database/sql"
	"fmt"
)

// replicatedTable is a table replicated from the primary and exported to an Iceberg table with the same name.
type replicatedTable struct {
	Name    string
	Key     string
	KeyType string
	Columns string
}

var replicatedTables = []replicatedTable{
	{"persons", "pk", "bigint", "id, host, inserted, updated"},
	{"notes", "pk", "bigint", "author, host, inserted"},
	{"outbox", "pk", "bigint", "inserted, sent"},
	{"servers", "host", "text", "capabilities"},
}

// Setup creates the tables of a pg_lake database and subscribes to the publication named lake on the primary.
//
// publisher is the connection string the pg_lake server uses to connect to the primary. The subscription and its
// replication slot on the primary are named after the pg_lake database.
func Setup(ctx context.Context, db *sql.DB, publisher string) error {
	queries := []string{
		`CREATE EXTENSION IF NOT EXISTS pg_lake CASCADE`,

		`CREATE TABLE IF NOT EXISTS public.persons(pk bigint PRIMARY KEY, id text NOT NULL, host text, inserted timestamptz NOT NULL, updated timestamptz NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS public.notes(pk bigint PRIMARY KEY, author text NOT NULL, host text, inserted timestamptz NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS public.outbox(pk bigint PRIMARY KEY, inserted timestamptz NOT NULL, sent boolean)`,
		`CREATE TABLE IF NOT EXISTS public.servers(host text PRIMARY KEY, capabilities bigint NOT NULL)`,

		`CREATE SCHEMA IF NOT EXISTS lake`,
		`CREATE TABLE IF NOT EXISTS lake.persons(pk bigint NOT NULL, id text NOT NULL, host text, inserted timestamptz NOT NULL, updated timestamptz NOT NULL) USING iceberg`,
		`CREATE TABLE IF NOT EXISTS lake.notes(pk bigint NOT NULL, author text NOT NULL, host text, inserted timestamptz NOT NULL) USING iceberg`,
		`CREATE TABLE IF NOT EXISTS lake.outbox(pk bigint NOT NULL, inserted timestamptz NOT NULL, sent boolean) USING iceberg`,
		`CREATE TABLE IF NOT EXISTS lake.servers(host text NOT NULL, capabilities bigint NOT NULL) USING iceberg`,
		`CREATE TABLE IF NOT EXISTS lake.exports(started timestamptz NOT NULL, duration interval NOT NULL, rows bigint NOT NULL)`,
	}

	for _, table := range replicatedTables {
		queries = append(
			queries,
			fmt.Sprintf(`CREATE TABLE IF NOT EXISTS lake.%s_changes(seq bigint GENERATED ALWAYS AS IDENTITY PRIMARY KEY, %s %s NOT NULL)`, table.Name, table.Key, table.KeyType),
			fmt.Sprintf(`CREATE OR REPLACE FUNCTION lake.log_%[1]s_change() RETURNS trigger LANGUAGE plpgsql AS $$
			BEGIN
				INSERT INTO lake.%[1]s_changes(%[2]s) VALUES (coalesce(NEW.%[2]s, OLD.%[2]s));
				RETURN NULL;
			END
			$$`, table.Name, table.Key),
			fmt.Sprintf(`CREATE OR REPLACE TRIGGER %[1]s_changes_insert_delete AFTER INSERT OR DELETE ON public.%[1]s FOR EACH ROW EXECUTE FUNCTION lake.log_%[1]s_change()`, table.Name),
			fmt.Sprintf(`CREATE OR REPLACE TRIGGER %[1]s_changes_update AFTER UPDATE ON public.%[1]s FOR EACH ROW WHEN (OLD IS DISTINCT FROM NEW) EXECUTE FUNCTION lake.log_%[1]s_change()`, table.Name),
			fmt.Sprintf(`ALTER TABLE public.%[1]s ENABLE ALWAYS TRIGGER %[1]s_changes_insert_delete`, table.Name),
			fmt.Sprintf(`ALTER TABLE public.%[1]s ENABLE ALWAYS TRIGGER %[1]s_changes_update`, table.Name),
		)
	}

	for _, query := range queries {
		if _, err := db.ExecContext(ctx, query); err != nil {
			return fmt.Errorf("failed to run %s: %w", query, err)
		}
	}

	var subscribed bool
	if err := db.QueryRowContext(ctx, `select exists (select 1 from pg_subscription where subname = current_database())`).Scan(&subscribed); err != nil {
		return fmt.Errorf("failed to check if subscribed: %w", err)
	}

	if subscribed {
		return nil
	}

	var subscribe string
	if err := db.QueryRowContext(ctx, `select format('CREATE SUBSCRIPTION %I CONNECTION %L PUBLICATION lake', current_database(), $1::text)`, publisher).Scan(&subscribe); err != nil {
		return fmt.Errorf("failed to build subscription statement: %w", err)
	}

	if _, err := db.ExecContext(ctx, subscribe); err != nil {
		return fmt.Errorf("failed to subscribe: %w", err)
	}

	return nil
}
