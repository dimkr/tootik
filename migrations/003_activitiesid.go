package migrations

import (
	"context"
	"database/sql"
)

func activitiesid(ctx context.Context, db *sql.DB) error {
	if _, err := db.ExecContext(ctx, `CREATE UNIQUE INDEX activitiesid ON activities(activity->>'id')`); err != nil {
		return err
	}

	return nil
}
