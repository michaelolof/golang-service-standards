# 07 · Testing

This area defines which kinds of test exist and where each lives, how tests are gated, how assertions and test doubles are written, how each layer is tested, and how tests stay deterministic.

It builds on these earlier areas:
- [03](03-domain-and-abstractions.md): small consumer interfaces, `New(Deps)`, and injected clock and IDs.
- [04](04-errors.md): assert on kinds, and use stamp `0` in tests.
- [05](05-transport-and-runtime.md): handler constructors and `Handle(ctx, Job)`.
- [06](06-persistence.md): the shared paging, conflict and migrate-twice helpers.

**Paused: restructuring tests (TEST-14).** In an existing project, this document changes nothing about its tests. Existing tests are the guardrails during refactors. New tests follow the style already in their package. New projects follow this document from the start.

"Parked" marks a decision that is still open; see [decisions.md](decisions.md).

Without these rules, tests drift. Across three codebases with no test linter enabled, there were about 4,700 `assert.*` calls, 47 `time.Sleep` calls in tests, and 260 test files that reach real infrastructure with no build tag.

## Rules at a glance

| ID | Rule | Enforced by |
|---|---|---|
| TEST-1 | Each kind of test has one home and one build tag. A test with no tag uses no infrastructure. Each behaviour is tested once, at the lowest layer where it is real. | arch test, review |
| TEST-2 | Under its tag, missing infrastructure fails the test; it never skips. Nothing is gated by `testing.Short()` or environment variables. | forbidigo, arch test |
| TEST-3 | Run one docker-compose stack through Task, and isolate tests by namespace. Never reset or flush global state. | depguard, arch test |
| TEST-4 | Use testify `require` only, and compare whole values. | testifylint, arch test |
| TEST-5 | Test doubles are hand-written fakes named `fake…`. They are small working implementations that never embed the interface. | arch test |
| TEST-6 | A shared fake lives in `xtest` and passes the same conformance suite as the real adapter. | review |
| TEST-7 | Tables are for three or more data-only cases. Rows hold no functions and the loop body has no branches. Names follow fixed patterns. | arch test |
| TEST-8 | `t.Parallel()` is optional. Use it where a test shares nothing. | review |
| TEST-9 | Helpers take `t testing.TB`, call `t.Helper()`, fail with `require`, use `t.Context()` and register `t.Cleanup`. | thelper, usetesting |
| TEST-10 | Builders return valid values with fixed IDs and times. `testdata/` holds no Go code. | arch test |
| TEST-11 | Each layer has one testing recipe. | arch test, review |
| TEST-12 | Time, IDs and randomness are injected. Use `synctest` inside a process. Polling across processes must fail on timeout. | forbidigo, testifylint |
| TEST-13 | Architecture tests are table-driven, typed, and check that they inspected something. | review |
| TEST-14 | Restructuring tests is paused. Existing tests stay as they are during refactors, as the guardrails. | review |

---

## TEST-1 · One home and one tag per kind

**Rule.**

| Kind | Location | Build tag | Needs |
|---|---|---|---|
| unit | beside the code | none | nothing: fakes and `httptest.Server` only |
| integration (repositories, adapters, migrations) | beside the code, in `*_integration_test.go` | `integration` | the compose stack |
| flow (the composed app: several components through their real wiring) | `tests/flows` | `integration` | the compose stack |
| e2e | `tests/e2e`, which is its own Go module | `e2e` | a running deployment |
| third-party sandbox | `tests/contract` | `contract` | a vendor sandbox, never default CI |
| load | `tests/perf` | `perf` | the compose stack |

- A file carries `//go:build integration` **if and only if** its name ends in `_integration_test.go`, or it lives under `tests/flows`.
- The integration `TestMain` lives in `main_integration_test.go`.

**What to test where.** Not every function needs its own test, but every behaviour needs one, at the lowest layer where that behaviour is real:

| Behaviour | Tested by | Not by |
|---|---|---|
| SQL, CQL or Mongo queries; constraints, guards in `WHERE`, CTEs, indexes, migrations | an integration test beside the repository | a unit test with a mocked driver or fake SQL |
| Business rules, branches and error kinds in `domain` and core | unit tests with fakes | a flow test that goes through the database for each branch |
| Wiring, middleware order, auth, and the HTTP contract across components | flow tests in `tests/flows` | a handler unit test per route |

