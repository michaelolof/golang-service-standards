package ports

import (
	"context"

	"example.com/txshape/internal/domain"
	"example.com/txshape/internal/foundation/txboundary"
)

// Tx runs fn in one transaction.
type Tx interface {
	Within(ctx context.Context, fn func(ctx context.Context) error) error
}

// AuditWriter is the shared audit writer, a boundary port.
type AuditWriter interface {
	txboundary.Port
	Record(ctx context.Context, entry domain.AuditEntry) error
}

// Idempotency is the shared idempotency store, a boundary port.
type Idempotency interface {
	txboundary.Port
	Save(ctx context.Context, record domain.IdempotencyRecord) error
}

// Events writes an outbox event. It isn't a boundary port: the event goes in
// the use-case statement.
type Events interface {
	Create(ctx context.Context, event domain.Event) error
}

// EventWriter is marked as a boundary port, but runs the caller's code.
type EventWriter interface {
	txboundary.Port
	CreateWith(ctx context.Context, event domain.Event, fn func(ctx context.Context) error) error // want `EventWriter.CreateWith takes a callback: a boundary port writes its own record in the caller's transaction, and never runs the caller's code \(DATA-4\)`
}
