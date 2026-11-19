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
// need, in heap tables with the same names in the public schema. [Exporter] copies them to Iceberg tables in the lake
// schema and records the last export in lake.exports, and [StatusUpdater] computes statistics from these Iceberg
// tables and writes them to the primary.
package lake

import (
	"context"
	"database/sql"
	"fmt"
)

// Setup creates the tables of a pg_lake database and subscribes to the publication named lake on the primary.
//
// publisher is the connection string the pg_lake server uses to connect to the primary. The subscription and its
// replication slot on the primary are named after the pg_lake database.
func Setup(ctx context.Context, db *sql.DB, publisher string) error {
	for _, query := range []string{
		`CREATE EXTENSION IF NOT EXISTS pg_lake CASCADE`,

		`CREATE TABLE IF NOT EXISTS public.persons(pk bigint PRIMARY KEY, id text NOT NULL, host text, inserted timestamptz NOT NULL, updated timestamptz NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS public.notes(pk bigint PRIMARY KEY, author text NOT NULL, host text, inserted timestamptz NOT NULL)`,
		`CREATE TABLE IF NOT EXISTS public.outbox(pk bigint PRIMARY KEY, inserted timestamptz NOT NULL, sent boolean)`,
		`CREATE TABLE IF NOT EXISTS public.servers(host text PRIMARY KEY, capabilities bigint NOT NULL)`,

		`CREATE SCHEMA IF NOT EXISTS lake`,
		`CREATE TABLE IF NOT EXISTS lake.persons(id text NOT NULL, host text, inserted timestamptz NOT NULL, updated timestamptz NOT NULL) USING iceberg`,
		`CREATE TABLE IF NOT EXISTS lake.notes(author text NOT NULL, host text, inserted timestamptz NOT NULL) USING iceberg`,
		`CREATE TABLE IF NOT EXISTS lake.outbox(inserted timestamptz NOT NULL, sent boolean) USING iceberg`,
		`CREATE TABLE IF NOT EXISTS lake.servers(capabilities bigint NOT NULL) USING iceberg`,
		`CREATE TABLE IF NOT EXISTS lake.exports(started timestamptz NOT NULL, duration interval NOT NULL, rows bigint NOT NULL)`,
	} {
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
