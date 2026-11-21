package order_test

import (
	"testing"

	"example.com/kinds/internal/core/order"
)

// A unit test, with no tag.
func TestPlace_PlacesAnOrder(t *testing.T) {
	if !order.Place() {
		t.Fatal("not placed")
	}
}
