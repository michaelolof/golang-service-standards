package testharness

import "example.com/good/internal/repository/orders"

// SeedPlacedOrder writes an order's row. Only a flow listed under "Not yet
// proved as the caller" uses it, so it stays until TEST-14 is lifted.
func (w *World) SeedPlacedOrder(id string) {
	w.tb.Helper()

	if err := orders.New().Place(w.tb.Context(), id); err != nil {
		w.tb.Fatal(err)
	}
}
