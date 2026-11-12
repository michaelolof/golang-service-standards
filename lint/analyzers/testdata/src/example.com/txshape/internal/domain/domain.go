package domain

import "context"

type OrderID [16]byte

type Order struct {
	ID     OrderID
	Status string
}

type PlaceOrderArgs struct{ ID OrderID }

type AuditEntry struct{ Action string }

type Event struct{ Kind string }

type IdempotencyRecord struct{ Key string }

// AuditCapture writes its entry through w, out of txshape's sight.
type AuditCapture struct{ Entry AuditEntry }

func (c AuditCapture) Save(ctx context.Context, w interface {
	Record(ctx context.Context, entry AuditEntry) error
}) error {
	return w.Record(ctx, c.Entry)
}

type actorKey struct{}

// ActorFrom reads the actor from ctx. It writes nothing.
func ActorFrom(ctx context.Context) (string, bool) {
	actor, ok := ctx.Value(actorKey{}).(string)

	return actor, ok
}