- Don't assert the same behaviour at two layers. A repository method whose only logic is its query has an integration test and no unit test.
- A unit test isn't redundant with an integration test when it covers branches that are expensive to reach through the database: each error kind, each guard, each edge case. It runs in milliseconds and points straight at the failing rule.

**Why.** "Untagged" must reliably mean "fast and needs no infrastructure". Without this rule, it stops being true. One codebase grew to 127 untagged files that reach a database. Another had no build tags at all, and its test command ran the whole suite with `-p 1`.

**Do:**

```go
//go:build integration

func TestMain(m *testing.M) {
	cfg, err := config.Parse(os.LookupEnv)
	// ...
	os.Exit(testharness.RunMain(m, testharness.RunMainOptions{Config: cfg, Limits: limits, Mongo: true}))
}
```

## TEST-2 · Fail, don't skip

**Rule.**
- Under its tag, missing infrastructure is a `t.Fatal`.
- Never gate tests with `testing.Short()` or environment variables.
- `t.Skip` is allowed only for an OS or architecture reason.

**Don't.** Whether 127 files run or silently pass depends on environment variables:

```go
func GetTestInfraOrSkip(t *testing.T) *Infra {
	t.Helper()
	infra, err := tryGetInfra(t)
	if err != nil {
		if strings.EqualFold(os.Getenv("TEST_INFRA_EXTERNAL"), "true") || os.Getenv("CI") != "" {
			t.Fatalf("required integration infrastructure failed: %v", err)
		}
		t.Skipf("skipping integration test: %v", err)
	}
	return infra
}
```

## TEST-3 · One stack, isolated by namespace

**Rule.**
- `task test:up` starts `docker-compose.test.yml`, and `.env.test` supplies the endpoints. Use no testcontainers.
- `internal/testutils/testharness` gives every test its own namespace:

| Store | Namespace |
|---|---|
| Mongo | one database per test |
| CockroachDB, Scylla | one database or keyspace per package (migrated with the production runner), plus unique tenant and org IDs per test (parked: see [decisions.md](decisions.md)) |
| Redis | a key prefix |
| Kafka | topics per test |

- Nothing calls a global `ResetState`, `DELETE FROM` every table, or `FLUSHDB`.

**Why.**
- `go test` runs each package in its own process. Containers started from test code therefore mean either N stacks or reuse hacks, such as a `composeInfraAlreadyRunning` check.
- Wiping every table before each test forces a project to run one package at a time (`-p 1`), and it races `t.Parallel()` tests.

**Do:**

```go
//   - One test owns one world; worlds never share logical resource names.
//   - Containers and provider clients are shared; mutable test data is not.
//   - No test calls a global database reset, Kafka-wide delete, Redis
//     FLUSH*, or object-store-wide purge.
//   - Correctness comes from unique namespaces. Cleanup is comprehensive
//     but is not trusted to prevent cross-test observation.
```

## TEST-4 · `require` only, whole values

**Rule.**
- Use testify's `require` package only.
- Compare whole values, expected first: `require.Equal(t, want, got)`. A single failure then prints the full diff.
- Assert error kinds as `require.True(t, errx.IsKind(err, K), "got %v", err)`. Never assert on message text.
- Every HTTP status assertion passes `resp.BodyString()` as its message.
- Use `require.ElementsMatch` for results that have no order.
- Banned: suites, testify/mock, go-cmp, gomock, and hand-rolled `if got != want { t.Fatalf }`.

**Why.** A test makes a non-fatal `assert.Len` or `assert.NotNil` call and then indexes into or dereferences the value. Two codebases had about 150 such sites, and each one panics instead of reporting the failure.

**Don't:**

```go
assert.Equal(t, 200, resp.Code())                                 // no body in the failure message
// ...
assert.Len(t, body.Data, 1)
assert.Equal(t, domain.TransferStatusFailed, body.Data[0].Status) // panics if Len failed
```

**Do:**

```go
require.Equal(t, http.StatusAccepted, resp.Code(), resp.BodyString())
require.Len(t, publisher.Published(), 1, "exactly one message must reach the publisher")
```

## TEST-5 · Fakes

**Rule.**

- **Naming.** Every hand-written double is a fake:
  - `fakeX` locally, where `X` is the name of the interface;
  - `xtest.FakeX` when shared.

  No stubs, spies, mocks, `recording…` types, or mocking frameworks.
