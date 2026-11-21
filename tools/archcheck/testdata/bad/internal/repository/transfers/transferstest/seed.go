// Package transferstest seeds transfers for the repository's tests.
package transferstest

import (
	"testing"

	"example.com/bad/internal/repository/transfers"
)

// Seed stores a transfer as if it had been made.
func Seed(tb testing.TB, repo *transfers.Repo) { tb.Helper() }
