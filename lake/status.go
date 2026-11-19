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

package lake

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"

	"github.com/dimkr/tootik/ap"
)

// Names of graphs in the statusgraphs table.
const (
	GraphDailyPosts             = "dailyposts"
	GraphWeeklyPosts            = "weeklyposts"
	GraphWeeklyFailedDeliveries = "weeklyfaileddeliveries"
	GraphUsers                  = "users"
	GraphActiveUsers            = "activeusers"
	GraphKnownInstances         = "knowninstances"
	GraphActiveInstances        = "activeinstances"
	GraphInstanceCapabilities   = "instancecapabilities"
)

// StatusUpdater computes statistics from the Iceberg tables and writes them to the status and statusgraphs tables.
type StatusUpdater struct {
	Domain string
	Lake   *sql.DB
	DBW    *sql.DB
}

type graphPoint struct {
	Label string
	Value int64
}

type graph struct {
	Name   string
	Points []graphPoint
}

func (u *StatusUpdater) queryGraph(ctx context.Context, name, query string, args ...any) (graph, error) {
	rows, err := u.Lake.QueryContext(ctx, query, args...)
	if err != nil {
		return graph{}, fmt.Errorf("failed to query %s: %w", name, err)
	}
	defer rows.Close()

	g := graph{Name: name}
	for rows.Next() {
		var p graphPoint
		if err := rows.Scan(&p.Label, &p.Value); err != nil {
			return graph{}, fmt.Errorf("failed to scan %s: %w", name, err)
		}
		g.Points = append(g.Points, p)
	}

	if err := rows.Err(); err != nil {
		return graph{}, fmt.Errorf("failed to query %s: %w", name, err)
	}

	return g, nil
}

func (u *StatusUpdater) queryGraphs(ctx context.Context) ([]graph, error) {
	graphs := make([]graph, 0, 8)

	for _, q := range []struct {
		Name  string
		Query string
		Args  []any
	}{
		{
			GraphDailyPosts,
			`select to_char(hour at time zone 'UTC', 'YYYY-MM-DD HH24:MI'), count(*) from (select date_trunc('hour', inserted, 'UTC') as hour from lake.notes where inserted > now() - interval '1 day' and inserted < date_trunc('hour', now(), 'UTC')) hours group by hour order by hour`,
			nil,
		},
		{
			GraphWeeklyPosts,
			`select to_char(day at time zone 'UTC', 'YYYY-MM-DD'), count(*) from (select date_trunc('day', inserted, 'UTC') as day from lake.notes where inserted > now() - interval '7 days' and inserted < date_trunc('day', now(), 'UTC')) days group by day order by day`,
			nil,
		},
		{
			GraphWeeklyFailedDeliveries,
			`select to_char(day at time zone 'UTC', 'YYYY-MM-DD HH24:MI'), count(*) from (select date_trunc('day', inserted, 'UTC') as day from lake.outbox where not sent and inserted > now() - interval '7 days' and inserted < date_trunc('day', now(), 'UTC')) days group by day order by day`,
			nil,
		},
		{
			GraphUsers,
			`select to_char(hour at time zone 'UTC', 'YYYY-MM-DD HH24:MI'), count(*) from (select date_trunc('hour', inserted, 'UTC') as hour from lake.persons where inserted > now() - interval '1 day' and inserted < date_trunc('hour', now(), 'UTC')) hours group by hour order by hour`,
			nil,
		},
		{
			GraphActiveUsers,
			`select to_char(day at time zone 'UTC', 'YYYY-MM-DD'), count(distinct author) from (select date_trunc('day', notes.inserted, 'UTC') as day, persons.id as author from lake.notes notes join lake.persons persons on persons.id = notes.author where notes.inserted > now() - interval '7 days' and notes.inserted < date_trunc('day', now(), 'UTC')) days group by day order by day`,
			nil,
		},
		{
			GraphKnownInstances,
			`select to_char(days.day at time zone 'UTC', 'YYYY-MM-DD'), count(*) from (select host, min(date_trunc('day', inserted, 'UTC')) as day from lake.persons group by host) hosts join (select distinct date_trunc('day', inserted, 'UTC') as day from lake.persons where inserted > now() - interval '7 days' and inserted < date_trunc('day', now(), 'UTC')) days on hosts.day < days.day group by days.day order by days.day`,
			nil,
		},
		{
			GraphActiveInstances,
			`select host, (cast(round(avg(posts)) as int)) as daily from (select host, day, count(*) as posts from (select host, date_trunc('day', inserted, 'UTC') as day from lake.notes where inserted > now() - interval '7 days') days group by host, day) posts group by host order by daily desc limit 10`,
			nil,
		},
		{
			GraphInstanceCapabilities,
			`
			select 'RFC9421 with ML-DSA-44', (select count(*) from lake.servers where capabilities & $1 > 0)
			union all
			select 'RFC9421 with Ed25519', (select count(*) from lake.servers where capabilities & $2 > 0)
			union all
			select 'RFC9421 with RSA but without Ed25519 or ML-DSA-44', (select count(*) from lake.servers where capabilities & ($1 | $2 | $3) = $3)
			union all
			select 'RFC9421 without draft-cavage-http-signatures', (select count(*) from lake.servers where capabilities & $4 = 0 and capabilities & ($1 | $2 | $3) > 0)
			union all
			select 'draft-cavage-http-signatures without RFC9421', (select count(*) from lake.servers where capabilities & $4 > 0 and capabilities & ($1 | $2 | $3) = 0)
			union all
			select 'draft-cavage-http-signatures', (select count(*) from lake.servers where capabilities & $4 > 0)
			union all
			select 'Total', (select count(*) from lake.servers)
			`,
			[]any{
				ap.RFC9421MLDSA44Signatures,
				ap.RFC9421Ed25519Signatures,
				ap.RFC9421RSASignatures,
				ap.CavageDraftSignatures,
			},
		},
	} {
		g, err := u.queryGraph(ctx, q.Name, q.Query, q.Args...)
		if err != nil {
			return nil, err
		}
		graphs = append(graphs, g)
	}

	return graphs, nil
}

