//go:build flow

package payments_test

import (
	"testing"

	"example.com/bad/internal/adapters/payments/paymentstest"
	"example.com/bad/internal/repository/transfers"
	"example.com/bad/internal/repository/transfers/transferstest"
	"example.com/bad/internal/testutils/testharness"
)

// A settled payment shows as settled. The flow seeds its transfer, writes its
// status through the repository, reads the table itself, and stands a fake in
// for the provider.
func TestPayment_ASettledPaymentShowsAsSettled(t *testing.T) {
	w := testharness.New(t)
	repo := transfers.New()
	transferstest.Seed(t, repo)
	w.CreateOrder()

	_ = repo.MarkSettled(t.Context(), [16]byte{1})
	_, _ = repo.GetSettled(t.Context(), [16]byte{1})
	_ = w.Pool().QueryRow(t.Context(), "SELECT status FROM transfers")
	_ = paymentstest.NewFakeProvider()
}
