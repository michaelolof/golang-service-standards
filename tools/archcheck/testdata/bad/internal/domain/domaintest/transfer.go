// Package domaintest builds valid domain values for tests.
package domaintest

import "example.com/bad/internal/domain"

// Transfer is a builder nothing calls.
func Transfer() domain.Transfer { return domain.Transfer{} }
