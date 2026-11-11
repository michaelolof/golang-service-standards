// Package queue runs jobs.
package queue

import (
	"context"

	"example.com/good/internal/jobs"
)

var _ jobs.Enqueuer = (*Queue)(nil)

// Queue implements jobs.Enqueuer.
type Queue struct{}

// New builds a Queue.
func New() *Queue { return &Queue{} }

// Enqueue puts a job on the queue.
func (q *Queue) Enqueue(_ context.Context, _ jobs.Job) error { return nil }
