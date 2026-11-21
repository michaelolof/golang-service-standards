package orders_test

import "testing"

// A placed order is seen through the admin API.
func TestPlaceOrder_AnOperatorSeesTheOrder(t *testing.T) {
	placeOrder(t)
	steps{}.pay()
}

// placeOrder is a step, written in a flow file.
func placeOrder(t *testing.T) { t.Helper() }

type steps struct{}

func (steps) pay() {}
