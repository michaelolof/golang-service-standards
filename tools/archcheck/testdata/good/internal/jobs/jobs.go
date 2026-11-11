// Package jobs holds the job contract core, adapters and workers share.
package jobs

import "context"

// Job is what every runtime moves.
type Job struct {
	Kind string
	Body []byte
}

// Enqueuer puts a job on its runtime. Only other packages use it: core
// enqueues through it, and each runtime's adapter implements it (JOB-1).
type Enqueuer interface {
	Enqueue(ctx context.Context, job Job) error
}
