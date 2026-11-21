//go:build flow

package orders_test

import (
	"testing"

	"example.com/good/internal/testutils/testharness"
)

// An operator sees an order the tenant's app placed through the API.
func TestPlaceOrder_AnOperatorSeesTheOrder(t *testing.T) {
	w := testharness.New(t)

	if got := w.ReadTransfer("o-1"); got != "o-1" {
		t.Fatalf("read %q", got)
	}
}
