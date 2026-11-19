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

// Exporter applies the changes recorded in the change logs of the replicated tables to the Iceberg tables.
type Exporter struct {
	Lake *sql.DB
}

// exportChanges consumes the change log of a table, replaces the Iceberg rows of every logged key with the current
// replicated row, if any, and returns the number of distinct keys.
func exportChanges(ctx context.Context, tx *sql.Tx, table replicatedTable) (int64, error) {
	if _, err := tx.ExecContext(ctx, fmt.Sprintf(`CREATE TEMP TABLE %s_batch(%s %s PRIMARY KEY) ON COMMIT DROP`, table.Name, table.Key, table.KeyType)); err != nil {
		return 0, fmt.Errorf("failed to create batch table for %s: %w", table.Name, err)
	}

	res, err := tx.ExecContext(ctx, fmt.Sprintf(`WITH consumed AS (DELETE FROM lake.%[1]s_changes RETURNING %[2]s) INSERT INTO %[1]s_batch SELECT DISTINCT %[2]s FROM consumed`, table.Name, table.Key))
	if err != nil {
		return 0, fmt.Errorf("failed to consume changes to %s: %w", table.Name, err)
	}

	keys, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("failed to count changes to %s: %w", table.Name, err)
	}

	if keys == 0 {
		return 0, nil
	}

	if _, err := tx.ExecContext(ctx, fmt.Sprintf(`DELETE FROM lake.%[1]s WHERE %[2]s IN (SELECT %[2]s FROM %[1]s_batch)`, table.Name, table.Key)); err != nil {
		return 0, fmt.Errorf("failed to delete changed rows of %s: %w", table.Name, err)
	}

	if _, err := tx.ExecContext(ctx, fmt.Sprintf(`INSERT INTO lake.%[1]s(%[2]s, %[3]s) SELECT %[2]s, %[3]s FROM public.%[1]s WHERE %[2]s IN (SELECT %[2]s FROM %[1]s_batch)`, table.Name, table.Key, table.Columns)); err != nil {
		return 0, fmt.Errorf("failed to insert changed rows of %s: %w", table.Name, err)
	}

	return keys, nil
}

// Run applies all logged changes in one transaction, unless the initial synchronization of the replica is incomplete.
//
// The transaction is REPEATABLE READ, so all tables are exported from the same snapshot of the replica, and changes
// logged after the snapshot are left for the next run. The number of distinct keys applied is recorded in
// lake.exports.
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

	var rows int64
	for _, table := range replicatedTables {
		keys, err := exportChanges(ctx, tx, table)
		if err != nil {
			return err
		}
		rows += keys
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
