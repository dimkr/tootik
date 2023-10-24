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

package migrations

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
)

type migration struct {
	ID string
	Up func(context.Context, *sql.Tx) error
}

//go:generate ./list.sh

func applyMigration(ctx context.Context, db *sql.DB, m migration) error {
	tx, err := db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("Failed to apply %s: %w", m.ID, err)
	}
	defer tx.Rollback()

	if err := m.Up(ctx, tx); err != nil {
		return fmt.Errorf("Failed to apply %s: %w", m.ID, err)
	}

	if _, err := tx.ExecContext(ctx, `insert into migrations(id) values (?)`, m.ID); err != nil {
		return fmt.Errorf("Failed to record %s: %w", m.ID, err)
	}

	if err = tx.Commit(); err != nil {
		return fmt.Errorf("Failed to commit %s: %w", m.ID, err)
	}

	return nil
}

func Run(ctx context.Context, log *slog.Logger, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `create table if not exists migrations(id string not null primary key, applied integer default (unixepoch()))`); err != nil {
		return err
	}

	for _, m := range migrations {
		var applied string
		if err := db.QueryRowContext(ctx, `select datetime(applied, 'unixepoch') from migrations where id = ?`, m.ID).Scan(&applied); err != nil && !errors.Is(err, sql.ErrNoRows) {
			return fmt.Errorf("Failed to check if %s is applied: %w", m.ID, err)
		} else if err == nil {
			log.Debug("Skipping migration", "id", m.ID, "applied", applied)
			continue
		}

		log.Info("Applying migration", "id", m.ID)
		if err := applyMigration(ctx, db, m); err != nil {
			return err
		}
	}

	return nil
}
