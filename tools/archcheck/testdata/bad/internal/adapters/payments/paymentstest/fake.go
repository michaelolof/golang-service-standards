// Package paymentstest has the payment provider's fake, for unit tests.
package paymentstest

// FakeProvider stands in for the payment provider.
type FakeProvider struct{}

// NewFakeProvider returns a FakeProvider.
func NewFakeProvider() *FakeProvider { return &FakeProvider{} }