// Run computes statistics and replaces the previous ones, unless the Iceberg tables have no local users.
func (u *StatusUpdater) Run(ctx context.Context) error {
	var usersCount, postsCount, postsToday, federatedPostsCount, federatedPostsToday int64
	var lastPost, lastFederatedPost, lastRegister, lastFederatedUser sql.NullTime

	if err := u.Lake.QueryRowContext(ctx, `select count(*), max(inserted) from lake.persons where host = $1`, u.Domain).Scan(&usersCount, &lastRegister); err != nil {
		return fmt.Errorf("failed to get users count: %w", err)
	}

	if usersCount == 0 {
		slog.Warn("Skipping status update from Iceberg tables without local users")
		return nil
	}

	if err := u.Lake.QueryRowContext(ctx, `select count(*), count(*) filter (where inserted >= now() - interval '1 day'), max(inserted) from lake.notes where host = $1`, u.Domain).Scan(&postsCount, &postsToday, &lastPost); err != nil {
		return fmt.Errorf("failed to get posts count: %w", err)
	}

	if err := u.Lake.QueryRowContext(ctx, `select count(*), count(*) filter (where inserted >= now() - interval '1 day'), max(inserted) from lake.notes where host != $1`, u.Domain).Scan(&federatedPostsCount, &federatedPostsToday, &lastFederatedPost); err != nil {
		return fmt.Errorf("failed to get federated posts count: %w", err)
	}

	if err := u.Lake.QueryRowContext(ctx, `select greatest(max(inserted), max(updated)) from lake.persons where host != $1`, u.Domain).Scan(&lastFederatedUser); err != nil {
		return fmt.Errorf("failed to get last federated user update time: %w", err)
	}

	var exported sql.NullTime
	var exportDuration sql.NullString
	var exportedRows sql.NullInt64
	if err := u.Lake.QueryRowContext(ctx, `select started, duration::text, rows from lake.exports`).Scan(&exported, &exportDuration, &exportedRows); err != nil && !errors.Is(err, sql.ErrNoRows) {
		return fmt.Errorf("failed to get last export: %w", err)
	}

	graphs, err := u.queryGraphs(ctx)
	if err != nil {
		return err
	}

	tx, err := u.DBW.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	if _, err := tx.ExecContext(ctx, `delete from status`); err != nil {
		return fmt.Errorf("failed to delete status: %w", err)
	}

	if _, err := tx.ExecContext(
		ctx,
		`insert into status(users, posts, poststoday, federatedposts, federatedpoststoday, lastpost, lastfederatedpost, lastregister, lastfederateduser, exported, exportduration, exportedrows) values($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11::interval, $12)`,
		usersCount,
		postsCount,
		postsToday,
		federatedPostsCount,
		federatedPostsToday,
		lastPost,
		lastFederatedPost,
		lastRegister,
		lastFederatedUser,
		exported,
		exportDuration,
		exportedRows,
	); err != nil {
		return fmt.Errorf("failed to insert status: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `delete from statusgraphs`); err != nil {
		return fmt.Errorf("failed to delete graphs: %w", err)
	}

	for _, g := range graphs {
		for i, p := range g.Points {
			if _, err := tx.ExecContext(ctx, `insert into statusgraphs(graph, position, label, value) values($1, $2, $3, $4)`, g.Name, i, p.Label, p.Value); err != nil {
				return fmt.Errorf("failed to insert %s: %w", g.Name, err)
			}
		}
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit status: %w", err)
	}

	return nil
}
