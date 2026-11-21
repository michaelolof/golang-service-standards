package transfer_test

import (
	"testing"

	"example.com/good/internal/core/transfer"
	"example.com/good/internal/repository/transfers/transferstest"
)

// fakeStore is what transfer.Store needs, and no test calls Get. Deleting Get,
// or notFound, which it calls, would break the build.
type fakeStore struct{}

func (fakeStore) Get(id [16]byte) error { return notFound(id) }

func notFound([16]byte) error { return nil }

var _ transfer.Store = fakeStore{}

// ExampleNew has no output comment, so go test compiles it and never runs it.
// Deleting exampleDeps would still break the build.
func ExampleNew() { _, _ = transfer.New(exampleDeps()) }

func exampleDeps() transfer.Deps { return transfer.Deps{} }

// windowsPath is called only from a _windows_test.go file, which this
// machine's build may leave out.
func windowsPath() string { return `C:\\transfers` }

// A service builds over the fake repository.
func TestNew_BuildsOverTheFakeRepository(t *testing.T) {
	if _, err := transfer.New(transfer.Deps{Store: transferstest.NewFakeRepo()}); err != nil {
		t.Fatal(err)
	}
}
