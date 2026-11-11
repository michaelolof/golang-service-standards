// Package transfers stores transfers.
package transfers

import (
	"example.com/good/internal/core/transfer"
	"example.com/good/internal/foundation/errkinds"
	"github.com/michaelolof/errx"
)

var _ transfer.Store = (*Repo)(nil)

// Repo stores transfers.
type Repo struct{}

// New builds a Repo.
func New() *Repo { return &Repo{} }

// Get reads a transfer.
func (r *Repo) Get(id [16]byte) error {
	return errx.NewKind(0, errkinds.ErrKindNotFound, "missing")
}
