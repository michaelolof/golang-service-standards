// Package orderstest seeds orders for the repository's tests.
package orderstest

import (
	"testing"

	"example.com/good/internal/repository/orders"
)

// Seed stores a placed order.
func Seed(tb testing.TB, repo *orders.Repo, id string) {
	tb.Helper()

	if err := repo.Place(tb.Context(), id); err != nil {
		tb.Fatal(err)
	}
}
