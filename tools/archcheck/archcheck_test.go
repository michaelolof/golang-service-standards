package main

import (
	"go/build/constraint"
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

// A standard project passes, so every check has a clean case to stay quiet on:
// among them a table of rows in a flow file, bootstrap.go seeding the first
// admin, a harness that reads with SQL, binds a value that reads like a write
// and creates its database, promises that agree with the flows and the API
// description in other valid Markdown (no outer pipes, an escaped pipe, a
// nested item, a numbered item, closing hashes, a fenced example), a harness
// step only a flow calls, a helper only an interface method calls, a helper only an
// example with no output calls, a helper only a _windows_test.go file calls,
// and a folder go.mod's ignore directive leaves out.
func TestCheck_AStandardProjectPasses(t *testing.T) {
	if got := run(t, "testdata/good"); len(got) > 0 {
		t.Fatalf("a standard project has problems:\n%s", strings.Join(got, "\n"))
	}
}

// Every check reports what it's for, beside clean cases that stay quiet: a
// commented variable, a variable no tag reads, a README in testdata, fakes in
// internal and external tests, an interface used only in its own foundation
// package, the job port core uses, an asserted adapter, a kind in a mapping
// table, a binary with no test code, a fully documented operation, a file
// under internal/http that declares no handler and has no test file, a
// repository read in a flow, and a fake's method that an interface needs and
// no test calls. The bad module's flows include two with the same name.
func TestCheck_ReportsEachProblem(t *testing.T) {
	want := []string{
		"cmd/services/api/main.go: this binary carries test code: github.com/michaelolof/errx/leaky imports testing. Replace that library, or the package of ours that imports it (TEST-13)",
		"db/migrations/07_old.cql: a migration is named YYYYMMDDHHMMSS_<snake>.up.<sql|cql|json>, and runs forward only (DATA-11)",
		"db/migrations/20260101000000_init.up.sql: shares its timestamp with 20260101000000_again.up.sql: each migration has its own (DATA-11)",
		"db/migrations/20260102000000_drop.down.sql: a migration is named YYYYMMDDHHMMSS_<snake>.up.<sql|cql|json>, and runs forward only (DATA-11)",
		"db/migrations/20260103000000_Mixed.up.sql: a migration is named YYYYMMDDHHMMSS_<snake>.up.<sql|cql|json>, and runs forward only (DATA-11)",
		"docs/openapi/api.json:4: GET /v1/transfers is in no area's Operations line in docs/promises: add it to the area that covers it (TEST-18)",
		"docs/openapi/api.json:4: GET /v1/transfers: 2 fields have no description: query cursor, 200.data[].status (HTTP-4)",
		"docs/openapi/api.json:4: GET /v1/transfers: 2 scalar fields have no example: query cursor, 200.data[].status (HTTP-4)",
		"docs/promises/payments.md:10: PAY-2 names TestPayment_ARefusedPaymentSaysWhy, and no flow in test/flows has that name: write the flow, or fix the name (TEST-18)",
		"docs/promises/payments.md:11: PAY-3 names TestRepo_StoresATransfer, which isn't a flow in test/flows: a promise is proved by a flow, as the caller meets it (TEST-18)",
		"docs/promises/payments.md:12: PAY-4 names no test in Proved by: name the flow that proves it (TEST-18)",
		"docs/promises/payments.md:13: PAY-1 is also the ID of the promise at docs/promises/payments.md:9: give each promise its own (TEST-18)",
		"docs/promises/payments.md:17: a \"Not reachable from outside\" entry says why no caller can reach the promise, and names the lower test that covers it (TEST-18)",
		"docs/promises/payments.md:17: the \"Not reachable from outside\" entry names TestChargeOrder_MarksTheOrderFailed, and no test in the module has that name (TEST-18)",
		"docs/promises/payments.md:18: a \"Not reachable from outside\" entry says why no caller can reach the promise, and names the lower test that covers it (TEST-18)",
		"internal/adapters/queue/sandbox_test.go:3: a contract test lives in test/contract: move this file there (TEST-1)",
		"internal/architecture: the standard's analyzers and tools/archcheck now check these rules: delete internal/architecture and the arch task (TEST-13)",
		"internal/config/mongo.go:7: MONGO_DATABASE is read here, and .env.example doesn't list it (LAY-13)",
		"internal/core/payout/payout.go:5: Clock has the same methods as clock.Clock: declare it once, in internal/ports when several packages need it (ABS-1)",
		"internal/core/transfer/README.md: a README.md beside Go code goes stale: move what it says into the package's doc comment (LAY-11)",
		"internal/core/transfer/flow_test.go:1: a flow lives in test/flows: move this file there (TEST-1)",
		"internal/core/transfer/transfer.go:19: nothing implements Unused, not even a test fake: delete it until something does (ABS-1)",
		"internal/core/transfer/transfer_test.go:10: newUnusedFixture is test code nothing calls: delete it (TEST-19)",
		"internal/domain/domaintest/transfer.go:7: Transfer is test code nothing calls: delete it (TEST-19)",
		"internal/foundation/clock/clock.go:8: nothing implements Unwatched, not even a test fake: delete it until something does (ABS-1)",
		"internal/foundation/clock/clock.go:8: nothing uses Unwatched: delete it until something does (ABS-1)",
		"internal/foundation/errkinds/errkinds.go:8: nothing checks ErrKindConflict with errx.IsKind: a kind exists only for code that branches on it (ERR-5)",
		"internal/foundation/errkinds/errkinds.go:9: nothing outside errkinds uses ErrKindUnused: delete it (ERR-5)",
		"internal/foundation/errkinds/errkinds.go:10: nothing creates an error of kind ErrKindChecked, so checking for it never matches (ERR-5)",
		"internal/http/api/orders/cancel.go:7: CancelOrderHandler has no test file: add cancel_test.go beside it, with the handler's tests (TEST-15)",
		"internal/jobs/jobs.go:15: nothing outside internal/jobs uses Claimer: jobs holds the contract other packages share, so delete it until one does (ABS-1, JOB-1)",
		"internal/ports/ports.go:8: nothing outside internal/ports uses Unshared: a port is shared by several packages, so move it to its one consumer or delete it (ABS-1)",
		"internal/repository/transfers/get_test.go:1: only a file named …_integration_test.go has //go:build integration: rename it, or take the tag off shared test code, which every kind builds (TEST-1)",
		"internal/repository/transfers/repo.go:13: Repo is used as transfer.Store: declare var _ transfer.Store = (*Repo)(nil) beside it, so a change that breaks it fails here (ABS-2)",
		"internal/repository/transfers/repo_integration_test.go: every _integration_test.go file has //go:build integration, so only task test:integration builds it (TEST-1)",
		"internal/testutils/testharness/world.go:24: pgxpool.Pool.Exec writes rows: the harness runs no SQL that writes, so set up through the API, the queued jobs and the vendor mock's webhooks, and seed only an identity no API can create, in internal/testutils/testharness/bootstrap.go (TEST-16)",
		"internal/testutils/testharness/world.go:28: World.CountOrders is test code nothing calls: delete it (TEST-19)",
		"test/contract/provider_test.go: every file in test/contract has //go:build contract, so only task test:contract reaches the vendor (TEST-1)",
		"test/flows/orders/main_integration_test.go: every file in test/flows has //go:build flow, so only task test:flows builds a flow (TEST-1)",
		"test/flows/orders/main_integration_test.go: rename it: a flow package's TestMain is in main_test.go, and no file in test/flows ends in _integration_test.go (TEST-1)",
		"test/flows/orders/place_test.go: every file in test/flows has //go:build flow, so only task test:flows builds a flow (TEST-1)",
		"test/flows/orders/place_test.go:6: TestPlaceOrder_AnOperatorSeesTheOrder proves no promise: name it in its area's file in docs/promises, or delete it (TEST-18)",
		"test/flows/orders/place_test.go:12: placeOrder is a helper in a flow file: move it into the harness, unchanged, and call it from there (TEST-19)",
		"test/flows/orders/place_test.go:16: steps.pay is a helper in a flow file: move it into the harness, unchanged, and call it from there (TEST-19)",
		"test/flows/payments/settle_test.go:8: imports example.com/bad/internal/adapters/payments/paymentstest, which stands in for the vendor mock: run against the mock, and set up through the API, the queued jobs and the vendor mock's webhooks (TEST-16)",
		"test/flows/payments/settle_test.go:10: imports the seed package example.com/bad/internal/repository/transfers/transferstest: set up through the API, the queued jobs and the vendor mock's webhooks (TEST-16)",
		"test/flows/payments/settle_test.go:23: transfers.Repo.MarkSettled writes through the repository: set up through the API, the queued jobs and the vendor mock's webhooks (TEST-16)",
		"test/flows/payments/settle_test.go:25: pgxpool.Pool.QueryRow reaches the database: a flow runs no SQL, even to read. Read storage through a harness reader, and set up through the API, the queued jobs and the vendor mock's webhooks (TEST-16)",
		"test/flows/payments/settle_test.go:31: TestPlaceOrder_AnOperatorSeesTheOrder is also a flow at test/flows/orders/place_test.go:6: a promise names its flow by name, so give each flow its own (TEST-18)",
	}

	got := run(t, "testdata/bad")
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("problems:\n%s\n\nwant:\n%s", strings.Join(got, "\n"), strings.Join(want, "\n"))
	}
}

