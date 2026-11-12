package orders

import (
	"context"

	"example.com/txshape/internal/domain"
	"example.com/txshape/internal/ports"
	"github.com/jackc/pgx/v5/pgxpool"
)

// Repo opens its own transaction where the database can't make the change in
// one statement: DATA-4's database-limit exception, which txshape doesn't check.
type Repo struct {
	db *pgxpool.Pool
	tx ports.Tx
}

func New(db *pgxpool.Pool, tx ports.Tx) *Repo { return &Repo{db: db, tx: tx} }

func (r *Repo) Place(ctx context.Context, args domain.PlaceOrderArgs) (domain.Order, error) {
	err := r.tx.Within(ctx, func(ctx context.Context) error {
		if _, err := r.db.Exec(ctx, `INSERT INTO orders (id) VALUES ($1)`, args.ID); err != nil {
			return err
		}

		_, err := r.db.Exec(ctx, `UPDATE orders SET status = 'PLACED' WHERE id = $1`, args.ID)

		return err
	})

	return domain.Order{ID: args.ID}, err
}
