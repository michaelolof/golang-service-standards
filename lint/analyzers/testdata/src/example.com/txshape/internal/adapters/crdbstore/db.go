package crdbstore

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// DB is the transaction adapter. Its Within runs the callback, which is what
// a transaction adapter is for.
type DB struct{ pool *pgxpool.Pool }

func (d *DB) Within(ctx context.Context, fn func(ctx context.Context) error) error {
	tx, err := d.pool.Begin(ctx)
	if err != nil {
		return err
	}

	if err := fn(ctx); err != nil {
		return err
	}

	return tx.Commit()
}
