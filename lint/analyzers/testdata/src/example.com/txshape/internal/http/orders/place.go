package orders

import (
	"context"

	"example.com/txshape/internal/domain"
	"example.com/txshape/internal/ports"
)

// Store is the orders repository, as this handler uses it.
type Store interface {
	Place(ctx context.Context, args domain.PlaceOrderArgs) (domain.Order, error)
	Cancel(ctx context.Context, id domain.OrderID) error
}

// Deps stands in for gofi's Deps.
type Deps interface {
	Tx() ports.Tx
	Orders() Store
	Audit() ports.AuditWriter
}

// PlaceHandler stands in for a gofi handler: a function literal in a
// package-level variable (HTTP-1).
var PlaceHandler = func(ctx context.Context, d Deps, args domain.PlaceOrderArgs) error {
	return d.Tx().Within(ctx, func(ctx context.Context) error {
		if _, err := d.Orders().Place(ctx, args); err != nil {
			return err
		}

		if err := d.Orders().Cancel(ctx, args.ID); err != nil { // want `Store.Cancel is a second business write in the transaction`
			return err
		}

		return d.Audit().Record(ctx, domain.AuditEntry{Action: "order.place"})
	})
}

var withTx = func(ctx context.Context, tx ports.Tx, fn func(ctx context.Context) error) error { // want `this function takes a func\(context.Context\) error and runs it in a transaction`
	return tx.Within(ctx, fn)
}
