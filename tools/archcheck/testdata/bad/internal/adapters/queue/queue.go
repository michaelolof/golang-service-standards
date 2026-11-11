// Package queue runs jobs.
package queue

import (
	"context"

	"example.com/bad/internal/jobs"
)

var _ jobs.Enqueuer = (*Queue)(nil)

// Queue implements both of jobs' interfaces.
type Queue struct{}

// Enqueue puts a job on the queue.
func (q *Queue) Enqueue(_ context.Context, _ jobs.Job) error { return nil }

// Claim claims a job.
func (q *Queue) Claim() {}
