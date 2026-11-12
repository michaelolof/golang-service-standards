package jobs

import (
	"context"

	"example.com/txshape/internal/foundation/txboundary"
)

type Job struct{ Kind string }

// Enqueuer is a boundary port: the queue lives in the same database.
type Enqueuer interface {
	txboundary.Port
	Enqueue(ctx context.Context, job Job) error
}

// BatchEnqueuer is a boundary port through Enqueuer.
type BatchEnqueuer interface {
	Enqueuer
	EnqueueBatch(ctx context.Context, all []Job) error
}
