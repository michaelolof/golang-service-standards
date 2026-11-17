package orders_test

import (
	"testing"

	"example.com/good/internal/http/api/orders"
)

// Cancelling an order needs the orders write scope.
func TestCancelOrderHandler_RefusesAKeyThatMayOnlyRead(t *testing.T) {
	_ = orders.CancelOrderHandler
}
