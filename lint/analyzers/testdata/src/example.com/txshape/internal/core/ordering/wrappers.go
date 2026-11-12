package ordering

import (
	"context"

	"example.com/txshape/internal/domain"
)

// WriteWithEvent adds an event to any write.
func (s *Service) WriteWithEvent(ctx context.Context, event domain.Event, fn func(ctx context.Context) error) error { // want `WriteWithEvent takes a func\(context.Context\) error and runs it in a transaction: a generic wrapper lets any number of writes share one. Open the transaction in the use case, around its one write and its boundary writes \(DATA-4\)`
	return s.tx.Within(ctx, func(ctx context.Context) error {
		if err := fn(ctx); err != nil {
			return err
		}

		return s.events.Create(ctx, event) // want `ports.Events.Create isn't a boundary port`
	})
}

// WithAudit adds an audit record to any write.
func (s *Service) WithAudit(ctx context.Context, entry domain.AuditEntry, fn func(ctx context.Context) error) error { // want `WithAudit takes a func\(context.Context\) error and runs it in a transaction`
	return s.tx.Within(ctx, func(ctx context.Context) error {
		if err := fn(ctx); err != nil {
			return err
		}

		return s.audit.Record(ctx, entry)
	})
}

// Run hands its callback straight to Within.
func (s *Service) Run(ctx context.Context, fn func(ctx context.Context) error) error { // want `Run takes a func\(context.Context\) error and runs it in a transaction`
	return s.tx.Within(ctx, fn)
}
