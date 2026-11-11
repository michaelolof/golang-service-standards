// Package jobs holds the job contract core, adapters and workers share.
package jobs

import "context"

// Job is what every runtime moves.
type Job struct{ Kind string }

// Enqueuer is used by core.
type Enqueuer interface {
	Enqueue(ctx context.Context, job Job) error
}

// Claimer has an implementation, and only its own package uses it.
type Claimer interface{ Claim() }

// claimAll is the only use of Claimer.
func claimAll(cs []Claimer) {
	for _, c := range cs {
		c.Claim()
	}
}

var _ = claimAll
