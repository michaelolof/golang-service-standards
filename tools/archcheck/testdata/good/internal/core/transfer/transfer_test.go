package transfer_test

import (
	"testing"

	"example.com/good/internal/core/transfer"
	"example.com/good/internal/repository/transfers/transferstest"
)

// A service builds over the fake repository.
func TestNew_BuildsOverTheFakeRepository(t *testing.T) {
	if _, err := transfer.New(transfer.Deps{Store: transferstest.NewFakeRepo()}); err != nil {
		t.Fatal(err)
	}
}
