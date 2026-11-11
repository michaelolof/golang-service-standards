package transferjobs

import (
	"context"
	"errors"

	"example.com/jobs/internal/jobs"
	"github.com/michaelolof/errx"
)

func initiate(context.Context) error { return nil }

// Initiate submits a transfer.
type Initiate struct{}

func (h *Initiate) Handle(ctx context.Context, job jobs.Job) error {
	if err := initiate(ctx); err != nil {
		return errx.Wrap(0, err)
	}

	return nil
}

// Processor decides its own outcome.
type Processor struct{}

func (p *Processor) Handle(ctx context.Context, job jobs.Job) error { // want `a job handler is named for its action, such as Initiate or BuildCSV, not Processor \(JOB-1\)`
	err := initiate(ctx)
	if errx.IsKind(err, "rejected") { // want `a job handler returns its error and never inspects it: the runtime's one mapping picks the outcome \(JOB-1, ERR-8\)`
		return nil
	}

	if errors.Is(err, context.Canceled) { // want `a job handler returns its error and never inspects it`
		return nil
	}

	return err
}

// Execute is too vague.
type Execute struct{}

func (e Execute) Handle(ctx context.Context, job jobs.Job) error { return initiate(ctx) } // want `not Execute \(JOB-1\)`

// ProvisionWorker names its action, so the suffix is left to review.
type ProvisionWorker struct{}

func (w ProvisionWorker) Handle(ctx context.Context, job jobs.Job) error { return initiate(ctx) }

// Router isn't a job handler: its Handle has another shape.
type Router struct{}

func (r Router) Handle(path string) bool { return errors.Is(nil, nil) }