// A module with flows and no promise files is told to write them, once, not
// once per flow.
func TestCheck_AModuleWithFlowsAndNoPromisesIsToldToWriteThem(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "go.mod", "module example.com/nopromises\n\ngo 1.26\n")
	write(t, dir, "test/flows/orders/place_test.go", "//go:build flow\n\npackage orders_test\n\nimport \"testing\"\n\nfunc TestOrder_IsPlaced(t *testing.T) {}\n\nfunc TestOrder_IsPaid(t *testing.T) {}\n")

	want := "docs/promises: missing: write one file per area, as .standards/templates/promises.md shows, listing every operation in docs/openapi and naming the flow that proves each promise (TEST-18)"
	if got := run(t, dir); strings.Join(got, "\n") != want {
		t.Fatalf("problems:\n%s\n\nwant:\n%s", strings.Join(got, "\n"), want)
	}
}

// A statement writes rows when it starts with a write, or holds one in a WITH
// or a batch. A read, a change to the schema, or a statement built at run time,
// which has no text here, doesn't.
func TestIsWriteStatement_ReadsWhatTheStatementDoes(t *testing.T) {
	cases := []struct {
		name string
		text string
		want bool
	}{
		{name: "an insert", text: "INSERT INTO orders (id) VALUES ($1)", want: true},
		{name: "an update after a comment", text: "-- mark it paid\n  update orders SET status = $1", want: true},
		{name: "a write inside a WITH", text: "WITH moved AS (DELETE FROM orders RETURNING id) SELECT count(*) FROM moved", want: true},
		{name: "a CQL batch", text: "BEGIN BATCH INSERT INTO orders (id) VALUES (?) APPLY BATCH", want: true},
		{name: "a write split across lines", text: "DELETE\nFROM orders WHERE id = $1", want: true},
		{name: "a write after a tab", text: "UPDATE\t\torders SET status = $1", want: true},
		{name: "an update with an alias inside a WITH", text: "WITH due AS (SELECT id FROM orders) UPDATE orders AS o SET status = 'PAID' FROM due WHERE o.id = due.id", want: true},
		{name: "a read", text: "SELECT count(*) FROM orders", want: false},
		{name: "a read that locks its rows", text: "WITH next AS (SELECT id FROM jobs FOR UPDATE SKIP LOCKED) SELECT id FROM next", want: false},
		{name: "a read with a write in a literal", text: "WITH ops AS (SELECT id FROM audit WHERE op = 'DELETE FROM') SELECT count(*) FROM ops", want: false},
		{name: "a read with a write in a comment", text: "WITH kept AS (SELECT id FROM orders) -- never delete from kept\nSELECT count(*) FROM kept", want: false},
		{name: "a read inside a WITH", text: "WITH recent AS (SELECT id FROM orders) SELECT count(*) FROM recent", want: false},
		{name: "a change to the schema", text: "CREATE DATABASE flows_orders", want: false},
		{name: "no text", text: "", want: false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := isWriteStatement(tc.text); got != tc.want {
				t.Fatalf("isWriteStatement(%q) = %v, want %v", tc.text, got, tc.want)
			}
		})
	}
}

