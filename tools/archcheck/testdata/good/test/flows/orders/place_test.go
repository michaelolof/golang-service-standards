//go:build flow

package orders_test

import (
	"testing"

	"example.com/good/internal/repository/orders"
	"example.com/good/internal/repository/orders/orderstest"
	"example.com/good/internal/testutils/testharness"
)

// A placed order is seen through the admin API. This flow predates the rules
// on setup: docs/promises/orders.md lists it under "Not yet proved as the
// caller", so it keeps its seeds until TEST-14 is lifted.
func TestPlaceOrder_AnOperatorSeesTheOrder(t *testing.T) {
	w := testharness.New(t)
	orderstest.Seed(t, orders.New(), "o-1")
	w.SeedPlacedOrder("o-2")
}
