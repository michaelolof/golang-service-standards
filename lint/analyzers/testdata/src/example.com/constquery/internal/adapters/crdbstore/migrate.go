// A migration runner runs the statements it reads, so they aren't constants.
package crdbstore

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

func Migrate(ctx context.Context, db *pgxpool.Pool, statements []string) error {
	for _, statement := range statements {
		if _, err := db.Exec(ctx, statement); err != nil {
			return err
		}
	}

	_, err := db.Exec(ctx, `UPDATE schema_version SET applied_at = now()`) // want `time comes from the clock`

	return err
}