- **Shape.** A fake is a small working implementation:
  - in-memory maps for stores;
  - a recorded-calls slice for outbound ports, read through an accessor that returns a copy;
  - one `<Method>Err` field per method, for injecting failures;
  - a `sync.Mutex`;
  - a `newFakeX(seed...)` constructor;
  - `var _ pkg.X = (*fakeX)(nil)`.
- **Never embed the interface, and never use function fields.** If a test needs different behaviour, add a field or write another fake.
- **Location.**
  - Used by one package: `fakes_test.go` in that package.
  - Used by several packages: `xtest`, beside the package that declares the interface.
  - Never in a production package.
- **Third-party HTTP providers** are faked with an in-process server (`httptest.Server` or [gofi-proxy-mocker](https://github.com/michaelolof/gofi-proxy-mocker)) in `adapters/<vendor>/<vendor>test`.

**Why.**
- Across three codebases, four fake designs were in use, under five names (`fake`, `stub`, `spy`, `mock`, `recording`).
- An embedded interface turns every method you didn't fake into a nil-pointer panic at runtime. This rule only works because ABS-1 keeps interfaces small.

**Don't:**

```go
type fakeWalletStore struct {
	domain.WalletRepository
	created []domain.CreateWalletParams
	byID    map[string]domain.Wallet
```

**Canonical** (the fake for ABS-2's `transfer.Store`):

```go
type fakeStore struct {
	mu              sync.Mutex
	byID            map[domain.TransferID]domain.Transfer
	updates         []domain.UpdateStatusArgs
	GetErr          error
	UpdateStatusErr error
}

var _ transfer.Store = (*fakeStore)(nil)

func newFakeStore(seed ...domain.Transfer) *fakeStore {
	f := &fakeStore{byID: map[domain.TransferID]domain.Transfer{}}
	for _, tr := range seed {
		f.byID[tr.ID] = tr
	}
	return f
}

func (f *fakeStore) Get(_ context.Context, _ domain.TenantID, id domain.TransferID) (domain.Transfer, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.GetErr != nil {
		return domain.Transfer{}, f.GetErr
	}
	tr, ok := f.byID[id]
	if !ok {
		return domain.Transfer{}, errx.NewKind(0, errkinds.ErrKindNotFound, "transfer not found")
	}
	return tr, nil
}

func (f *fakeStore) UpdateStatus(_ context.Context, args domain.UpdateStatusArgs) (domain.Transfer, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.UpdateStatusErr != nil {
		return domain.Transfer{}, f.UpdateStatusErr
	}
	f.updates = append(f.updates, args)
	f.byID[args.Transfer.ID] = args.Transfer
	return args.Transfer, nil
}

func (f *fakeStore) Updates() []domain.UpdateStatusArgs {
	f.mu.Lock()
	defer f.mu.Unlock()
	return slices.Clone(f.updates)
}
```

## TEST-6 · Shared fakes pass conformance

**Rule.**
- Every shared fake `xtest.FakeX` passes `xtest.RunXConformance(t, newX)`.
- The real adapter's integration test runs the same suite.

This keeps fakes honest as the real implementation changes.

**Do.** One suite runs against both the fake and the real Kafka publisher:

```go
func RunPublisherConformance(t *testing.T, newPublisher func(t *testing.T) stream.Publisher) {
	t.Helper()
	t.Run("a cancelled context never reports success", func(t *testing.T) { /* ... */ })
	// ...
}
```

## TEST-7 · Tables and names

**Rule.**
- **When to use a table:** when three or more cases run identical act-and-assert code and differ only in data. Otherwise write separate subtests or tests.
- **Row fields,** in this order:
  1. `name`;
  2. inputs, named after the parameters;
  3. `want` or `wantX`.

  Rows have no function-typed fields, and no `setup` or `mock` fields.
- **The loop body** doesn't branch. Error cases go in their own table, whose loop asserts one kind.
- **Naming:**
  - The loop variable is `tc`.
  - Top-level tests are `Test<Func>` or `Test<Type>_<Method>`, optionally followed by `_<Scenario>`.
  - Subtest names are lowercase phrases.
- **Shape:** arrange, blank line, **one** act, blank line, assert. Extract helpers once a test passes about 60 lines.

**Why.**
- Banning `t.Run` inside loops altogether is too blunt.
- A `wantErr bool` field proves only that *some* error happened.
- `tc := tc` copies have been unnecessary since Go 1.22.

**Canonical:**

```go
func TestParseAmount(t *testing.T) {
	t.Parallel()

	valid := []struct {
		name      string
		in        string
		wantMinor int64
	}{
		{name: "whole units", in: "10", wantMinor: 10_000_000},
		{name: "smallest unit", in: "0.000001", wantMinor: 1},
	}
	for _, tc := range valid {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			got, err := domain.ParseAmount(tc.in, domaintest.USDC)

			require.NoError(t, err)
			require.Equal(t, tc.wantMinor, got.Minor())
		})
	}

	invalid := []struct{ name, in string }{
		{name: "empty", in: ""},
		{name: "negative", in: "-1"},
		{name: "too precise", in: "0.0000001"},
	}
	for _, tc := range invalid {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()

			_, err := domain.ParseAmount(tc.in, domaintest.USDC)

			require.True(t, errx.IsKind(err, errkinds.ErrKindInvalidInput), "got %v", err)
		})
	}
}
```

## TEST-8 · Parallel where it costs nothing

**Rule.**
- `t.Parallel()` isn't required.
- Use it where a test shares nothing, such as a pure unit test.
- Don't convert existing tests to it (TEST-14).
- CI runs `-race -count=1 -shuffle=on`.

**Why.** Making every test parallel makes tests harder to set up, because each database test needs its own data. Converting an existing suite costs even more: about 380 tests across two codebases, for a speed gain nobody had asked for.

## TEST-9 · Helpers

**Rule.**
- A helper's signature is `func X(t testing.TB, …)`, and its first statement is `t.Helper()`.
- It fails with `require` and never returns an error.
- It takes no ctx parameter; it uses `t.Context()`.
- It registers teardown with `t.Cleanup` and never returns a cleanup function.
- `t.Context()` is cancelled *before* cleanups run, so cleanups use `context.WithoutCancel(t.Context())`.

**Why.**
- Without a fixed signature, helper parameters drift. One codebase had both `(ctx, t, …)` (23 helpers) and `(t, ctx, …)` (25).
- Without this rule, seeders take `t` *and* return an error.
- A helper that returns a cleanup function leaves callers able to forget to call it.

```go
// internal/testutils/dbtest/dbtest.go
func New(t testing.TB) *pgxpool.Pool {
	t.Helper()
	pool, err := pgxpool.New(t.Context(), testharness.From(t).SQLURL)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	return pool
}
```

## TEST-10 · Builders and fixtures

**Rule.**
- **Builders.** `xtest.<Type>(t)` returns a *valid* value with fixed IDs and fixed times derived from `xtest.T0`. The test then overrides fields directly.
  - Builders don't use functional options and don't call `time.Now()`.
  - Uniqueness in shared stores comes from the harness namespace, not from the wall clock (such as `UnixNano()`).
- **Seeding** goes through repository-level helpers, `xtest.Seed…(t, repo, v)`. Seed through the HTTP API only in e2e tests.
- **`testdata/`** holds only non-Go files. Load them with a typed loader (such as `fixture.LoadJSON[T](t, parts...)`). Golden files are refreshed with an `-update` flag.

**Why.** A package of builders inside a `testdata/` directory was imported by 114 files. `./...` skips `testdata` directories, so `go vet ./...` and golangci-lint never analyse that package.

```go
// internal/domain/domaintest/transfer.go
var T0 = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

// Fixed IDs, so whole-value comparisons work.
var (
	TransferID1 = domain.TransferID{1}
	TenantID1   = domain.TenantID{1}
)

// Transfer returns a valid initiated transfer; override fields in the test.
func Transfer(t testing.TB) domain.Transfer {
	t.Helper()
	amount, err := domain.NewAmount(1_000_000, USDC)
	require.NoError(t, err)
	return domain.Transfer{ID: TransferID1, TenantID: TenantID1, Amount: amount,
		Status: domain.TransferStatusInitiated, CreatedAt: T0}
}
```

## TEST-11 · How each layer is tested

| Layer | Recipe |
|---|---|
| `domain` | <ul><li>Pure tests, no fakes.</li><li>A valid table and an invalid table for each constructor and `Validate()`.</li><li>The transition-table test ranges over enumx `Values()` (DOM-7).</li><li>Fuzz parsers of untrusted input and assert a *property*, such as round-trip or idempotence. "Doesn't panic" is not enough.</li></ul> |
| `core` | <ul><li>Build the real `Service` with `New(Deps{…})`, with **every** dependency set:<ul><li>fakes;</li><li>`clocktest`;</li><li>`idstest`;</li><li>a noop meter, if the capability has metrics.</li></ul></li><li>Never pass `nil` with a "not reached" comment.</li><li>`cancel.go` is tested in `cancel_test.go`.</li><li>Assert on the result and on the fake's state.</li></ul> |
| `http` | <ul><li>Mount the handler variable on `apitest.NewRouter(t, d)`, which registers `d` as the router's `Deps` and uses the production body parser, error encoder and middleware.</li><li>Build `d` with `deps.New` from a real `Service` over fakes, or from fake repositories when the capability has no service.</li><li>Call `r.Test(...)` and decode the body.</li><li>Full route table plus database is tested once, in `tests/flows`. (parked: see [decisions.md](decisions.md))</li></ul> |
| `workers` | <ul><li>Call `Handle(t.Context(), job)` directly, with fakes.</li><li>Assert the resulting state and the returned error kind. Test the runtime's outcome mapping once, in its adapter.</li><li>Never start the queue runtime in a unit test. Integration tests drive `RunOnce`.</li></ul> |
| `repository` | <ul><li>Tagged `integration`, beside the repo, in an isolated namespace, migrated with the production runner.</li><li>Required: `AssertStablePaging`, `AssertConcurrentUpdateConflicts`, and the migrate-twice test.</li></ul> |
| redis | <ul><li>`miniredis`, a real Redis server that runs inside the test process, can be used in any test.</li></ul> |

**Don't:**

```go
return transfersvc.New(transfersvc.ServiceDependencies{
	Wallets: walletReader,
	Repo:    nil, // not reached by validation-failure tests
	Clock:   apptime.NewFrozenClock(time.Now()),
```

**Do.** The handler is mounted on a router with fakes:

```go
func TestRecord_PublishFails(t *testing.T) {
	authn := writerAuthenticator("order.*")
	publisher := streamtest.NewFakePublisher()
	publisher.PublishErr = context.DeadlineExceeded
	r := apitest.NewRouter(t, apitest.Deps(t, deps.Config{Authenticator: authn, Publisher: publisher}))

	resp, err := postEvent(t, r, producerToken, minimalEventBody())
	require.NoError(t, err)
	require.Equal(t, http.StatusServiceUnavailable, resp.Code(), resp.BodyString())
}
```

**Canonical handler test:**

```go
func TestCancel(t *testing.T) {
	t.Parallel()
	svc, err := transfer.New(transfer.Deps{
		Store:     newFakeStore(domaintest.Transfer(t)),
		Custodian: newFakeCustodian(),
		Clock:     clocktest.New(domaintest.T0),
		IDs:       idstest.Sequence(),
	})
	require.NoError(t, err)

	r := apitest.NewRouter(t, apitest.Deps(t, deps.Config{Transfers: svc}))
	r.Post("/v1/transfers/:transfer_id/cancel", transfers.CancelTransferHandler)

	resp, err := r.Test(gofi.TestOptions{Method: http.MethodPost, Path: "/v1/transfers/" + domaintest.TransferID1.String() + "/cancel", Headers: apitest.Auth(t)})

	require.NoError(t, err)
	require.Equal(t, http.StatusOK, resp.Code(), resp.BodyString())
	var body schemas.SuccessSchema[domain.Transfer]
	require.NoError(t, resp.BindBody(&body))
	require.Equal(t, domain.TransferStatusCancelled, body.Data.Status)
}
```

## TEST-12 · Determinism

**Rule.**
- **Time:** `clocktest.New(T0)` from `foundation/clock/clocktest`. It is guarded by a mutex and has `Set` and `Advance`. Every fixture time is derived from `T0`.
- **IDs:** `idstest.Sequence()` returns an `ids.Generator` that yields the same `[16]byte` values in the same order on every run, so whole-struct equality works.
- **Randomness:** injected, and seeded in tests.
- **Timers and concurrency inside one process** (limiters, backoff, worker loops, TTL caches): wrap the test in `synctest.Test`.
  - Inside it, `time.Sleep` uses virtual time and is exact.
  - "Let it initialise" sleeps become `synctest.Wait()`.
- **Work across processes** (Kafka, database, queue): drive step functions with a harness `RunUntil(ctx, deadline)` that **fails on timeout**. If polling is unavoidable, use `require.EventuallyWithT` with `*assert.CollectT`. `CollectT` is the only `assert` identifier allowed.
- **Forbidden:**
  - calling `require.X(t, …)` inside an `Eventually` condition or a goroutine;
  - a bare `time.Sleep` used to synchronise.

**Why.**
- Across three codebases there were three different fake clocks, and one lived in a production package.
- The same three used `time.Now()` more than 900 times in tests.
- `synctest` has been in the standard library since Go 1.25, so every project can use it.

**Don't.** After its last attempt this loop falls through silently, and the test goes on as if it had succeeded:

```go
for attempt := 0; attempt < 20; attempt++ {
	n, runErr := runner.RunOnce(context.Background(), stream.PollArgs{MaxRecords: 100})
	require.NoError(t, runErr)
	if n > 0 {
		break
	}
	time.Sleep(200 * time.Millisecond)
}
```

**Canonical:**

```go
func TestLimiter_EvictsIdleEntries(t *testing.T) {
	t.Parallel()
	synctest.Test(t, func(t *testing.T) {
		l := newShardedLimiters(1, 100, 5*time.Millisecond, 10, 10)
		reserveAndRelease(t, l, "k1")

		time.Sleep(10 * time.Millisecond) // virtual time: instant and exact
		reserveAndRelease(t, l, "k2")

		require.NotContains(t, l.shards[0].entries, "k1")
	})
}

require.EventuallyWithT(t, func(c *assert.CollectT) {
	got, err := repo.Get(t.Context(), tenant, id)
	require.NoError(c, err)
	require.Equal(c, domain.TransferStatusSettled, got.Status)
}, 10*time.Second, 50*time.Millisecond)
```

## TEST-13 · Architecture and meta tests

**Rule.** `internal/architecture` is table-driven and built on `golang.org/x/tools/go/packages` and `go/types`, not on string matching. It holds the arch-test rules from every area. For testing specifically, it checks:

1. **Test-code isolation.** The dependencies of `./cmd/...` include no `testutils`, `*test` or mock packages. `*test` packages are imported only from `_test.go` files.
2. **Tag honesty** (TEST-1). The `integration` tag appears on `_integration_test.go` files and the flow tests in `tests/flows`, and nowhere else, and untagged tests import no infrastructure drivers.
3. **No Go packages under `testdata/`.**
4. **Fake hygiene** (TEST-5): an assertion is present, no interface is embedded, and no banned names are used.
5. **Table and name rules** (TEST-7).
6. **`time.Sleep` only inside `synctest`** (TEST-12).
7. **Every meta test checks that it inspected at least one target,** so a broken package walk fails instead of passing vacuously.

**Do.** A self-guarding meta test:

```go
for _, binary := range []string{"api", "provider-webhook", "worker", "admin-api"} {
	// ...
	require.NotContains(t, deps, mockPkg, "%s must not import the provider mock, directly or indirectly", binary)
}
// Guard the check itself: the provider-mock binary does import the mock, so
// a broken walk that finds nothing would fail here instead of passing above.
require.Contains(t, moduleDeps(t, root, modulePath+"/cmd/tools/provider-mock"), mockPkg)
```

**Linters** (not applied while restructuring is paused, because the lint config skips test code):
- testifylint, with all checks enabled;
- thelper, usetesting and copyloopvar;
- testpackage: use package `x_test`, and package `x` only in `*_internal_test.go`;
- depguard: suite, mock, go-cmp, gomock and testcontainers;
- forbidigo: `testing.Short`, and `time.Now` in `*test` packages.

Benchmarks use `b.Loop()`.

## TEST-14 · Existing tests are the guardrails

**Rule.**
- **Restructuring tests is paused.** That covers moving or renaming tests, changing assertion style, and changing helpers, harnesses or test infrastructure.
- **Existing tests stay as they are during a refactor.** If they still pass, the behaviour hasn't changed.
- **If a refactor changes a signature,** change the test only enough to compile. Don't change what it asserts. If an assertion has to change, that's a behaviour change: stop and ask (COL-4, COL-11).
- **New tests** follow the style already in their package. New behaviour still gets its acceptance tests first (COL-4).
- **New projects** follow this document from the start.

**Why.** It's risky to change tests during a major refactor. If the code and its tests change together, nothing shows that the behaviour stayed the same.
