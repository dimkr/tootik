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

package lake

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"time"
)

// Exporter replaces the contents of the Iceberg tables with the replicated rows.
type Exporter struct {
	Domain string
	Lake   *sql.DB
}

// Run exports all replicated rows in one transaction, unless the replica is incomplete.
//
// The transaction is REPEATABLE READ, so all tables are exported from the same snapshot of the replica.
func (e *Exporter) Run(ctx context.Context) error {
	started := time.Now()

	tx, err := e.Lake.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelRepeatableRead})
	if err != nil {
		return fmt.Errorf("failed to begin transaction: %w", err)
	}
	defer tx.Rollback()

	var syncing int64
	if err := tx.QueryRowContext(ctx, `select count(*) from pg_subscription_rel join pg_subscription on pg_subscription.oid = pg_subscription_rel.srsubid where pg_subscription.subname = current_database() and pg_subscription_rel.srsubstate != 'r'`).Scan(&syncing); err != nil {
		return fmt.Errorf("failed to check subscription state: %w", err)
	}

	if syncing > 0 {
		slog.Warn("Skipping export until initial synchronization is complete", "tables", syncing)
		return nil
	}

	var users int64
	if err := tx.QueryRowContext(ctx, `select count(*) from public.persons where host = $1`, e.Domain).Scan(&users); err != nil {
		return fmt.Errorf("failed to count local users: %w", err)
	}

	if users == 0 {
		slog.Warn("Skipping export of replica without local users")
		return nil
	}

	for _, query := range []string{
		`delete from lake.persons`,
		`delete from lake.notes`,
		`delete from lake.outbox`,
		`delete from lake.servers`,
	} {
		if _, err := tx.ExecContext(ctx, query); err != nil {
			return fmt.Errorf("failed to run %s: %w", query, err)
		}
	}

	var rows int64
	for _, query := range []string{
		`insert into lake.persons(id, host, inserted, updated) select id, host, inserted, updated from public.persons`,
		`insert into lake.notes(author, host, inserted) select author, host, inserted from public.notes`,
		`insert into lake.outbox(inserted, sent) select inserted, sent from public.outbox`,
		`insert into lake.servers(capabilities) select capabilities from public.servers`,
	} {
		res, err := tx.ExecContext(ctx, query)
		if err != nil {
			return fmt.Errorf("failed to run %s: %w", query, err)
		}

		n, err := res.RowsAffected()
		if err != nil {
			return fmt.Errorf("failed to count rows inserted by %s: %w", query, err)
		}
		rows += n
	}

	if _, err := tx.ExecContext(ctx, `delete from lake.exports`); err != nil {
		return fmt.Errorf("failed to delete previous export: %w", err)
	}

	if _, err := tx.ExecContext(ctx, `insert into lake.exports(started, duration, rows) values($1, make_interval(secs => $2), $3)`, started, time.Since(started).Seconds(), rows); err != nil {
		return fmt.Errorf("failed to record export: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return fmt.Errorf("failed to commit export: %w", err)
	}

	return nil
}
