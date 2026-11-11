// Package transfer moves money.
package transfer

import (
	"example.com/bad/internal/domain"
	"example.com/bad/internal/foundation/clock"
	"example.com/bad/internal/foundation/errkinds"
	"example.com/bad/internal/jobs"
	"example.com/bad/internal/ports"
	"github.com/michaelolof/errx"
)

// Store is implemented by the transfers repository.
type Store interface {
	Get(id [16]byte) (domain.Transfer, error)
}

// Unused is implemented by nothing.
type Unused interface{ Do() }

// Notifier is implemented only by a fake in an internal test.
type Notifier interface{ Notify(t Transfer) error }

// Lister is implemented only by a fake in an external test.
type Lister interface{ List() ([]Transfer, error) }

// Transfer is this package's own view.
type Transfer struct{}

// Deps are the service's dependencies.
type Deps struct {
	Store    Store
	Clock    clock.Clock
	Mailer   ports.Mailer
	Notifier Notifier
	Lister   Lister
	Unused   Unused
	Jobs     jobs.Enqueuer
}

// Service orchestrates transfers.
type Service struct{ d Deps }

// New builds a Service.
func New(d Deps) (*Service, error) { return &Service{d: d}, nil }

// Get reads a transfer.
func (s *Service) Get(id [16]byte) (domain.Transfer, error) {
	t, err := s.d.Store.Get(id)
	if errx.IsKind(err, errkinds.ErrKindNotFound) || errx.IsKind(err, errkinds.ErrKindChecked) {
		return domain.Transfer{}, err
	}

	return t, err
}
