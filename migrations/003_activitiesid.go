package migrations

import (
	"context"
	"database/sql"
)

func activitiesid(ctx context.Context, tx *sql.Tx) error {
	if _, err := tx.ExecContext(ctx, `CREATE UNIQUE INDEX activitiesid ON activities(activity->>'id')`); err != nil {
		return err
	}

	return nil
}
