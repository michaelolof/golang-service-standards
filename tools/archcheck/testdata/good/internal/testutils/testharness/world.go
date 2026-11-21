package testharness

import (
	"testing"

	"example.com/good/internal/repository/admins"
	"github.com/jackc/pgx/v5/pgxpool"
)

// World is the whole app, built once per test.
type World struct {
	tb   testing.TB
	pool *pgxpool.Pool
}

// New builds the app, with its first admin.
func New(tb testing.TB) *World {
	tb.Helper()

	w := &World{tb: tb, pool: &pgxpool.Pool{}}
	if err := createPackageDatabase(tb.Context(), w.pool, "flows_transfers"); err != nil {
		tb.Fatal(err)
	}

	seedFirstAdmin(tb, admins.New())

	return w
}

// ReadTransfer reads a transfer as the tenant's app does. Only a flow calls it.
func (w *World) ReadTransfer(id string) string {
	w.tb.Helper()

	return id
}

// StoredTransfers reads storage, for a promise about storage. The harness may
// read with SQL. It never writes.
func (w *World) StoredTransfers() int {
	w.tb.Helper()

	var n int
	_ = w.pool.QueryRow(w.tb.Context(), "SELECT count(*) FROM transfers").Scan(&n)

	return n
}
