package ordering

import (
	"context"

	"example.com/txshape/internal/domain"
	"example.com/txshape/internal/jobs"
	"example.com/txshape/internal/ports"
)

// Store is the orders repository.
type Store interface {
	Get(ctx context.Context, id domain.OrderID) (domain.Order, error)
	Place(ctx context.Context, args domain.PlaceOrderArgs) (domain.Order, error)
	Create(ctx context.Context, order domain.Order) error
	Cancel(ctx context.Context, id domain.OrderID) error
}

// ItemStore is the order items repository.
type ItemStore interface {
	Create(ctx context.Context, id domain.OrderID, line int) error
	CreateMany(ctx context.Context, id domain.OrderID) error
}

type Service struct {
	tx     ports.Tx
	orders Store
	items  ItemStore
	audit  ports.AuditWriter
	keys   ports.Idempotency
	events ports.Events
	jobs   jobs.BatchEnqueuer
}