// A module whose dependency doesn't build stops the run, though no package of
// the module reports an error of its own. Here the broken file is in another
// module, and builds only with a kind's tag, so go build alone passes. Before,
// the call graph met the ill-typed package and archcheck crashed.
func TestCheck_AModuleWhoseDependencyDoesntBuildStops(t *testing.T) {
	dir := t.TempDir()
	write(t, dir, "go.mod", "module example.com/brokendep\n\ngo 1.26\n\nrequire example.com/dep v0.0.0\n\nreplace example.com/dep => ./third_party/dep\n")
	write(t, dir, "internal/core/x/x.go", "package x\n\nimport _ \"example.com/dep\"\n")
	write(t, dir, "internal/core/x/x_test.go", "package x\n\nimport \"testing\"\n\nfunc TestX(t *testing.T) {}\n")
	write(t, dir, "third_party/dep/go.mod", "module example.com/dep\n\ngo 1.26\n")
	write(t, dir, "third_party/dep/dep.go", "package dep\n")
	write(t, dir, "third_party/dep/load.go", "//go:build perf\n\npackage dep\n\nvar _ int = \"not an int\"\n")

	if _, err := check(dir); err == nil || !strings.Contains(err.Error(), "doesn't build") {
		t.Fatalf("check returned %v, want a build failure", err)
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

// A file needs a tag when no setting of its other tags builds it without
// that one.
func TestRequires_ReadsTheWholeBuildExpression(t *testing.T) {
	cases := []struct {
		name string
		line string
		want bool
	}{
		{name: "the tag alone", line: "//go:build contract", want: true},
		{name: "the tag and another", line: "//go:build contract && linux", want: true},
		{name: "the tag or another", line: "//go:build contract || linux", want: false},
		{name: "the tag negated", line: "//go:build !contract", want: false},
		{name: "another tag only", line: "//go:build integration", want: false},
		{name: "the tag in a group", line: "//go:build (contract || e2e) && !linux && contract", want: true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			expr, err := constraint.Parse(tc.line)
			if err != nil {
				t.Fatal(err)
			}

			if got := requires(expr, "contract"); got != tc.want {
				t.Fatalf("requires(%q, contract) = %v, want %v", tc.line, got, tc.want)
			}
		})
	}

	if requires(nil, "contract") {
		t.Fatal("a file with no build line requires no tag")
	}
}

// The template a project copies reads the way the check reads a promise file:
// its four operations, its one promise and the test that proves it, and its
// "Not reachable from outside" entry, with a reason and a test.
func TestReadPromiseFile_ReadsTheTemplate(t *testing.T) {
	f, err := readPromiseFile("templates/promises.md", "../../templates/promises.md")
	if err != nil {
		t.Fatal(err)
	}

	wantOps := []string{"POST /api/v1/orders", "GET /api/v1/orders/{order_id}", "POST /api/v1/orders/{order_id}/cancel", "GET /admin/orders/{order_id}"}
	if strings.Join(f.operations, ", ") != strings.Join(wantOps, ", ") {
		t.Fatalf("operations = %q, want %q", f.operations, wantOps)
	}

	if len(f.promises) != 1 || f.promises[0].id != "ORD-1" || strings.Join(f.promises[0].tests, ",") != "TestOrder_EachKindOfCustomerPlacesAnOrderAndPaysIt" {
		t.Fatalf("promises = %+v, want ORD-1 proved by TestOrder_EachKindOfCustomerPlacesAnOrderAndPaysIt", f.promises)
	}

	if len(f.entries) != 1 {
		t.Fatalf("entries = %+v, want one", f.entries)
	}

	if !hasReason(f.entries[0].text) || len(testsIn(f.entries[0].text)) != 1 {
		t.Fatalf("the %q entry %q has no reason or no test", notReachable, f.entries[0].text)
	}
}
