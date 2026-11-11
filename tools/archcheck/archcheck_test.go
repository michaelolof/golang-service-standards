package main

import (
	"strings"
	"testing"
)

func run(t *testing.T, dir string) []string {
	t.Helper()

	problems, err := check(dir)
	if err != nil {
		t.Fatal(err)
	}

	lines := make([]string, len(problems))
	for i, p := range problems {
		lines[i] = p.String()
	}

	return lines
}

// A standard project passes, so every check has a clean case to stay quiet on.
func TestCheck_AStandardProjectPasses(t *testing.T) {
	if got := run(t, "testdata/good"); len(got) > 0 {
		t.Fatalf("a standard project has problems:\n%s", strings.Join(got, "\n"))
	}
}

// Every check reports what it's for, beside clean cases that stay quiet: a
// commented variable, a variable no tag reads, a README in testdata, fakes in internal and external
// tests, an interface used only in its own foundation package, an asserted
// adapter, a kind in a mapping table, a binary with no test code and a fully
// documented operation.
func TestCheck_ReportsEachProblem(t *testing.T) {
	want := []string{
		"cmd/services/api/main.go: this binary carries test code: github.com/michaelolof/errx/leaky imports testing. Replace that library, or the package of ours that imports it (TEST-13)",
		"db/migrations/07_old.cql: a migration is named YYYYMMDDHHMMSS_<snake>.up.<sql|cql|json>, and runs forward only (DATA-11)",
		"db/migrations/20260101000000_init.up.sql: shares its timestamp with 20260101000000_again.up.sql: each migration has its own (DATA-11)",
		"db/migrations/20260102000000_drop.down.sql: a migration is named YYYYMMDDHHMMSS_<snake>.up.<sql|cql|json>, and runs forward only (DATA-11)",
		"db/migrations/20260103000000_Mixed.up.sql: a migration is named YYYYMMDDHHMMSS_<snake>.up.<sql|cql|json>, and runs forward only (DATA-11)",
		"docs/openapi/api.json:4: GET /v1/transfers: 2 fields have no description: query cursor, 200.data[].status (HTTP-4)",
		"docs/openapi/api.json:4: GET /v1/transfers: 2 scalar fields have no example: query cursor, 200.data[].status (HTTP-4)",
		"internal/architecture: the standard's analyzers and tools/archcheck now check these rules: delete internal/architecture and the arch task (TEST-13)",
		"internal/config/mongo.go:7: MONGO_DATABASE is read here, and .env.example doesn't list it (LAY-13)",
		"internal/core/payout/payout.go:5: Clock has the same methods as clock.Clock: declare it once, in internal/ports when several packages need it (ABS-1)",
		"internal/core/transfer/README.md: a README.md beside Go code goes stale: move what it says into the package's doc comment (LAY-11)",
		"internal/core/transfer/transfer.go:18: nothing implements Unused, not even a test fake: delete it until something does (ABS-1)",
		"internal/foundation/clock/clock.go:8: nothing implements Unwatched, not even a test fake: delete it until something does (ABS-1)",
		"internal/foundation/clock/clock.go:8: nothing uses Unwatched: delete it until something does (ABS-1)",
		"internal/foundation/errkinds/errkinds.go:8: nothing checks ErrKindConflict with errx.IsKind: a kind exists only for code that branches on it (ERR-5)",
		"internal/foundation/errkinds/errkinds.go:9: nothing outside errkinds uses ErrKindUnused: delete it (ERR-5)",
		"internal/foundation/errkinds/errkinds.go:10: nothing creates an error of kind ErrKindChecked, so checking for it never matches (ERR-5)",
		"internal/ports/ports.go:8: nothing outside internal/ports uses Unshared: a port is shared by several packages, so move it to its one consumer or delete it (ABS-1)",
		"internal/repository/transfers/repo.go:11: Repo is used as transfer.Store: declare var _ transfer.Store = (*Repo)(nil) beside it, so a change that breaks it fails here (ABS-2)",
	}

	got := run(t, "testdata/bad")
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("problems:\n%s\n\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// A module that doesn't build stops the run, so no check passes on a partial load.
func TestCheck_AModuleThatDoesntBuildStops(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "go.mod", "module example.com/broken\n\ngo 1.26\n")
	write(t, dir, "internal/core/x/x.go", "package x\n\nvar _ = undefined\n")

	if _, err := check(dir); err == nil || !strings.Contains(err.Error(), "doesn't build") {
		t.Fatalf("check returned %v, want a build failure", err)
	}
}
