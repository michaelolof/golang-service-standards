// Package adminstest seeds admins for the repository's tests, and for the one
// identity no API can create: the first admin.
package adminstest

import (
	"testing"

	"example.com/good/internal/repository/admins"
)

// Seed stores an admin.
func Seed(tb testing.TB, repo *admins.Repo, email string) {
	tb.Helper()

	if err := repo.Create(tb.Context(), email); err != nil {
		tb.Fatal(err)
	}
}
