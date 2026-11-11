// Package transfers stores transfers.
package transfers

import (
	"example.com/bad/internal/domain"
	"example.com/bad/internal/foundation/errkinds"
	"github.com/michaelolof/errx"
)

// Repo implements transfer.Store, without saying so.
type Repo struct{}

// New builds a Repo.
func New() *Repo { return &Repo{} }

// Get reads a transfer.
func (r *Repo) Get(id [16]byte) (domain.Transfer, error) {
	if id == ([16]byte{}) {
		return domain.Transfer{}, errx.NewKind(0, errkinds.ErrKindNotFound, "missing")
	}

	return domain.Transfer{}, errx.NewKind(0, errkinds.ErrKindConflict, "taken")
}
