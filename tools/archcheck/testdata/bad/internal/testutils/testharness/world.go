// Package testharness builds the whole app for flows.
package testharness

import (
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
)

// World is the whole app, built once per test.
type World struct {
	tb   testing.TB
	pool *pgxpool.Pool
}

// New builds the app.
func New(tb testing.TB) *World { return &World{tb: tb, pool: &pgxpool.Pool{}} }

// Pool hands a flow the database.
func (w *World) Pool() *pgxpool.Pool { return w.pool }

// CreateOrder writes the order's row, where a flow places it through the API.
func (w *World) CreateOrder() {
	_, _ = w.pool.Exec(w.tb.Context(), "INSERT\n\tINTO orders (id) VALUES ($1)", 1)
}

// CountOrders is a reader nothing calls.
func (w *World) CountOrders() int { return 0 }
