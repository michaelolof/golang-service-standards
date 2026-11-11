// Package transfer moves money.
package transfer

import (
	"example.com/good/internal/foundation/errkinds"
	"example.com/good/internal/jobs"
	"github.com/michaelolof/errx"
)

// Store is implemented by the transfers repository.
type Store interface{ Get(id [16]byte) error }

// Deps are the service's dependencies.
type Deps struct {
	Store Store
	Jobs  jobs.Enqueuer
}

// Service orchestrates transfers.
type Service struct{ store Store }

// New builds a Service.
func New(d Deps) (*Service, error) { return &Service{store: d.Store}, nil }

// Exists says whether a transfer exists.
func (s *Service) Exists(id [16]byte) (bool, error) {
	err := s.store.Get(id)
	if errx.IsKind(err, errkinds.ErrKindNotFound) {
		return false, nil
	}

	return err == nil, err
}
