// Package orders serves the orders API.
package orders

import "github.com/michaelolof/gofi"

// CancelOrderHandler cancels an order.
var CancelOrderHandler = gofi.DefineHandler(gofi.RouteOptions{
	Schema: &cancelOrderSchema{},

	Handler: func(c gofi.Context) error {
		return c.Send(200, nil)
	},
})

type cancelOrderSchema struct{}
