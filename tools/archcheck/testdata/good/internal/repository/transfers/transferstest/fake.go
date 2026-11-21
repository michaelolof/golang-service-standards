// Package transferstest has the transfers repository's fake.
package transferstest

import (
	"example.com/good/internal/core/transfer"
	"example.com/good/internal/foundation/errkinds"
	"github.com/michaelolof/errx"
)

var _ transfer.Store = (*FakeRepo)(nil)

// FakeRepo stores transfers in memory.
type FakeRepo struct{}

// NewFakeRepo returns an empty FakeRepo.
func NewFakeRepo() *FakeRepo { return &FakeRepo{} }

// Get is what transfer.Store needs. A test passes the fake as the interface,
// so the call graph counts Get as reached, though no test calls it.
func (f *FakeRepo) Get(id [16]byte) error {
	return errx.NewKind(0, errkinds.ErrKindNotFound, "missing")
}
