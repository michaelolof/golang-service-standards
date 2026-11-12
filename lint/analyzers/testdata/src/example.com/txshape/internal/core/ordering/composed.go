package ordering

import (
	"context"

	"example.com/txshape/internal/domain"
)

// PlaceComposed builds the use case out of four repository calls.
func (s *Service) PlaceComposed(ctx context.Context, order domain.Order) error {
	return s.tx.Within(ctx, func(ctx context.Context) error {
		if err := s.orders.Create(ctx, order); err != nil {
			return err
		}

		if err := s.items.CreateMany(ctx, order.ID); err != nil { // want `ItemStore.CreateMany is a second business write in the transaction: one use-case repository method makes the whole change, in one statement, or in a transaction it opens itself where the database can't \(DATA-4\)`
			return err
		}

		if err := s.events.Create(ctx, domain.Event{Kind: "order.placed"}); err != nil { // want `ports.Events.Create isn't a boundary port: its write goes in the use-case statement, or after the commit. Only a port that passes DATA-4's three tests embeds txboundary.Port \(DATA-4\)`
			return err
		}

		return s.audit.Record(ctx, domain.AuditEntry{Action: "order.place"})
	})
}

// PlaceWithEvent writes its outbox event through a port that isn't a boundary.
func (s *Service) PlaceWithEvent(ctx context.Context, args domain.PlaceOrderArgs) error {
	return s.tx.Within(ctx, func(ctx context.Context) error {
		if _, err := s.orders.Place(ctx, args); err != nil {
			return err
		}

		return s.events.Create(ctx, domain.Event{Kind: "order.placed"}) // want `ports.Events.Create isn't a boundary port`
	})
}

// Cancel reads, decides, then writes.
func (s *Service) Cancel(ctx context.Context, id domain.OrderID) error {
	return s.tx.Within(ctx, func(ctx context.Context) error {
		order, err := s.orders.Get(ctx, id) // want `Store.Get reads inside the transaction: put the guard in the statement's WHERE clause instead, so nothing can change between the check and the write \(DATA-4, DATA-7\)`
		if err != nil {
			return err
		}

		if order.Status != "PLACED" {
			return nil
		}

		if err := s.orders.Cancel(ctx, id); err != nil {
			return err
		}

		return s.audit.Record(ctx, domain.AuditEntry{Action: "order.cancel"})
	})
}

// Settle reads through a function of this package.
func (s *Service) Settle(ctx context.Context, id domain.OrderID) error {
	return s.tx.Within(ctx, func(ctx context.Context) error {
		if err := s.mustExist(ctx, id); err != nil {
			return err
		}

		if err := s.orders.Cancel(ctx, id); err != nil {
			return err
		}

		return s.audit.Record(ctx, domain.AuditEntry{Action: "order.settle"})
	})
}

func (s *Service) mustExist(ctx context.Context, id domain.OrderID) error {
	_, err := s.orders.Get(ctx, id) // want `called in the transaction at composed\.go:\d+: Store\.Get reads inside the transaction`

	return err
}

// AddItems writes one row per loop.
func (s *Service) AddItems(ctx context.Context, id domain.OrderID, lines int) error {
	return s.tx.Within(ctx, func(ctx context.Context) error {
		for line := range lines {
			if err := s.items.Create(ctx, id, line); err != nil { // want `ItemStore.Create runs in a loop, so the transaction makes several business writes: one use-case repository method writes every row, in one statement, or in a transaction it opens itself where the database can't \(DATA-4\)`
				return err
			}
		}

		return s.audit.Record(ctx, domain.AuditEntry{Action: "order.items"})
	})
}

// PlaceCaptured hands the ctx to code in another package.
func (s *Service) PlaceCaptured(ctx context.Context, args domain.PlaceOrderArgs, capture domain.AuditCapture) error {
	return s.tx.Within(ctx, func(ctx context.Context) error {
		if _, err := s.orders.Place(ctx, args); err != nil {
			return err
		}

		return capture.Save(ctx, s.audit) // want `domain.AuditCapture.Save is given the transaction's ctx, and txshape can't see what it writes: call the repository method or boundary port directly in the closure \(DATA-4\)`
	})
}

// PlaceAlone opens a transaction for one write.
func (s *Service) PlaceAlone(ctx context.Context, args domain.PlaceOrderArgs) error {
	return s.tx.Within(ctx, func(ctx context.Context) error { // want `the transaction holds one business write and no boundary write, so it has no reason to exist: call the use-case method on its own \(DATA-4\)`
		_, err := s.orders.Place(ctx, args)

		return err
	})
}

// Remember opens a transaction with no business write.
func (s *Service) Remember(ctx context.Context, key string) error {
	return s.tx.Within(ctx, func(ctx context.Context) error { // want `the transaction makes no business write: a transaction that core opens holds one use-case write and its boundary writes \(DATA-4\)`
		return s.keys.Save(ctx, domain.IdempotencyRecord{Key: key})
	})
}
