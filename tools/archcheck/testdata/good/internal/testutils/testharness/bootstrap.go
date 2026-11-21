package testharness

import (
	"testing"

	"example.com/good/internal/repository/admins"
	"example.com/good/internal/repository/admins/adminstest"
)

// seedFirstAdmin seeds the one identity no API can create: the first admin.
func seedFirstAdmin(tb testing.TB, repo *admins.Repo) {
	tb.Helper()

	adminstest.Seed(tb, repo, "first@example.com")
	_ = repo.Create(tb.Context(), "second@example.com")
}
