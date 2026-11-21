// Package testharness builds the whole app for flows, and the steps they compose.
package testharness

import (
	"context"

	"github.com/jackc/pgx/v5/pgxpool"
)

// createPackageDatabase changes the schema, not the rows, so it isn't a seed.
func createPackageDatabase(ctx context.Context, pool *pgxpool.Pool, name string) error {
	_, err := pool.Exec(ctx, "CREATE DATABASE "+name)

	return err
}
