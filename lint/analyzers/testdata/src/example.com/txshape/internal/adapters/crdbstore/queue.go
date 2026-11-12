package crdbstore

import (
	"context"

	"example.com/txshape/internal/jobs"
	"github.com/jackc/pgx/v5/pgxpool"
)

var _ jobs.Enqueuer = (*Queue)(nil)

// Queue opens its own transaction, so it can't join the caller's.
type Queue struct{ pool *pgxpool.Pool }

func (q *Queue) Enqueue(ctx context.Context, job jobs.Job) error {
	return q.insert(ctx, job)
}

func (q *Queue) insert(ctx context.Context, job jobs.Job) error {
	tx, err := q.pool.Begin(ctx) // want `Queue.Enqueue implements the boundary port jobs.Enqueuer, so it joins the caller's transaction and never opens one: Begin opens a transaction \(DATA-4\)`
	if err != nil {
		return err
	}

	return tx.Commit()
}
