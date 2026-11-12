package ordering

import (
	"context"

	"example.com/txshape/internal/domain"
	"example.com/txshape/internal/jobs"
)

// Place is one statement, with no transaction.
func (s *Service) Place(ctx context.Context, args domain.PlaceOrderArgs) (domain.Order, error) {
	return s.orders.Place(ctx, args)
}

// PlaceAudited is one use-case write beside two boundary writes.
func (s *Service) PlaceAudited(ctx context.Context, args domain.PlaceOrderArgs) (domain.Order, error) {
	var order domain.Order

	err := s.tx.Within(ctx, func(ctx context.Context) error {
		var err error

		order, err = s.orders.Place(ctx, args)
		if err != nil {
			return err
		}

		actor, _ := domain.ActorFrom(ctx)

		if err := s.audit.Record(ctx, domain.AuditEntry{Action: "order.place by " + actor}); err != nil {
			return err
		}

		return s.jobs.Enqueue(ctx, jobs.Job{Kind: "order.fulfil"})
	})

	return order, err
}

// PlaceOnce reaches its boundary write through a function of this package,
// which is followed.
func (s *Service) PlaceOnce(ctx context.Context, args domain.PlaceOrderArgs, key string) error {
	return s.tx.Within(ctx, func(ctx context.Context) error {
		if _, err := s.orders.Place(ctx, args); err != nil {
			return err
		}

		return s.remember(ctx, key)
	})
}

func (s *Service) remember(ctx context.Context, key string) error {
	return s.keys.Save(ctx, domain.IdempotencyRecord{Key: key})
}

// PlaceAll enqueues through a port that is a boundary through the one it embeds.
func (s *Service) PlaceAll(ctx context.Context, args domain.PlaceOrderArgs, all []jobs.Job) error {
	return s.tx.Within(ctx, func(ctx context.Context) error {
		if _, err := s.orders.Place(ctx, args); err != nil {
			return err
		}

		return s.jobs.EnqueueBatch(ctx, all)
	})
}
