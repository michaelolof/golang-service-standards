# 07 · Testing

This area defines which kinds of test exist, where each lives and which command runs it, how tests are gated, how assertions and test doubles are written, how each layer is tested, how handler, flow and contract tests are written, what callers are promised and how each promise is proved, how test helpers are shaped, and how tests stay deterministic.

It builds on these earlier areas:
- [03](03-domain-and-abstractions.md): small consumer interfaces, `New(Deps)`, and injected clock and IDs.
- [04](04-errors.md): assert on kinds, and use stamp `0` in tests.
- [05](05-transport-and-runtime.md): handler constructors, the `routes` package, and `Handle(ctx, Job)`.
- [06](06-persistence.md): the shared paging, conflict and migrate-twice helpers.

**New tests follow this chapter, in every project.** Restructuring existing tests waits until the engineer lifts the pause for that project (TEST-14). Until then, existing tests are the guardrails during refactors.

Without these rules, tests drift. Across three codebases with no test linter enabled, there were about 4,700 `assert.*` calls, 47 `time.Sleep` calls in tests, and 260 test files that reach real infrastructure with no build tag.

With them, tests change. One service applied this chapter in full, and one script counted its tests before and after:

| What | Before | After |
|---|---|---|
| Tests of the whole app against the database | 523 | 163 |
| Unit tests | 900, in 6 to 11 s | 2,408, in 23 s |
| Integration tests, flows included | 2,032 | 3,357 |
| Skips for a missing stack | 128 | 0 |
| Database resets (`ResetState`, `FLUSHDB`) | 175 | 0 |
| `assert` calls | 2,802 | 0 |
| `time.Sleep` | 19 | 6, each the behaviour under test |
| Doubles named stub, spy, mock or recording | 14 | 0 |

## Rules at a glance

| ID | Rule | Enforced by |
|---|---|---|
| TEST-1 | Each kind of test has one home, one build tag, one recipe and one task, and no task runs another kind's tests. A test with no tag uses no infrastructure. Each behaviour is tested once, at the lowest layer where it is real. A promise to a caller is the one exception: a flow proves it too. | `tools/archcheck` (each kind's tag, and the file names in `test/flows`), review |
| TEST-2 | Under its tag, missing infrastructure fails the test; it never skips. Nothing is gated by `testing.Short()` or environment variables. | forbidigo, review |
| TEST-3 | Run one test stack through Task. Each integration package gets its own database, and each test its own tenant. Never reset or flush global state. | depguard, review |
| TEST-4 | Use testify `require` only, and compare whole values. Assert an error's kind, and its text only when a person reads it. | testifylint, depguard |
| TEST-5 | Test doubles are hand-written fakes named `fake…`. They are small working implementations that never embed the interface. No test writes its own double for something a shared fake stands in for. | review |
| TEST-6 | Shared fakes come in three levels: boundary fakes, repository fakes and capability builders. Each boundary and repository fake passes the same conformance suite as the real implementation. A shared fake starts set up the way production is, and a parity test beside the real implementation fails when they differ. | the conformance suites, the parity tests, review |
| TEST-7 | Tables are for three or more data-only cases. Rows hold no functions and the loop body has no branches. Names say what the application does. | review |
| TEST-8 | `t.Parallel()` is optional. Use it where a test shares nothing. | review |
| TEST-9 | Helpers take `tb testing.TB`, call `tb.Helper()`, fail with `require`, use `tb.Context()` and register `tb.Cleanup`. | thelper, usetesting |
| TEST-10 | Builders return valid values with fixed IDs and times. `testdata/` holds no Go code. | review |
| TEST-11 | Each layer has one testing recipe. | review |
| TEST-12 | Time, IDs and randomness are injected. Use `synctest` inside a process. Polling across processes must fail on timeout. | forbidigo, testifylint |
| TEST-13 | Projects keep no `internal/architecture`, and no guard or check of their own. The standard's analyzers and `tools/archcheck` hold the structural checks, and each proves with fixtures that it reads code. Test code stays in tests. | `layers`, `tools/archcheck` (including the test checks of TEST-1, TEST-15, TEST-16, TEST-18 and TEST-19), review |
| TEST-14 | New tests follow these testing rules in every project. Restructuring existing tests is paused until the engineer lifts the pause for the project. Until then, existing tests stay as they are, as the guardrails. | review |
| TEST-15 | Every handler has a test file beside it. Requests go through the production route table and its auth to the real service over fakes. | `tools/archcheck` (the test file), review |
| TEST-16 | A flow proves one promise as its caller meets it. It signs in as a caller can, goes through the production servers, and sets up through the API, the queued jobs and the vendor mock. It states every choice, and checks only what the caller sees. | `tools/archcheck` (setup through boundaries, flow files, promises), review |
| TEST-17 | Adapters are tested against an in-process vendor mock that enforces the vendor's documented contract. A contract test, run by hand, checks the vendor itself. | review |
| TEST-18 | Every promise to a caller is written down, and a flow proves each one. A test's expected values come from the product, never from the code under test. A test that disagrees with the code is a finding. | `tools/archcheck` (promises, flows and operations agree), review |
| TEST-19 | Every shared test helper does one thing. Tests compose them. A test never has its own copy of a step, a body, a reader or a check, and no test function is left uncalled. | `tools/archcheck` (flow files, uncalled test code), review |

---

## TEST-1 · One home, one tag, one recipe and one command per kind

**Rule.** Each kind of test has one home, one tag, one recipe and one task. The first five kinds test the service's own code:

| Kind | Tests | Lives in | Tag | Needs | Run by |
|---|---|---|---|---|---|
| unit | domain rules; every business branch in core; each job handler; the HTTP plumbing; each shared fake against its conformance suite, and its parity test (TEST-6) | beside the code | none | nothing | `task test` |
| handler | each HTTP handler, through the production route table and its auth, over fakes (TEST-15) | beside the handler, one test file per handler file | none | nothing | `task test` |
| integration | SQL; adapters against the test stack or a vendor mock (TEST-17); each real repository and adapter against the same conformance suite as its fake (TEST-6); machinery no caller can reach, beside its job handler or repository | beside the code, `*_integration_test.go` | `integration` | the test stack | `task test:integration` |
| flow | each promise to a caller, as the caller meets it, and what needs several real parts together (TEST-16, TEST-18) | `test/flows/<area>` | `flow` | the test stack | `task test:flows` |
| contract | that a vendor still behaves the way its adapter assumes (TEST-17) | `test/contract` | `contract` | the vendor's sandbox and credentials | `task test:contract`, by hand |
| e2e | a running deployment, from outside | `test/e2e`, which is its own Go module | `e2e` | a running deployment | `task test:e2e` |
| load | the service under load | `test/perf` | `perf` | the test stack | `task test:perf`, by hand |

- A handler test is a unit test: no tag, no infrastructure. It has its own row because it has its own recipe.
- The test stack is the docker compose stack that `task test:up` starts (TEST-3).
- **Each tag matches one home or one name:**
  - A file carries `//go:build integration` if and only if its name ends in `_integration_test.go`. An integration package's `TestMain` is in `main_integration_test.go`.
  - A file carries `//go:build flow` if and only if it lives under `test/flows`. No file there ends in `_integration_test.go`. A flow package's `TestMain` is in `main_test.go`.
  - A file carries `//go:build contract` if and only if it lives under `test/contract`.
- **Shared test code carries no tag,** so every kind builds it. That's the harness, everything else under `internal/testutils`, and the `xtest`, `<aggregate>test` and `<capability>test` packages. TEST-13 already keeps them out of production.
- **No task runs another kind's tests.**
  - `task test:integration` runs only the packages that have integration files. It asks `tools/testpackages` which packages gain a test file under the tag, so a package with only unit tests isn't rebuilt and run again under it.
  - A package's own unit tests still run beside its integration tests, because a tag adds files and can't remove them. They take milliseconds.
  - `task test:all` runs unit tests, then integration tests, then flows, on a fresh stack (TEST-3).
  - `task test:build` compiles every tagged test without running it, and `task check` runs it. So a change that breaks an integration test, a flow or a contract test fails the check, not the next full run.
- **Editors need the tags too.** An editor that isn't given them reports every tagged file as outside the build, and checks none of it. In VS Code: `"gopls": {"build.buildFlags": ["-tags=integration,flow,contract,perf"]}`.

**What to test where.** Not every function needs its own test, but every behaviour needs one, at the lowest layer where that behaviour is real:

| A behaviour that depends on | Is tested | Not |
|---|---|---|
| a business rule | in core, a unit test over the capability's fakes | a flow through the database for each branch |
| SQL: queries, guards in `WHERE`, CTEs, constraints, paging, migrations | in the repository, an integration test | a unit test with a mocked driver |
| one handler: its status and body, its auth, role or scope, its binding, its tenant isolation | beside the handler, a handler test (TEST-15) | a flow per route |
| one job handler | beside it, a unit test over fakes | a flow that starts the queue |
| machinery no caller can reach, such as a sweep's query | beside its job handler or repository, an integration test, where seeding is allowed | a flow that seeds the state |
| an adapter's requests and replies | an integration test against the test stack or the vendor mock (TEST-17) | a unit test with an HTTP stub written in the test |
| a promise to a caller | a flow, as the caller meets it (TEST-16, TEST-18), as well as the lower test of each rule it relies on | only the lower tests |
| several real parts together | a flow (TEST-16) | |
| the vendor's real behaviour | a contract test (TEST-17) | default CI |

- **A test that checks two things is split.** Take "placing an order records the customer's usage". The rule, that an approved customer is required, is a core test. The row, that the usage is written in the same statement, is a repository test.
- **A flow that repeats a lower test is removed,** unless it proves a promise. When a project moves or removes a test, it records the test that covers the behaviour now ([10-adoption](10-adoption.md#lifting-test-14)).
- Don't assert the same behaviour at two layers. A repository method whose only logic is its query has an integration test and no unit test. The one exception is a promise to a caller. It's also proved by a flow, even when a lower test covers the same branch (TEST-18).
- A unit test isn't redundant with an integration test when it covers branches that are expensive to reach through the database: each error kind, each guard, each edge case. It runs in milliseconds and points straight at the failing rule.

**Why.**
- "Untagged" must reliably mean "fast and needs no infrastructure". Without this rule, it stops being true. One codebase grew to 127 untagged files that reach a database. Another had no build tags at all, and its test command ran the whole suite with `-p 1`.
- Without a recipe for each kind, one rule gets tested at two layers. In one service, about 490 of its 523 tests of the whole app sat beside handlers and job handlers. "An order with one item refuses a second item" had a core test. It also had a flow that built a customer, an account and an order in the database to check the same thing. A rule tested twice costs time, and when the slow copy fails, it points at nothing.
- That service moved 360 of those tests down to unit tests over fakes, handler tests among them. Its unit suite went from 900 tests to 2,408, and still runs in 23 seconds.
- **With one tag for two kinds, no command could run one of them.** This rule used to give flows the `integration` tag. So anything that built integration tests built flows too. In all three services that follow the standard, `task test:integration` ran both, and the template's task said so. None could run its integration tests without its flows, except by filtering a list of packages by hand.
- **A tag adds files, and removes none.** So `go test -tags integration ./...` also rebuilt and ran every unit test in every package, under `-race`. One service listed only the packages that have integration files. Its run went from about 3,800 tests in 166 packages, in about 1,070 s, to 1,380 tests in 56 packages, in about 745 s, with more flows than before. Its unit tests run on their own in about 13 s.
- **Each kind fails for a different reason.** A unit test points at a rule, an integration test at a query, and a flow at a promise. Each needs something different: nothing, the test stack, or the whole app on the stack. An engineer who changed a query wants that repository's integration tests, not a twelve-minute run that also builds every flow.
- **Nothing compiled the tagged tests until they ran.** `task test` builds no tagged file, and `task check` ran no tagged test. So a refactor could break an integration test, a flow or a contract test, and nothing showed it until the next full run. One service vetted every tag in a task of its own. The template had none.

**Don't.** One task for two kinds, over every package. It rebuilds every unit test under the tag, and an engineer can't run integration tests without the flows:

```yaml
test:integration:
  desc: Integration tests and flows against the test stack
  cmds:
    - go test -tags integration -race -count=1 ./...
```

**Do.** An integration package's `TestMain`, and a flow package's:

```go
//go:build integration

// internal/repository/orders/main_integration_test.go

package orders_test

// TestMain gives the package its own database (TEST-3).
func TestMain(m *testing.M) { os.Exit(testharness.RunMain(m)) }
```

```go
//go:build flow

// test/flows/orders/main_test.go

package orders_test

// TestMain gives the flow package its own database, like any integration package (TEST-3).
func TestMain(m *testing.M) { os.Exit(testharness.RunMain(m)) }
```

**Do.** The tasks, from [templates/Taskfile.yml](templates/Taskfile.yml). Each runs only its kind:

```yaml
test:integration:
  dotenv: [.env.test]
  deps: [test:up]
  vars:
    PACKAGES: {sh: go run -C .standards/tools/testpackages . -tags integration "$PWD"}
  cmds:
    - go test -tags integration -race -count=1 -shuffle=on {{if .CLI_ARGS}}{{.CLI_ARGS}}{{else}}{{.PACKAGES}}{{end}}
test:flows:
  dotenv: [.env.test]
  deps: [test:up]
  cmds:
    - go test -tags flow -race -count=1 -shuffle=on {{if .CLI_ARGS}}{{.CLI_ARGS}}{{else}}./test/flows/...{{end}}
test:build:
  cmds:
    - go vet -tags integration,flow,contract,perf ./...
```

**Enforced by** `tools/archcheck`: every file in `test/contract` has the `contract` tag and no file elsewhere has it; every file in `test/flows` has the `flow` tag and no file elsewhere has it; no file in `test/flows` ends in `_integration_test.go`; and a file has the `integration` tag exactly when its name ends in `_integration_test.go`. `task test:build` compiles every tagged test. What is tested where is for review.

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

## TEST-3 · One stack, one database per package, one tenant per test

**Rule.**
- `task test:up` starts the test stack, `docker-compose.test.yml`. `.env.test` holds its endpoints, and `task test:integration` and `task test:flows` each load them. Use no testcontainers.
- **One database per package.** Each integration package gets its own database, or keyspace, and so does each flow package. Its `TestMain` calls the same harness function, which:
  1. drops anything a killed run left;
  2. creates the package's database;
  3. migrates it with the production migration runner;
  4. runs the tests;
  5. drops the database.
- **One tenant per test.** Inside a package, every test makes its own tenant and org, and its own unique references.
  - Nothing deletes rows between tests.
  - A test that lists or sweeps looks only at the rows it made.
- **One copy of a package at a time.** Each package has a fixed database name, so two copies of one package would drop each other's database. Different packages run in parallel.
- **A full run is `task test:all`,** which stops the stack first, so it starts fresh. Databases created and dropped over runs in a row slow a database node down, until a package's migration times out before any test runs. One package, or a few, can run on the same stack as often as needed.
- Other stores are kept apart per test:

| Store | Namespace |
|---|---|
| Redis | a key prefix |
| Kafka | topics per test |

- Nothing calls a global `ResetState`, `DELETE FROM` every table, or `FLUSHDB`.

**Why.**
- `go test` runs each package in its own process. Containers started from test code therefore mean either N stacks or reuse hacks, such as a `composeInfraAlreadyRunning` check.
- Wiping every table before each test forces a project to run one package at a time (`-p 1`), and it races `t.Parallel()` tests. One service had 175 such resets. With a database per package and a tenant per test, it has none, and its packages run in parallel.
- On one stack, that service's second full run in a row took about 20% longer. In the third, some packages took over two minutes just to migrate, and failed with `context deadline exceeded` before any test ran. A fresh stack doesn't slow down.

**Do.** The harness function each integration and flow `TestMain` calls:

```go
// internal/testutils/testharness/main.go

// RunMain gives a test package its own database. Its name comes from the
// package path, so two packages never share one.
func RunMain(m *testing.M) int {
	stack, err := loadStack() // the settings task test:integration and task test:flows load from .env.test
	if err != nil {
		fmt.Fprintln(os.Stderr, err) // "the test stack isn't reachable: run task test:up"
		return 1
	}

	if err := stack.createPackageDatabase(); err != nil { // drops a leftover, creates, migrates
		fmt.Fprintln(os.Stderr, err)
		_ = stack.dropPackageDatabase() // a half-made database would stay until the next run
		return 1
	}

	code := m.Run()

	if err := stack.dropPackageDatabase(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return cmp.Or(code, 1)
	}

	return code
}
```

**Don't.** A flow package that skips the harness's `TestMain`, so every flow package runs on one database. A flow that lists or sweeps then sees another package's rows, and two packages drop each other's data:

```go
//go:build flow

package orders_test

func TestMain(m *testing.M) { os.Exit(m.Run()) } // no database of its own
```

**Enforced by** depguard, which bans testcontainers, and review.

## TEST-4 · `require` only, whole values

**Rule.**
- Use testify's `require` package only.
- Compare whole values, expected first: `require.Equal(t, want, got)`. A single failure then prints the full diff.
- **Error kinds.** Assert an error's kind as `require.True(t, errx.IsKind(err, K), "got %v", err)`.
- **Error text.**
  - Assert the text too when the text is the behaviour: a message a client or operator reads, such as a refusal that says what to fix.
  - An error with no kind, such as a config or missing-dependency error, asserts its text, because nothing branches on it (ERR-5).
  - Never assert text that only a developer reads.
  - An errx error's `Error()` also holds its stamps, so compare its whole message with `require.ErrorContains`.
- Every HTTP status assertion passes `resp.BodyString()` as its message.
- Use `require.ElementsMatch` for results that have no order.
- Banned: suites, testify/mock, go-cmp, gomock, and hand-rolled `if got != want { t.Fatalf }`.

**Why.**
- A test makes a non-fatal `assert.Len` or `assert.NotNil` call and then indexes into or dereferences the value. Two codebases had about 150 such sites, and each one panics instead of reporting the failure.
- ERR-5 adds a kind only when code branches on it. So a config error has no kind, and a rule against asserting text left nothing a test could check.
- A refusal that a client reads is part of the API, so its text is behaviour. Text that only a developer reads changes freely, and a test on it breaks for nothing.

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

**Do.** Text that a person reads:

```go
// The client is told what to fix.
require.Equal(t, http.StatusBadRequest, resp.Code(), resp.BodyString())
require.Equal(t, "an order needs at least one item", apitest.Decode[apitest.ErrorBody](t, resp).Message)

// A missing dependency has no kind. Its message is what an operator reads at startup.
_, err := order.New(order.Deps{Orders: orderstest.NewFakeRepo()})
require.ErrorContains(t, err, "order.New: missing required dependency")
```

## TEST-5 · Fakes

**Rule.**

- **Naming.** Every hand-written double is a fake:
  - `fakeX` locally, where `X` is the name of the interface;
  - `xtest.FakeX` when shared (TEST-6).

  No stubs, spies, mocks, `recording…` types, or mocking frameworks. A vendor mock (TEST-17) isn't a double of an interface. It's a server that stands in for a vendor's API, and keeps its name.
- **Shape.** A fake is a small working implementation:
  - in-memory maps for stores;
  - a recorded-calls slice for outbound ports;
  - a `sync.Mutex`;
  - a `newFakeX()` constructor;
  - `var _ pkg.X = (*fakeX)(nil)`.
- **Never embed the interface, and never use function fields.** If a test needs different behaviour, add a field or write another fake.
- **How every fake behaves:**
  - **`Seed(tb, value)`** stores a value as if it had been created. It validates the value, because a row must be valid to exist.
  - **`XErr`** makes every call to method X fail. **`XErrs`** is a list for a test that needs a failure and then a success: one entry per call, and `nil` succeeds. Once the list is used up, `XErr` applies.
  - **Accessors return copies**, such as `Records()` and `Calls()`.
  - **Fakes whose tables are written in one statement are linked,** so a write through one is visible through the other. The capability builder links them (TEST-6).
  - **The real implementation is the truth.** When a fake and the SQL disagree, the fake changes to match. If the SQL looks wrong, report it. Never copy a bug into the fake, and never change production code from a test change.
- **No test writes its own double** for something a shared fake already stands in for. It seeds the shared fake, sets an error on it, and reads it back.
- **Location.**
  - Used by one package: `fakes_test.go` in that package.
  - Used by several packages: one of TEST-6's three levels.
  - Never in a production package.
- **A vendor's HTTP API** is stood in for by its vendor mock (TEST-17), never by an HTTP stub written in a test.

**Why.**
- Across three codebases, four fake designs were in use, under five names (`fake`, `stub`, `spy`, `mock`, `recording`).
- Before one service applied this rule, it had 78 private doubles in six naming styles, and one provider double was written again in at least five packages.
- An embedded interface turns every method you didn't fake into a nil-pointer panic at runtime. This rule only works because ABS-1 keeps interfaces small.
- A fake that accepts a value the database would refuse lets a test pass on data that can't exist.

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
	mu      sync.Mutex
	byID    map[domain.TransferID]domain.Transfer
	updates []domain.UpdateStatusArgs

	GetErr          error
	UpdateStatusErr error
	// UpdateStatusErrs holds one result per call, for a failure and then a
	// success. A nil entry succeeds.
	UpdateStatusErrs []error
}

var _ transfer.Store = (*fakeStore)(nil)

func newFakeStore() *fakeStore {
	return &fakeStore{byID: map[domain.TransferID]domain.Transfer{}}
}

// Seed stores tr as if it had been created. It must be valid, as a stored row is.
func (f *fakeStore) Seed(tb testing.TB, tr domain.Transfer) domain.Transfer {
	tb.Helper()
	require.NoError(tb, tr.Validate())

	f.mu.Lock()
	defer f.mu.Unlock()
	f.byID[tr.ID] = tr

	return tr
}

// Get finds the transfer only in its own tenant, as the query's WHERE does.
func (f *fakeStore) Get(_ context.Context, tenant domain.TenantID, id domain.TransferID) (domain.Transfer, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if f.GetErr != nil {
		return domain.Transfer{}, f.GetErr
	}
	tr, ok := f.byID[id]
	if !ok || tr.TenantID != tenant {
		return domain.Transfer{}, errx.NewKind(0, errkinds.ErrKindNotFound, "transfer not found")
	}
	return tr, nil
}

func (f *fakeStore) UpdateStatus(_ context.Context, args domain.UpdateStatusArgs) (domain.Transfer, error) {
	f.mu.Lock()
	defer f.mu.Unlock()
	if err := nextErr(&f.UpdateStatusErrs, f.UpdateStatusErr); err != nil {
		return domain.Transfer{}, err
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

// nextErr returns the next call's result from errs, or err once errs is used up.
func nextErr(errs *[]error, err error) error {
	if len(*errs) == 0 {
		return err
	}
	next := (*errs)[0]
	*errs = (*errs)[1:]
	return next
}
```

## TEST-6 · Shared fakes: three levels, each proven

**Rule.** A project's shared fakes come in three levels:
1. **Boundary fakes:** one per external system or shared port, in an `xtest` package beside what it stands in for, such as `jobstest.FakeEnqueuer` beside `internal/jobs`.
2. **Repository fakes:** one `FakeRepo` per repository, in `<aggregate>test` beside the repository, with that repository's seed helpers, such as `orderstest.FakeRepo` and `orderstest.Seed`.
3. **Capability builders:** one `<capability>test` package per capability, such as `ordertest`.
   - `NewFakes(tb)` returns every fake the capability needs, already wired together.
   - `NewService(tb, f)` builds the real service over them, with every dependency set (TEST-11).
   - Core, handler and job handler tests all start here.

- Every fake at levels 1 and 2 runs its conformance suite twice: against the fake in a unit test, and against the real implementation in an integration test. The suite is `xtest.RunXConformance(t, newX)`.
- A capability is named in the singular and its repository in the plural, such as `order` and `orders`. So `ordertest` and `orderstest` never clash, and no import needs an alias (LAY-8).
- **A shared fake starts the way production is set up.**
  - It declares what the real implementation declares, with the settings that ship: its capabilities, requirements and limits. Every fake a capability builder makes starts that way. A fake never leaves out a rule because most tests don't need it.
  - A parity test beside the real implementation compares the two, and fails when they differ. It's a unit test, so a drift fails in `task test`, not only in an integration run. It builds the real implementation with the settings that ship, from the same place the service reads them.
  - A test that needs something different changes the fake in its own body, with a comment saying why. A builder or a shared constructor never changes it for the test.
  - The conformance suite still checks behaviour. The parity test checks set-up. A fake needs both: it can pass every conformance case and still declare a vendor production doesn't have.

**Why.**
- A suite that runs on both sides keeps a fake honest as the real implementation changes.
- Without levels, each package writes its own fakes. With one builder per capability, a core test, a handler test and a job handler test of the same capability start from the same fakes, wired the way production wires them.
- A fake set up unlike production lets every test built on it skip a rule production applies. In one service, the fake payment provider that every account unit test used declared the real provider's account shape, but not its rule that a customer is verified first. Its comment said a test that needs the rule should switch it on.
  - 140 of the 143 account unit tests ran on a provider that never asks for an approval.
  - The 3 that switched the rule on tested only refusals.
  - So no unit test showed an approved customer, of any kind, opening an account with the rule on. If the approval lookup broke for one kind of customer, no unit test that expects success could fail. An approved business couldn't open an account, and every test passed.

**Do.** One suite runs against both the fake and the real Kafka publisher:

```go
func RunPublisherConformance(t *testing.T, newPublisher func(tb testing.TB) stream.Publisher) {
	t.Helper()
	t.Run("a cancelled context never reports success", func(t *testing.T) { /* ... */ })
	// ...
}
```

**Do.** A capability builder:

```go
// internal/core/order/ordertest/ordertest.go

// Fakes holds every dependency of order.Service as a fake. Orders and Items
// share rows, because one statement writes an order and its items.
type Fakes struct {
	Orders   *orderstest.FakeRepo
	Items    *itemstest.FakeRepo
	Payments *paymentstest.FakeProvider
	Jobs     *jobstest.FakeEnqueuer
	Clock    *clocktest.Clock
	IDs      ids.Generator
}

// NewFakes returns empty fakes, linked, with the clock at domaintest.T0.
func NewFakes(tb testing.TB) *Fakes {
	tb.Helper()

	orders, items := orderstest.NewFakeRepo(), itemstest.NewFakeRepo()
	orders.LinkItems(items)

	return &Fakes{
		Orders: orders, Items: items,
		Payments: paymentstest.NewFakeProvider(), Jobs: jobstest.NewFakeEnqueuer(),
		Clock: clocktest.New(domaintest.T0), IDs: idstest.Sequence(),
	}
}

// NewService builds the real order.Service over f, with every dependency set.
func NewService(tb testing.TB, f *Fakes) *order.Service {
	tb.Helper()

	svc, err := order.New(order.Deps{
		Orders: f.Orders, Items: f.Items, Payments: f.Payments, Jobs: f.Jobs, Clock: f.Clock, IDs: f.IDs,
	})
	require.NoError(tb, err)

	return svc
}
```

**Don't.** A shared fake set up unlike production. Every account test runs on a provider that never asks for an approval:

```go
// internal/core/account/accounttest/accounttest.go

// ProviderCaps are the fake provider's capabilities: the real provider's
// account shape, without its rule that a customer is verified first. A test
// that needs the rule sets Provider.Caps.RequiresVerifiedCustomer.
func ProviderCaps() domain.Capabilities {
	return domain.Capabilities{OneCurrencyPerAccount: true, DetailsArriveLater: true}
}
```

**Do.** A shared fake set up as production is, and its parity test beside the real adapter:

```go
// internal/adapters/payments/paymentstest/provider.go

// Capabilities are what the real payment provider declares, with the settings
// that ship: it verifies customers first, opens one-currency accounts whose
// details arrive later, and keeps the shipped review deadline. Every
// FakeProvider starts with them. TestClient_DeclaresWhatTheFakeProviderDeclares,
// beside the real adapter, fails when the two differ.
func Capabilities() domain.Capabilities {
	return domain.Capabilities{
		RequiresVerifiedCustomer: true,
		OneCurrencyPerAccount:    true,
		DetailsArriveLater:       true,
		ReviewDeadline:           24 * time.Hour,
		// ... every field the adapter sets
	}
}
```

```go
// internal/adapters/payments/capabilities_test.go

// Every core test runs on the fake provider, so it declares what this adapter
// declares with the settings that ship. Otherwise a core test passes on a
// provider production doesn't have.
func TestClient_DeclaresWhatTheFakeProviderDeclares(t *testing.T) {
	client, err := payments.New(shippedConfig(t)) // read from the same place the service reads it
	require.NoError(t, err)

	require.Equal(t, paymentstest.Capabilities(), client.Capabilities())
}
```

**Do.** A test that opens an account states what it relies on, for each kind of customer. A test that needs a provider unlike production's changes the fake in its own body:

```go
// An approved customer's account is opened as PENDING at the provider,
// whichever kind of customer the provider approved.
func TestOpenAccount_OpensForACustomerTheProviderApproved(t *testing.T) {
	for _, kind := range []domain.CustomerKind{domain.CustomerKindPerson, domain.CustomerKindBusiness} {
		t.Run(string(kind), func(t *testing.T) {
			f := accounttest.NewFakes(t)
			req := accounttest.OpenRequest(t)
			f.ApproveCustomer(t, req, kind)

			account, err := accounttest.NewService(t, f).Open(t.Context(), req)

			require.NoError(t, err)
			require.Equal(t, domain.AccountStatusPending, account.Status)
		})
	}
}

// A provider that verifies no one opens an account without an approval. No
// provider in production works this way, so the test changes the fake itself.
func TestOpenAccount_AProviderThatVerifiesNoOneOpensWithoutAnApproval(t *testing.T) {
	f := accounttest.NewFakes(t)
	f.Provider.Caps.RequiresVerifiedCustomer = false
	// ...
}
```

**Enforced by** the conformance suites and the parity tests, and review that each shared fake of a vendor or a store has both.

## TEST-7 · Tables and names

**Rule.**
- **When to use a table:** when three or more cases run identical act-and-assert code and differ only in data. Otherwise write separate subtests or tests.
- **Row fields,** in this order:
  1. `name`;
  2. inputs, named after the parameters;
  3. `want` or `wantX`.

  Rows have no function-typed fields, and no `setup` or `mock` fields.
- **The loop body** doesn't branch. Error cases go in their own table, whose loop asserts one kind.
- **Names say what the application does,** so a reader learns the behaviour from the tests:
  - A top-level test is `Test<Unit>_<WhatItDoes>`, such as `TestCancelOrderHandler_RefusesAKeyThatMayOnlyRead`, with a sentence above it saying the behaviour.
  - A table test's name says what its rows share. Subtest names are lowercase phrases.
  - The loop variable is `tc`.
  - Variables, helpers and constructors say what they hold or do: `readOnlyKey`, `otherTenant`, `newAdminAPI(tb, role)`.
  - Names hold no labels from plans or tickets, such as "R14" or "D12".
- **Shape:** arrange, blank line, **one** act, blank line, assert. Extract helpers once a test passes about 60 lines.

**Why.**
- Banning `t.Run` inside loops altogether is too blunt.
- A `wantErr bool` field proves only that *some* error happened.
- `tc := tc` copies have been unnecessary since Go 1.22.
- A name that says the behaviour keeps its meaning. A label from a plan means nothing once the plan is gone.

**Canonical:**

```go
// ParseAmount reads an amount in the asset's smallest unit, and refuses one
// the asset can't hold.
func TestParseAmount_ReadsOnlyAmountsTheAssetCanHold(t *testing.T) {
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
- A helper's signature is `func X(tb testing.TB, …)`, and its first statement is `tb.Helper()`.
- It fails with `require` and never returns an error.
- It takes no ctx parameter; it uses `tb.Context()`.
- It registers teardown with `tb.Cleanup` and never returns a cleanup function.
- `tb.Context()` is cancelled *before* cleanups run, so cleanups use `context.WithoutCancel(tb.Context())`.

**Why.**
- Without a fixed signature, helper parameters drift. One codebase had both `(ctx, t, …)` (23 helpers) and `(t, ctx, …)` (25).
- Without this rule, seeders take `t` *and* return an error.
- A helper that returns a cleanup function leaves callers able to forget to call it.
- The `thelper` linter the standard ships names a `testing.TB` parameter `tb`. This rule used to say `t`, so the first project to turn the linter on found every helper reported.

```go
// internal/testutils/dbtest/dbtest.go
func New(tb testing.TB) *pgxpool.Pool {
	tb.Helper()
	pool, err := pgxpool.New(tb.Context(), testharness.From(tb).SQLURL)
	require.NoError(tb, err)
	tb.Cleanup(pool.Close)
	return pool
}
```

## TEST-10 · Builders and fixtures

**Rule.**
- **Builders.** `xtest.<Type>(tb)` returns a *valid* value with fixed IDs and fixed times derived from `xtest.T0`. The test then overrides fields directly.
  - Builders don't use functional options and don't call `time.Now()`.
  - Uniqueness in shared stores comes from the harness namespace, not from the wall clock (such as `UnixNano()`).
  - When a choice changes what the service does with a value, such as the kind of customer or the country, the builder or seed helper takes that choice as an argument. It never picks one for the test.
- **Seeding.** A repository test seeds rows through the seed helpers beside the repository, `<aggregate>test.Seed…(tb, repo, v)`. So does an integration test of machinery no caller can reach, beside its job handler or repository. A flow never seeds (TEST-16). An e2e test sets up through the HTTP API.
- **`testdata/`** holds only non-Go files. Load them with a typed loader (such as `fixture.LoadJSON[T](tb, parts...)`). Golden files are refreshed with an `-update` flag.

**Why.**
- A package of builders inside a `testdata/` directory was imported by 114 files. `./...` skips `testdata` directories, so `go vet ./...` and golangci-lint never analyse that package.
- A seed helper that always made a person is how an approved business went untested. Every account test in one service got its verified customer from that one helper. The rule "an account needs a verified customer" was one core test, seeded with a person, and about 3,800 tests passed while a business couldn't open an account.

**Don't.** A seed helper that picks the kind of customer, so no test that uses it meets any other kind:

```go
// SeedApproved stores an approved customer. It always makes a person.
func SeedApproved(tb testing.TB, repo *customers.Repo) domain.Customer {
	tb.Helper()

	return Seed(tb, repo, Customer(tb, domain.CustomerKindPerson, domain.CustomerStatusApproved))
}
```

**Do.** The test names the choice, because the service treats each kind differently:

```go
// SeedApproved stores an approved customer of kind.
func SeedApproved(tb testing.TB, repo *customers.Repo, kind domain.CustomerKind) domain.Customer {
	tb.Helper()

	return Seed(tb, repo, Customer(tb, kind, domain.CustomerStatusApproved))
}
```

**Do.** A builder with fixed IDs and times:

```go
// internal/domain/domaintest/transfer.go
var T0 = time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)

// Fixed IDs, so whole-value comparisons work.
var (
	TransferID1 = domain.TransferID{1}
	TenantID1   = domain.TenantID{1}
)

// Transfer returns a valid initiated transfer; override fields in the test.
func Transfer(tb testing.TB) domain.Transfer {
	tb.Helper()
	amount, err := domain.NewAmount(1_000_000, USDC)
	require.NoError(tb, err)
	return domain.Transfer{ID: TransferID1, TenantID: TenantID1, Amount: amount,
		Status: domain.TransferStatusInitiated, CreatedAt: T0}
}
```

**Enforced by** review.

## TEST-11 · How each layer is tested

| Layer | Recipe |
|---|---|
| `domain` | <ul><li>Pure tests, no fakes.</li><li>A valid table and an invalid table for each constructor and `Validate()`.</li><li>The transition-table test ranges over enumx `Values()` (DOM-7).</li><li>Fuzz parsers of untrusted input and assert a *property*, such as round-trip or idempotence. "Doesn't panic" is not enough.</li></ul> |
| `core` | <ul><li>Build the service with the capability builder, `<capability>test.NewService(tb, f)`, which sets **every** dependency:<ul><li>fakes;</li><li>`clocktest`;</li><li>`idstest`;</li><li>a noop meter, if the capability has metrics.</li></ul></li><li>Seed the fakes, call the service, and read the fakes back.</li><li>Time passes with `f.Clock.Advance`.</li><li>Never pass `nil` with a "not reached" comment.</li><li>`cancel.go` is tested in `cancel_test.go`.</li></ul> |
| `http` | <ul><li>See TEST-15.</li><li>The HTTP plumbing, such as middleware and the error encoder, has unit tests beside it.</li></ul> |
| `workers` | <ul><li>Call `Handle(t.Context(), job)` directly, over the capability's fakes.</li><li>A realistic start creates the entity through the service and runs the job that create queued.</li><li>Assert the resulting state and the returned error kind. Test the runtime's outcome mapping once, in its adapter.</li><li>Never start the queue runtime in a unit test. Integration tests drive `RunOnce`.</li><li>Machinery no caller can reach, such as a sweep's query or an archive of rows older than any caller can make, is an integration test here, where seeding is allowed.</li></ul> |
| `repository` | <ul><li>Tagged `integration`, beside the repo, in its package's database (TEST-3), migrated with the production runner.</li><li>Seed rows with the seed helpers in `<aggregate>test`.</li><li>The repository's conformance suite runs here against the real repository (TEST-6).</li><li>Required: `AssertStablePaging`, `AssertConcurrentUpdateConflicts`, and the migrate-twice test.</li></ul> |
| `adapters` | <ul><li>An integration test against the test stack, or against the vendor mock for a vendor (TEST-17).</li><li>Each test or flow harness starts its own mock in the process, on its own port.</li></ul> |
| redis | <ul><li>`miniredis`, a real Redis server that runs inside the test process, can be used in any test.</li></ul> |
| `flows` | <ul><li>See TEST-16, TEST-18 and TEST-19.</li></ul> |
| `contract` | <ul><li>See TEST-17.</li></ul> |

**Don't:**

```go
return transfersvc.New(transfersvc.ServiceDependencies{
	Wallets: walletReader,
	Repo:    nil, // not reached by validation-failure tests
	Clock:   apptime.NewFrozenClock(time.Now()),
```

**Do.** A core test starts from the capability builder:

```go
// An order with one item refuses a second item, and nothing is written.
func TestAddItem_RefusesASecondItemOnASingleItemOrder(t *testing.T) {
	f := ordertest.NewFakes(t)
	single := f.Orders.Seed(t, orderstest.SingleItemOrder(t))
	svc := ordertest.NewService(t, f)

	_, err := svc.AddItem(t.Context(), order.AddItemArgs{TenantID: single.TenantID, OrderID: single.ID, Item: orderstest.Item(t)})

	require.True(t, errx.IsKind(err, errkinds.ErrKindConflict), "got %v", err)
	require.Empty(t, f.Items.Added())
}
```

The canonical handler test is in TEST-15.

**Enforced by** review.

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
  - a bare `time.Sleep` used to synchronise. A sleep stays only where it is the behaviour under test, with a comment saying so.

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
// An entry nobody uses for longer than its idle time is evicted.
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

## TEST-13 · The standard ships the structural checks

**Rule.**
- A project keeps no `internal/architecture`, and writes no checking code of its own. The structural checks, the ones golangci-lint's own linters can't express, ship with the standard:
  - the analyzers in [lint/analyzers](lint/analyzers), which `task lint` runs inside golangci-lint, one package at a time;
  - [tools/archcheck](tools/archcheck), which `task standards:check` runs, for what needs the whole module or files that aren't Go.

  [02](02-style-and-naming.md#what-the-lint-config-covers-and-what-it-cant) maps each check to its rule.
- They take nothing from the project. The module comes from the package path, the layer from the folder under `internal/`, and each SDK's home from the system it belongs to (LAY-7). So a rename in a project changes nothing in a check.
- A justified exception is a `//nolint:<analyzer> // reason` on the line it's about, agreed with the engineer (COL-7). Nothing keeps a list of exceptions. archcheck takes no exceptions: each of its findings is fixed, and one that's wrong is a bug in archcheck, fixed in the standard.
- **A check that reads nothing must fail.** Each analyzer has fixtures with at least one case it reports and one it doesn't, and archcheck has a fixture module where every check fires and one where none does. A check that stopped reading code fails the standard's own tests, instead of passing quietly in every project.
- **Test code stays in tests.** A `<pkg>test` package, anything under `internal/testutils`, the `testing` package and testify are imported only by `_test.go` files and other test packages (the `layers` analyzer). No binary in `cmd/` carries test code that a library brings in (`tools/archcheck`).
- **Tests archcheck checks.** archcheck loads the module with every kind's tag, so it reads every test:
  - each kind's tag and home: the `contract` tag in `test/contract` only, the `flow` tag in `test/flows` only, the `integration` tag on exactly the `_integration_test.go` files, and no `_integration_test.go` in `test/flows` (TEST-1);
  - every handler file has its test file (TEST-15);
  - flow files hold tests only (TEST-19);
  - flows and the harness write only through boundaries: no seed package, no repository write, no SQL in a flow, no SQL that writes in the harness, and no stand-in for the vendor mock (TEST-16);
  - the promises in `docs/promises`, the flows and the operations in `docs/openapi` agree (TEST-18);
  - no test function is left uncalled (TEST-19).

  Fixing a finding changes no existing assertion (TEST-14).
- **A project that wrote its own guard or promise check deletes it when it upgrades,** because archcheck runs the same check. Two copies of one rule drift, as the architecture tests did.
- **Tests left to review:** the lint config doesn't read test files, because its checks would make a paused project's existing tests change (TEST-14). So fake hygiene (TEST-5), tables and names (TEST-7), and Go under `testdata/` (TEST-10) are for review. archcheck isn't lint, and a tag fix is allowed under the pause, so archcheck checks the `integration` tag on `_integration_test.go` files.

**Why.** The standard used to ask each project for "one table-driven architecture test", copied into every project, and promised a shared one it never built. Across three codebases, one wrote 20 files and 3,931 lines of it, one wrote 1 file and 506 lines, and one wrote none. One rule set was enforced three ways, or not at all. The largest copy:
- broke on renames. It hard-coded package paths and exception maps, so renaming four adapter packages meant editing four test files, and none of those edits found a problem;
- guarded against reading nothing with counts such as "fewer than 1,000 stamps", which fail on a small project and needed a test of their own;
- repeated checks lint and `task stamps:check` already made;
- enforced libraries' own conventions, and one project's history, such as "the removed worker family doesn't come back";
- kept its exceptions in maps inside the tests, where nobody reading the code could see that a file was exempt, or why.

A project can't fix that alone, because the checks are only worth having if every project runs the same ones.

**Do.** An exception, where it applies:

```go
import "example.com/myservice/internal/adapters/scylla" //nolint:layers // keeps its name until the rename to scyllastore (LAY-8)
```

**Linters** (not applied while restructuring is paused, because the lint config skips test code):
- testifylint, with all checks enabled;
- thelper, usetesting and copyloopvar;
- testpackage: use package `x_test`, and package `x` only in `*_internal_test.go`;
- depguard: suite, mock, go-cmp, gomock and testcontainers;
- forbidigo: `testing.Short`, and `time.Now` in `*test` packages.

Benchmarks use `b.Loop()`.

**Don't.** A guard the project wrote for itself. archcheck runs the same check on every project, so this copy can only drift from it:

```go
// internal/testutils/testharness/write_guard_test.go

// Nothing in a flow, or in the harness, seeds a repository's rows.
func TestFlowsWriteOnlyThroughTheAPI(t *testing.T) {
	// ... parses every file in test/flows and the harness, with its own list of exceptions
}
```

**Enforced by** the `layers` analyzer and `tools/archcheck`, each proven by its fixtures, and review.

## TEST-14 · Existing tests are the guardrails

**Rule.**
- **New tests follow this chapter, in every project.** New behaviour still gets its acceptance tests first (COL-4).
- **Restructuring existing tests is paused** until the engineer lifts the pause for the project. Restructuring covers moving or renaming tests, changing assertion style, and changing helpers, harnesses or test infrastructure.
- **A new test may add what it needs.** Where it needs a helper the project doesn't have yet, such as an `apitest` constructor or a capability builder, it adds one. Where it shares setup that already exists, such as its package's `TestMain`, it uses that setup as it is until the lift.
- **Existing tests stay as they are during a refactor.** If they still pass, the behaviour hasn't changed.
- **If a refactor changes a signature,** change the test only enough to compile. Don't change what it asserts. If an assertion has to change, that's a behaviour change: stop and ask (COL-4, COL-11).
- **A finding from `tools/archcheck` on a test is fixed,** and fixing it isn't restructuring. A missing handler test file is a new test (TEST-15). A wrong tag is fixed with the build line, or by moving a contract test to `test/contract`. Either changes when a test builds, not what it asserts (TEST-1). These are fixes too, because none of them changes what a test asserts:
  - moving a flow file's function into the harness, unchanged (TEST-19);
  - renaming a flow's `TestMain` file to `main_test.go` (TEST-1);
  - deleting a test function nothing calls (TEST-19).
- **How the rules on promises and helpers apply under the pause:**
  - **These apply at once,** because each changes where code lives or when it builds, not what a test asserts:
    - the `flow` tag, and the `main_test.go` name (TEST-1);
    - moving a flow file's functions into the harness, unchanged (TEST-19);
    - deleting a test function nothing calls (TEST-19);
    - the new test tasks (TEST-1);
    - writing the promise files (TEST-18).
  - **New flows follow TEST-16 and TEST-18 from the start,** as every new test follows this chapter.
  - **Rewriting an existing flow waits for the lift.** That's a flow that seeds rows, signs in with a test-only credential, or skips the production servers. Rewriting it is restructuring.
    - Until the lift, the area's promise file lists it under "Not yet proved as the caller" (TEST-18): the promise, the existing test, and what that test does that a caller can't.
    - The promise check accepts what that section names. So does the setup check (TEST-16), for those flows and for the harness code only they reach. Code that any other test reaches is still checked, so a new flow, or a harness step a new flow uses, can't seed.
    - The lift empties the section, and every check then applies in full.
  - **An existing shared fake gets production's set-up at the lift** (TEST-6), because the tests built on it start meeting rules they used to skip. A new shared fake starts that way, with its parity test.
- **The engineer lifts the pause per project, and the project records it in its `AGENTS.md`,** under `## This project's own rules`, as a line starting `TEST-14 lifted:` with the date. Lifting it follows the recipe in [10-adoption](10-adoption.md#lifting-test-14).
- **New projects** have nothing to restructure. They follow this chapter from the start.

**Why.**
- It's risky to change tests during a major refactor. If the code and its tests change together, nothing shows that the behaviour stayed the same.
- This rule used to say that new tests follow the style already in their package. So in a project under the pause, every new test copied the old patterns, and they kept spreading. A new test changes no existing guardrail, so it can follow this chapter.
- A project under the pause must still pass `task check`. Two of three services that follow the standard haven't lifted it, and their flows seed rows and write through repositories: 60 such findings in one of them. Rewriting those flows is restructuring. Without the "Not yet proved as the caller" list, that project couldn't upgrade until it lifted the pause. With it, the old flows are named in one place, and nothing new can join them.

**Don't.** Rewrite an existing flow's setup in the upgrade PR, while the pause holds. Its assertions now run on different state, so a pass no longer shows the behaviour stayed the same:

```diff
 func TestPlaceOrder_ChargesTheCustomer(t *testing.T) {
 	w := testharness.New(t)
-	customer := customerstest.SeedApproved(t, w.Repo(), domain.CustomerKindPerson)
+	customer := w.NewTenant(domain.ModeLive, testharness.ScopesToPlaceOrders...).VerifyCustomer(testharness.NewCustomer(domain.CustomerKindPerson, domain.CountryUS))
```

**Do.** List it in its area's promise file until the lift:

```markdown
## Not yet proved as the caller

- **ORD-1.** `TestPlaceOrder_ChargesTheCustomer` seeds its customer straight into the table, always as a person, where the tenant's app verifies one through the API.
```

**Enforced by** review. The promise check and the setup check read the "Not yet proved as the caller" list (TEST-16, TEST-18).

## TEST-15 · Handler tests

**Rule.** Every handler has a test file beside it. Its requests go through the production route table and its auth, to the real service over fakes.

- **Setup.** The project's `apitest` package has one constructor per HTTP surface, such as `NewProductAPI(tb, deps)` and `NewAdminAPI(tb, deps)`.
  - Each one mounts the production route registration, `routes.RegisterXRoutes`, with its real middleware.
  - The deps hold only what the handlers use: the real service over the capability's fakes, or the repository fake when the capability has no service (HTTP-3).
  - The constructor fills the rest with working stand-ins, and the whole set passes the same check the binary runs at startup (LAY-3).
- **Credentials.** Each surface has one helper per kind of caller. It stores the caller in the fakes and returns its token or key, such as `api.SignIn(tb, role)` or `api.IssueKey(tb, access)`.
- **Requests.** `api.Send(tb, token, method, path, body)`.
  - An empty token sends no `Authorization` header.
  - A write sends a fresh idempotency key. `api.SendWithIdempotencyKey` is for a retry test.
  - `apitest.Decode[T](tb, resp)` reads the body.
- **Layout.** `create.go` is tested in `create_test.go`. One `helpers_test.go` per package holds:
  - one type that embeds the surface's API and holds the package's fakes;
  - one constructor with explicit parameters;
  - only helpers that remove real repetition or hold logic. A helper that only renames something doesn't belong there.
- **What a handler's tests cover:**
  - **behaviour:** the status and body on success, each error kind it treats specially, and the response shape;
  - **binding:** one bad-request check per kind of input. The framework's own tests prove each validation tag, so there's no test per field;
  - **auth:**
    - each write refuses a caller without the role or scope it needs (403) and a request with no credentials (401), and changes nothing;
    - each read works for the least role that may read;
  - **tenancy:** another tenant's record gets exactly the reply a missing one gets (SEC-19).

**Why.**
- TEST-11 used to mount one handler on a bare router. Such a test never passes through the production route table, its auth or its role check. So a route registered in the wrong group, or with no auth, passes every handler test.
- One service has a test file beside each of its 110 handler files. These tests run in milliseconds. They reach every branch, including failures that are hard to cause for real, such as a timeout.
- The SQL under each handler is proven by its repository's conformance suite against the database (TEST-6), so the handler test doesn't repeat it.
- This settles the question TEST-11 left parked: handler tests use fakes.

**Don't.** The handler is mounted on a router of its own, so no request meets the route table, its auth or its scope check:

```go
r := apitest.NewRouter(t, apitest.Deps(t, deps.Config{Orders: svc}))
r.Post("/v1/orders/:order_id/cancel", orders.CancelOrderHandler)
```

**Do.** The surface, in the project's `apitest` package:

```go
// internal/testutils/apitest/product_api.go

// NewProductAPI mounts the product API's production routes over d. The test
// sets the deps its handlers use. The rest get working stand-ins, and the set
// passes deps.New, the check the api binary runs at startup.
func NewProductAPI(tb testing.TB, d deps.Config) *ProductAPI {
	tb.Helper()

	keys := apikeytest.NewFakes(tb)
	d.APIKeys = apikeytest.NewService(tb, keys) // the product API checks the keys it issues
	fillStandIns(tb, &d)

	built, err := deps.New(d)
	require.NoError(tb, err)

	router := gofi.NewRouter()
	deps.Register(router, built)
	routes.RegisterProductRoutes(router) // the global middleware, then each group's auth and scope

	return &ProductAPI{router: router, keys: keys}
}
```

**Do.** The package's helpers:

```go
// internal/http/api/orders/helpers_test.go

// productAPI is the product API as production routes it, over orders kept in
// memory. One tenant exists, with a key that may read and write orders.
type productAPI struct {
	*apitest.ProductAPI
	fakes  *ordertest.Fakes
	tenant domain.TenantID
	key    string
}

func newProductAPI(tb testing.TB) *productAPI {
	tb.Helper()

	f := ordertest.NewFakes(tb)
	api := &productAPI{
		ProductAPI: apitest.NewProductAPI(tb, deps.Config{Orders: ordertest.NewService(tb, f)}),
		fakes:      f,
		tenant:     domaintest.NewTenantID(),
	}
	api.key = api.IssueKey(tb, domain.Access{TenantID: api.tenant, Scopes: []domain.Scope{domain.ScopeOrdersRead, domain.ScopeOrdersWrite}})

	return api
}
```

**Canonical:**

```go
// internal/http/api/orders/cancel_test.go

// Cancelling an order needs a key with the orders write scope. A key that may
// only read is refused, and the order stays as it was.
func TestCancelOrderHandler_RefusesAKeyThatMayOnlyRead(t *testing.T) {
	api := newProductAPI(t)
	placed := api.fakes.Orders.Seed(t, orderstest.PlacedOrder(t, api.tenant))
	readOnlyKey := api.IssueKey(t, domain.Access{TenantID: api.tenant, Scopes: []domain.Scope{domain.ScopeOrdersRead}})

	resp := api.Send(t, readOnlyKey, http.MethodPost, "/api/v1/orders/"+placed.ID.String()+"/cancel", nil)

	require.Equal(t, http.StatusForbidden, resp.Code(), resp.BodyString())
	require.Empty(t, api.fakes.Orders.Updates())
}
```

**Enforced by** `tools/archcheck`: every file under `internal/http` that declares a handler has a test file of the same name beside it. What the tests cover, and that they go through the production route table, are for review.

## TEST-16 · Flow tests

**Rule.** A flow proves one promise as its caller meets it. It signs in as a caller can, goes through the production servers, and sets up through the API, the queued jobs and the vendor mock. It states every choice, and checks only what the caller sees.

- **What a flow is for:**
  - each promise in its area's promise file (TEST-18), with each variant that changes the answer, and each refusal and failure a caller can meet;
  - a write through one API, seen through another;
  - one check per capability that an audit record reaches its real table;
  - a request reaching the queue, and one job's happy path;
  - an inbound webhook through the real translation and SQL;
  - stories that cross components;
  - the lifecycle story (below).
- **What it isn't for:**
  - a rule's every branch, which is a core test;
  - SQL on its own, which is a repository test;
  - a route's binding, or its scope one route at a time, which is a handler test (TEST-15);
  - machinery no caller can reach, which is an integration test beside its job handler or repository, where seeding is allowed (TEST-11).

  Each area has one access promise, such as "only the tenant's key, with the right scope, reaches its orders". One flow proves it over all the area's routes, with keys issued the way callers get them. A handler test signs its caller in through fakes, so only a flow can show that the production credential path refuses.
- **It acts as the caller:**
  1. **It signs in only as a caller can.** A tenant's app uses the key its owner issued through the API, with the scopes and mode the test names. An operator signs in with their role. The service under test accepts no test-only credential. A refusal test issues a second key with fewer scopes.
  2. **It goes through the production servers,** with their middleware in order, their body limits and their request deadline. A limit the transport enforces outside the router is checked against the built binary.
  3. **It sets up the way a caller does:** through the API, the jobs the API queued (run as the queue runs them), and the vendor mock's webhooks.
     - No seeding, no repository writes and no SQL, in a flow or in the harness.
     - The one exception is an identity no API can create, such as the first admin. It's seeded in one harness file, `internal/testutils/testharness/bootstrap.go`.
     - archcheck enforces this.
  4. **It states every choice.** A step takes the kind of customer, the country, the currency and the mode as arguments, with no defaults.
  5. **It runs each variant that changes the answer,** as named subtests: each kind of customer in each country, and one run per value wherever the service or the vendor treats values differently. Not every combination.
  6. **It meets every refusal and failure a caller can:**
     - each refusal, with its status and exact message;
     - a retry with the same idempotency key;
     - a key without the permission;
     - another tenant's record;
     - the vendor down, refusing, or leaving the service in doubt;
     - a webhook that comes twice, late or never.
  7. **It checks only what the caller sees:** the answers and their messages, the events the caller is sent, what an operator sees, and what the audit store receives. It reads storage only when the promise is about storage, such as "the customer's details are kept nowhere". It does that through a harness reader, with a comment saying so.
  8. **It takes each next step with only what the caller holds:** what earlier answers gave it, and the events it was sent. A flow never reads an ID from storage to take a step. The vendor mock answers from what the service sent it, never from the service's storage.
  9. **It's named for what the caller does and sees,** such as `TestOrder_EachKindOfCustomerPlacesAnOrderAndPaysIt`. Never for the code path.
- **Shape:**
  - **A promise about one call's answer:** set up with steps, send that call in the test body with the harness's request builder, then assert.
  - **A promise about a record's life:** check each state the caller can see, in order, each before the next. That's the lifecycle test's shape, used for any promise about a life.
  - Different promises are different tests. The variants of one promise are its subtests.
- **The harness.** `testharness.New(tb, opts...)` builds one app against the stack: its repositories, services, clock, and a vendor mock of its own (TEST-17).
  - Each HTTP surface and the workers are lazily built facets of that one app, such as `w.API()`, `w.Admin()` and `w.Workers()`.
  - Options are applied once, before anything is built.
  - A test never stitches several separately wired apps together.
  - Its steps, journeys, readers and request builders follow TEST-19.
- **Setting up state.** Through steps, such as `app.PlaceOrder(...)`, and vendor steps, such as `w.Provider().SettlesPayment(...)`. Both use the real paths (point 3).
  - A state no boundary produces is usually a unit test over fakes instead, or an integration test beside its job handler or repository.
- **The harness behaves like production.**
  - Its settings are the ones that ship: cache lifetimes, vendor deadlines and retry budgets.
  - Its job runner ends each job as the queue does: completed, dropped, retried or dead-lettered. It applies each kind's attempt limit, and calls the dead-letter hook.
  - A harness that's kinder than production hides what callers will meet.
- **The lifecycle.** One test, in `test/flows/lifecycle`, follows one customer from onboarding to a settled payment. It's the product's main journey (TEST-18).
  - Every step is under test, so each step makes its own call and is checked before the next.
  - Each step is a subtest, and the test stops at the first failure.
  - No step uses a shortcut that makes what an earlier step should have made.
  - Each job it runs is the one a request queued, read back from the real queue.
- **Time.** A flow reads time from the harness, `w.Clock().Now()`, never `time.Now()`.
  - The harness gives its one clock to every repository and service it builds. A repository never builds its own clock (DATA-6), or a flow couldn't fix the time.
  - A behaviour that needs time to pass is a unit test with `Advance`.

**Why.**
- In one service, 523 tests ran the whole app against the database, and about 490 of them sat beside handlers and job handlers. Many built an app and its rows to check what one handler or one rule does, which a unit test over fakes checks in milliseconds. After the service applied this chapter, 163 remain.
- Separately wired apps don't share the services and clock that production shares. A flow across them tests wiring that production doesn't have.
- The lifecycle test catches what each smaller flow can't: a step that works alone but doesn't make what the next step needs.
- **Flows that didn't act as callers confirmed the code, not the product.** In that service:
  - **The flows signed in with a credential no caller can hold.** A test-only verifier accepted a console session on the product API and granted every permission, in sandbox mode only. Every handler test passed, because a handler test signs its caller in through fakes.
  - **They skipped the production servers.** They mounted the routes on a test router, without the real server's body limits or request deadline.
  - **Their setup wrote rows straight into the database,** through the door the old rule left open: "a seed helper is allowed when no boundary can produce the state". Almost all of them used one kind of customer, in one country, paying one currency. No flow ever showed an approved business opening an account, and it couldn't.
  - **Much of what callers see was never tested.** 22 of the 38 event types the service sends its callers were never named in a flow, and 8 were never named in any test. Seven are never written by any code, though callers see them in the API description.
- **Rebuilt to act as callers, the flows found three broken promises in their first pass:**
  - a payment in a currency the account doesn't hold was accepted, and could only fail later, at the vendor;
  - a request cut off by the server's deadline answered 400 "Please check your input", so no caller would retry, though nothing was wrong with the input;
  - a sandbox account whose activation webhook was lost was never activated.
- **The harness was kinder than production in seven ways.** Each one made a test pass that a caller would fail:
  - the vendor client had no review deadlines, so a "past its deadline" state could never appear;
  - a job the harness ran stayed "waiting", so a later job with the same key was swallowed, and a dropped job looked like a failure;
  - the vendor mock's webhooks had no change stamp, so a second real change looked like a resend;
  - the mock kept no record of one resource, so "the webhook is lost, and the service asks the vendor" couldn't be tested;
  - a sandbox key couldn't choose the vendor's answer for one resource;
  - credential checks were cached for 5 minutes and balances forever, where production allows 60 and 30 seconds;
  - a repository test counted every row of a shared table.

  After the rebuild, one more turned up. The harness's job runner ignored each job kind's attempt limit, and never called the dead-letter hook. So no flow could make a real dead letter, and recovering one had no flow.
- **"Exactly one call to the endpoint under test" can't prove a life.** A promise such as "an order is placed, then paid, and the app is told each time" needs each state in order. This rule used to allow that only for the lifecycle test.

**Don't.** One flow per route's scope check. Each route's scope is a handler test (TEST-15). The area's access promise is one flow over all its routes:

```go
func TestCancelOrder_ReadOnlyKeyIsRefused(t *testing.T) {
	w := testharness.New(t)
	tenant := w.NewTenant()
	order := tenant.CreateOrder(orderstest.TwoItems())

	resp, err := w.API().Test(cancelOrder(order.ID, tenant.ReadOnlyKey()))
	// ...
```

**Don't.** A flow that doesn't act as a caller. It signs in with a credential no caller can hold, seeds its customer straight into the table, always as a person, and skips the real server:

```go
func TestOrder_PlacesAnOrder(t *testing.T) {
	w := testharness.New(t)
	customer := customerstest.SeedApproved(t, w.Repo())
	token := w.TestToken(domain.EveryScope...)

	resp, err := w.TestRouter().Test(placeOrder(token, customer.ID))
	// ...
}
```

**Canonical:** a promise about one call's answer. The test sends the call itself, with the harness's body:

```go
// test/flows/orders/refusals_test.go

// An order in a currency the customer's account doesn't hold is refused
// before anything is written, and the app is told what to fix.
func TestOrder_RefusesACurrencyTheAccountDoesntHold(t *testing.T) {
	for _, customer := range testharness.Customers() {
		t.Run(customer.Name, func(t *testing.T) {
			w := testharness.New(t)
			app := w.NewTenant(domain.ModeLive, testharness.ScopesToPlaceOrders...)
			verified := app.VerifyCustomer(customer)
			account := app.OpenAccount(verified, domain.CurrencyUSD)

			resp := app.Send(http.MethodPost, "/api/v1/orders", testharness.OrderRequest(verified, account, domain.CurrencyEUR, domaintest.Item(t)))

			require.Equal(t, http.StatusBadRequest, resp.Code(), resp.BodyString())
			require.Equal(t, "currency is EUR, but account "+account.ID.String()+" holds USD", testharness.Refusal(t, resp).Message)
			require.Empty(t, app.ListOrders(verified), "nothing was written")
		})
	}
}
```

**Canonical:** a promise about a record's life. Each state the caller sees, in order:

```go
// test/flows/orders/pay_test.go

// Each kind of customer places an order and pays it. The order is PLACED,
// then PAID once the provider settles it, and the app is told ORDER_PLACED,
// then ORDER_PAID.
func TestOrder_EachKindOfCustomerPlacesAnOrderAndPaysIt(t *testing.T) {
	for _, customer := range testharness.Customers() {
		t.Run(customer.Name, func(t *testing.T) {
			w := testharness.New(t)
			app := w.NewTenant(domain.ModeLive, testharness.ScopesToPlaceOrders...)
			verified := app.VerifyCustomer(customer)
			account := app.OpenAccount(verified, domain.CurrencyUSD)

			order := app.PlaceOrder(verified, account, domain.CurrencyUSD, domaintest.Item(t))
			require.Equal(t, domain.OrderStatusPlaced, order.Status)

			w.RunQueued(jobs.KindOrderCharge, "order_id", order.ID.String())
			w.Provider().SettlesPayment(order)

			require.Equal(t, domain.OrderStatusPaid, app.ReadOrder(order.ID).Status)
			require.Equal(t, []domain.EventType{domain.EventTypeOrderPlaced, domain.EventTypeOrderPaid}, app.ToldOf(order.ID))
		})
	}
}
```

**Canonical:** a webhook through the real receiver, translation and SQL, then read through the API. The kind of customer doesn't change this answer, so the test names one:

```go
// test/flows/payments/settle_test.go

// A payment the provider settles shows on the order as paid. The settlement
// arrives as the provider's own webhook.
func TestOrder_ShowsAsPaidOnceTheProviderSettlesIt(t *testing.T) {
	w := testharness.New(t)
	app := w.NewTenant(domain.ModeLive, testharness.ScopesToPlaceOrders...)
	verified := app.VerifyCustomer(testharness.NewCustomer(domain.CustomerKindPerson, domain.CountryUS))
	account := app.OpenAccount(verified, domain.CurrencyUSD)
	order := app.PlaceOrder(verified, account, domain.CurrencyUSD, domaintest.Item(t))
	w.RunQueued(jobs.KindOrderCharge, "order_id", order.ID.String())
	w.Provider().SettlesPayment(order)

	resp := app.Send(http.MethodGet, "/api/v1/orders/"+order.ID.String(), nil)

	require.Equal(t, http.StatusOK, resp.Code(), resp.BodyString())
	require.Equal(t, domain.OrderStatusPaid, testharness.Data[schemas.Order](t, resp).Status)
}
```

**Canonical:** the lifecycle, one subtest per step.

```go
// test/flows/lifecycle/customer_lifecycle_test.go

// One customer, from the tenant's onboarding to a settled payment, with every
// step on its real path.
func TestCustomerLifecycle_FromOnboardingToASettledPayment(t *testing.T) {
	w := testharness.New(t)
	var app *testharness.Tenant
	var order schemas.Order

	// Each step needs what the steps before it made, so the test stops at the first that fails.
	require.True(t, t.Run("an operator onboards the tenant, and its owner issues the app's key", func(t *testing.T) {
		// ... one call per action, each checked, and sets app
	}))

	require.True(t, t.Run("the customer is verified, opens an account and places an order", func(t *testing.T) {
		// ... uses app, and sets order
	}))

	require.True(t, t.Run("the provider settles the payment, and the order shows as paid", func(t *testing.T) {
		// ... runs the job the order queued, then w.Provider().SettlesPayment
	}))
}
```

**Enforced by** `tools/archcheck`:
- setup through boundaries: a flow or the harness that imports a repository's seed package, calls a repository method that writes, or stands a fake in for the vendor mock; a flow that calls a database driver; and the harness running SQL that writes. `bootstrap.go` is the one place a seed goes;
- flow files that hold only tests (TEST-19);
- promises and flows that agree (TEST-18).

Review covers the rest: signing in as a caller, the production servers, the choices, the variants, and what a flow reads.

## TEST-17 · Contract tests and vendor mocks

**Rule.** Adapters are tested against an in-process vendor mock that enforces the vendor's documented contract. A contract test, run by hand, checks the vendor itself.

- **The vendor mock** lives in `cmd/tools/<vendor>-mock`, or in a `<vendor>mock` package beside the adapter.
  - A mock built on gofi, or on [gofi-proxy-mocker](https://github.com/michaelolof/gofi-proxy-mocker), lives under `cmd/tools`, because only `internal/http` and `cmd` import gofi. A mock beside the adapter uses `httptest.Server`.
  - Each route checks the request against a schema written from the vendor's documentation, not from what the adapter sends: field names, required fields, allowed values, lengths and formats.
  - A small check covers what a schema can't express, such as "this field is required when that one is set".
  - It answers a bad request in the vendor's own error shape, with the vendor's status code.
  - Each route cites the documentation page it follows.
  - It never models the vendor's business logic, such as approvals, balances, limits or duplicate checks. A test picks those answers with a mock header, such as `X-Mock-Payment-Outcome: insufficient_funds`.
  - It sends what the vendor always sends, such as the vendor's own IDs and a fresh stamp on each change. It keeps the records the vendor answers lookups from. A mock that leaves these out is kinder than the vendor. This is still the vendor's documented contract, not its business logic.
  - The service passes those headers through only in mock mode, never to the real vendor.
  - The mock has its own unit tests: one bad request per rule, and one good request per route.
  - Each test, and each flow harness, starts its own mock in the process, on its own port.
- **Contract tests** live in `test/contract`, with the `contract` tag. They run only by hand, through `task test:contract`, and never in default CI.
  - Missing credentials fail the test with a message naming the task. They never skip it (TEST-2).
  - The test refuses a production URL.
  - It records the vendor's raw answer, so a difference can be compared with the documentation.
  - Polling the vendor fails on a timeout (TEST-12).
  - What a contract test finds changes the vendor mock and the adapter together. The mock must never stay ahead of, or behind, the vendor.

**Why.**
- A mock that mirrors the adapter's assumptions lets every contract break pass every test. A schema from the vendor's documentation catches the adapter sending what the vendor refuses.
- One service compared its adapter, its vendor mock and its webhook helpers with 23 pages of its vendor's documentation, and found 44 mismatches. The adapter sent a network code the vendor refuses, so the vendor would have refused every payee on that network. The mock answered a bad request with a generic 400, where the vendor answers 422 in its own error shape. Every mismatch in the mock meant the tests checked the service against something the vendor never does.
- Business logic in a mock goes stale as the vendor changes it, and it can't produce the rare answers a test needs. A header chooses them exactly.
- A mock that sends less than the vendor hides what the service must handle. In one service, the mock's webhooks had no change stamp, so a second real change looked like a resend. The mock kept no record of one resource, so "the webhook is lost, and the service asks the vendor" couldn't be tested.
- A contract test needs a real sandbox and real credentials, and a vendor's sandbox is slow and changes on its own. So it runs by hand, and default CI stays fast and repeatable.

**Don't.** A mock that accepts whatever the adapter sends. A field the provider refuses passes every test:

```go
mux.HandleFunc("POST /v1/payments", func(w http.ResponseWriter, r *http.Request) {
	w.WriteHeader(http.StatusCreated)
	_, _ = w.Write(paymentCreatedJSON)
})
```

**Don't.** A mock that keeps balances and refuses a payment the way it guesses the provider would. Its rule drifts from the provider's, and a test can't ask it for a rare answer:

```go
if m.balances[req.Account] < req.Amount {
	return c.Send(402, insufficientFunds())
}
```

**Do.** A mock route, from the vendor's page:

```go
// cmd/tools/payments-mock/mock/payments.go

// createPaymentSchema is POST /v1/payments, as the provider documents it at
// https://docs.example.com/api/payments#create. A request that breaks it gets
// the provider's own 422.
type createPaymentSchema struct {
	Request struct {
		Header providerHeaders `validate:"required"`
		Body   struct {
			Amount    int64  `json:"amount"               validate:"required,gt=0"`
			Currency  string `json:"currency"             validate:"required,len=3,uppercase"`
			Reference string `json:"reference"            validate:"required,max=64"`
			Method    string `json:"method"               validate:"required,oneof=card bank_transfer"`
			CardToken string `json:"card_token,omitempty"`
		} `validate:"required"`
	}
	Created paymentReply
	Err     providerError
}

// cardTokenMissing is the rule the schema can't hold: a card payment needs its token.
func cardTokenMissing(method, cardToken string) bool { return method == "card" && cardToken == "" }
```

**Do.** A mock webhook with what the provider always sends, and the record it answers lookups from:

```go
// cmd/tools/payments-mock/mock/webhooks.go

// paymentSettled is the provider's webhook for a settled payment, as its
// documentation shows it: its own payment ID, and a fresh event ID and time on
// every change, so the service can tell a second change from a resend. The
// payment is kept, so GET /v1/payments/{id} answers as the provider does when
// the webhook is lost.
func (m *Mock) paymentSettled(p payment) webhook {
	m.mu.Lock()
	defer m.mu.Unlock()

	p.Status = "settled"
	m.payments[p.ID] = p

	return webhook{EventID: m.ids.Next(), OccurredAt: m.clock.Now(), Type: "payment.settled", PaymentID: p.ID}
}
```

**Do.** The mock's own test of that rule:

```go
// cmd/tools/payments-mock/mock/payments_test.go

// The provider refuses a card payment without its token, with its own 422.
func TestCreatePayment_RefusesACardPaymentWithoutItsToken(t *testing.T) {
	provider := mock.Start(t) // in this process, on a port of its own

	resp := provider.Send(t, http.MethodPost, "/v1/payments", map[string]any{
		"amount": 1000, "currency": "USD", "reference": "order-1", "method": "card",
	})

	require.Equal(t, http.StatusUnprocessableEntity, resp.StatusCode)
	require.Equal(t, providerError{Type: "validation_error", Field: "card_token"}, decodeProviderError(t, resp))
}
```

**Do.** A contract test's setup:

```go
//go:build contract

// test/contract/payments_test.go

// sandbox is the payment provider's sandbox, from the variables task
// test:contract runs with.
func sandbox(tb testing.TB) payments.Config {
	tb.Helper()

	cfg, err := config.Load[contractConfig](env.ToMap(os.Environ()))
	require.NoError(tb, err, "set the payment provider's sandbox credentials, then run task test:contract")
	require.NotEqual(tb, payments.ProductionURL, cfg.Payments.URL, "a contract test never runs against production")

	return cfg.Payments
}
```

**Enforced by** review. `tools/archcheck` checks that a contract test lives in `test/contract`, and that every file there has the `contract` tag (TEST-1). The mock's own tests prove its rules.

## TEST-18 · Tests prove what callers are promised

**Rule.** Every promise to a caller is written down, and a flow proves each one. A test's expected values come from the product, never from the code under test. A test that disagrees with the code is a finding.

- **A promise** is something a caller can rely on, written from their side. For example: "a business the provider approved can open an account", "the same idempotency key never pays twice", or "a key without the write permission is refused".
  - It comes from the API description, a feature document or a design.
  - Promises are the service's contract with its callers. They aren't TEST-17's contract tests, which check a vendor.
- **Where.** One file per area, `docs/promises/<area>.md`, beside the committed API description in `docs/openapi` (HTTP-7). Each file has:
  - who calls the area, and the sources its promises come from;
  - an **Operations** line, listing every operation the area covers as `METHOD /path`, in backticks, with the path as the API description writes it;
  - one table, one promise per row, with these columns: ID · Promise · Varies by · Edge cases a caller can meet · Proved by;
  - **"## Not reachable from outside":** each promise no caller can reach today, why, and the lower test that covers it;
  - **"## Not yet proved as the caller":** only under TEST-14's pause. Each existing flow that doesn't act as the caller yet, the promise it proves, and what it does that a caller can't. The lift empties it.

  [templates/promises.md](templates/promises.md) is the shape to copy.
- **What the files cover, together:**
  - every operation in the committed API descriptions, both the product API and the admin API;
  - every event type a caller can be sent, in an `events` area;
  - every journey the product documents, in a `journeys` area. A journey runs from nothing the caller doesn't have to what they came for. The product's main journey is the lifecycle test (TEST-16).
- **The columns:**
  - **ID:** the area's prefix and a number, such as `ORD-2`. No two promises share one, across every file.
  - **Varies by:** the choices that change the answer. The flow runs once for each.
  - **Edge cases a caller can meet:** each refusal with its status and exact message, and each failure in TEST-16's point 6.
  - **Proved by:** the flow, in backticks.
- **Expected values come from the product.**
  - The promise and the assertions are written from the API description, the feature document and the design, before anyone reads the implementation (COL-4). That covers statuses, exact messages, field names, the spelling of each value, and the order of events.
  - An expected value is never copied from what the code returned.
  - A unit test of a rule takes its expected outcome from the feature document's rule, in the same way.
- **The caller's experience is part of the promise.** A flow checks what a caller reads:
  - a refusal says what to fix, in the caller's words, and names the value it refused;
  - a refusal never blames another party for the caller's own limits;
  - the same kind of refusal has the same status in every area;
  - a temporary failure tells the caller to try again (5xx), and a bad input says what's wrong (4xx);
  - every answer spells a value the way its type does;
  - each answer gives the caller what their next step needs.

  An answer that breaks one of these is a finding, even when the outcome is right.
- **A disagreement is a finding.**
  - When a flow written from its promise fails, either the code is wrong or the promise is. Nothing else.
  - When the fix is clear, fix the code in the same change.
  - When it needs a product decision, the row's edge cases say what the caller gets today and why that's a finding, such as "(a finding: it blames the provider)". The flow asserts today's answer, so any change shows. The completion document lists the finding, and the decision it waits for (COL-9).
  - Never change an expected value to match the code without the engineer's agreement (COL-4, TEST-14).
- **The check** fails the build when:
  - a test the table names isn't a top-level test in `test/flows`;
  - a promise names no test;
  - a test the "Not reachable" or "Not yet proved" section names doesn't exist in the module;
  - a top-level test in `test/flows` is named by no promise;
  - two promises share an ID;
  - an operation in `docs/openapi` is in no area's Operations line;
  - a "Not reachable" entry gives no reason or names no test;
  - a module has flows or an API description, and no `docs/promises`.

**Why.**
- **TEST-1 alone tests the code, not the product.** Each behaviour tested once, at the lowest layer where it's real, keeps a suite fast and points straight at a broken rule. Applied alone, it has a cost nobody sees: each test confirms the code it sits beside, and nothing checks the promise a caller relies on. In one service, an approved business customer couldn't open an account or pay anyone, and about 3,800 passing tests didn't notice. The rule "an account needs a verified customer" was one core test over fakes, seeded with a person. No test ever showed a business doing it.
- **The gaps sat where two parts meet,** or between the code and what a caller was told. None of them was a bug in a rule.
- **Without a written list, "covered" meant whatever the tests happened to do.** 22 of the 38 event types that service sends its callers were never named in a flow. A written list says what callers rely on, and the check keeps the list and the tests together.
- **Flows written from the promises found answers that work, but that a caller can't use well:**
  - a refusal blamed the provider for the tenant's own limits;
  - a 403 didn't say why;
  - the same kind of refusal was 400 in one area and 409 in another;
  - a status was spelled in lower case in the first answer, and in capitals in every later one;
  - a refusal used an internal word where the caller's word is "customer".
- **An expected value copied from the code holds a bug in place.** A test is then a record of what the code did on the day it was written, and it fails only when someone fixes the bug.

**Cost.** Some behaviour is tested twice, and there are more flows. One service has 91 flows in 13 packages. Its integration tests and flows run in about 745 s on a fresh stack. That's less than its integration run took before it listed only the packages that have them (TEST-1).

**Don't.** An expected value copied from what the code returned. The message blames the provider for the tenant's own limit, and the test now holds that in place:

```go
// The expected message is what the code answered on the day the test was written.
require.Equal(t, http.StatusConflict, resp.Code(), resp.BodyString())
require.Equal(t, "the provider refused the payment", testharness.Refusal(t, resp).Message)
```

**Do.** A promise file, from [templates/promises.md](templates/promises.md). ORD-4 waits for a product decision, so its row says what the caller gets today, and why that's a finding:

```markdown
# Orders: what callers are promised

**Callers:** the tenant's app, with a key that has `orders:write` and `orders:read`; an operator; and the payment provider's webhooks.
**Sources:** the API description, and `docs/features/orders.md`.
**Operations:** `POST /api/v1/orders` · `GET /api/v1/orders/{order_id}` · `POST /api/v1/orders/{order_id}/cancel` · `GET /admin/orders/{order_id}`

| ID | Promise | Varies by | Edge cases a caller can meet | Proved by |
|---|---|---|---|---|
| ORD-1 | Each kind of customer can place an order and pay it. It's `PLACED`, then `PAID` once the provider settles it, and the app is told `ORDER_PLACED`, then `ORDER_PAID`. | person, business · each country · each currency | the provider times out: 503, and the order stays `PLACED` | `TestOrder_EachKindOfCustomerPlacesAnOrderAndPaysIt` |
| ORD-2 | An order is paid only from an account that holds its currency. | — | another currency: 400 "currency is EUR, but account {id} holds USD" | `TestOrder_RefusesACurrencyTheAccountDoesntHold` |
| ORD-3 | The same idempotency key places one order, and gives back the same answer. | — | — | `TestOrder_ARetryWithTheSameIdempotencyKeyPlacesOneOrder` |
| ORD-4 | Only the tenant's key, with the right scope, reaches its orders. | — | a read-only key placing an order: 403 · another tenant's key: 404 · a currency the tenant isn't allowed: 403 "You are not allowed to perform this action" (a finding: it doesn't say why) | `TestOrder_OnlyTheTenantsKeyWithTheRightScopeReachesItsOrders` |

## Not reachable from outside

- **`FAILED`.** The provider never refuses a payment for good once it has accepted it, so no caller can reach this status. Its rule is a core test: `TestChargeOrder_MarksTheOrderFailedWhenTheProviderRefusesForGood`.
```

**Enforced by** `tools/archcheck`: the promise files, the flows and the operations in `docs/openapi` agree, in every case the check lists. Review covers where expected values come from, and the caller's experience.

## TEST-19 · A test helper does one thing, and tests compose them

**Rule.** Every shared test helper does one thing. Tests compose them. A test never has its own copy of a step, a body, a reader or a check, and no test function is left uncalled.

- **What one thing is:**

| Helper | Does | Returns | Checks |
|---|---|---|---|
| step | one call a caller makes | what the caller sees, typed | that the service took it: its status |
| job step | the one job a call queued, run as the queue runs it | how the job ended | nothing more |
| vendor step | one thing the vendor does, such as a decision or a webhook | nothing | that the service took it |
| reader | one thing a caller can see: an answer, the events sent, what the audit store received, or storage for a storage promise | the value | nothing |
| request builder | one request's body, from the choices it's given | the body | nothing, and it sends nothing |
| journey | steps in order, for a test that needs the record and isn't testing how it's made | the last step's result | that it ended where its name says |
| builder (TEST-10) | one valid value | the value | nothing |
| seed helper (TEST-10) | one row through the repository, for a repository test | the row | nothing |
| fake (TEST-5) | what the real method does | what the real method returns | nothing |

- **A helper does nothing its name doesn't say.** A step doesn't also check an event or an audit record. That's a reader, which the test calls. A helper has no flag that switches it between two jobs, such as `PlaceOrder(tb, pay bool)`. That's a step and a journey.
- **Variations are arguments or options, not copies.** A choice the caller makes is an argument. A header for one call is an option on the actor, such as `app.WithHeaders(...)`. There's no `PlaceOrderWithX` for each variation.
- **Each kind of helper has one home:**
  - flow steps, journeys, readers, request builders and vendor steps: the harness, one file per area, such as `internal/testutils/testharness/orders.go`;
  - handler test helpers: the package's `helpers_test.go` (TEST-15);
  - fakes, builders and seed helpers: the homes in TEST-6 and TEST-10.
- **A flow file holds tests.** A file under `test/flows` declares no function or method other than `Test…` and `TestMain`. Package-level values, and types that only hold a table's rows, are fine. An area's readers live in the harness beside its steps, so the next area that needs one finds it.
- **A test composes, and never copies.**
  - When a test needs a helper that doesn't exist, it adds it to that helper's home in the same change. TEST-14 already lets a new test add what it needs.
  - When a journey would hide a step that a test needs, expose the step first. Then build the journey from it.
  - When two areas need the same thing, it's one helper, in the harness.
- **No test function is left uncalled.** Every function in test code is called, directly or not, by some test. Test code is a `_test.go` file, a package whose name ends in `test`, and anything under `internal/testutils`. A helper that only renames another is removed. TEST-15 says the same of `helpers_test.go`.

**Why.**
- **A journey held its own copy of each step.** In one service, "a verified customer" started a session, saved the details, attached the documents, submitted, and had the vendor decide, all inside one helper. A test that needed one of those steps on its own couldn't reach it, so it wrote another copy.
- **The same code was written again in each area.** Request bodies, readers and storage checks were written up to three times: a request body in two places, three copies of the storage and log checks, a reader per area, and a status check after every call to the vendor mock.
- **Nobody knew when a helper stopped being used.** After that service rebuilt its flows, a dead-code scan of its test code found 25 functions, in 12 packages, that nothing called: builders, seed helpers, counters and readers. One was the seed helper that always made a person, the one behind the gap in TEST-18.
- **Review alone didn't hold the line.** 18 functions are still declared inside its flow files, though its own guide says flows compose the harness. Eight of them do what the harness should:
  - three calls a caller makes, two of them the very calls their tests are about;
  - a journey;
  - two storage reads, one in raw SQL;
  - a reader of the events sent;
  - a request body.

  No check stopped any of them.

**Don't.** A journey that seeds rows and holds its own copy of each step:

```go
// CreatePaidOrder seeds an approved customer (always a person), writes the
// order's row and marks it paid: no request, no job, and no provider. A test
// that needs one of these steps on its own can't reach it, so it copies it.
func (w *World) CreatePaidOrder() domain.Order {
	w.tb.Helper()

	customer := customerstest.SeedApproved(w.tb, w.repo)
	order := orderstest.Seed(w.tb, w.repo, orderstest.Placed(customer.ID))
	require.NoError(w.tb, w.repo.Orders().MarkPaid(w.tb.Context(), order.ID))

	return order
}
```

**Don't.** A step written in a flow file. archcheck reports it:

```go
// test/flows/access/signin_test.go

// signIn is a step, written in a flow file, and it hides the call its tests
// are about. The step belongs in the harness. A test of sign-in sends the
// call itself.
func signIn(t *testing.T, w *testharness.World, email, password string) *gofi.InjectResponse {
	// ...
}
```

**Do.** Steps, a request builder and a journey in the harness. The flows in TEST-16 compose them:

```go
// internal/testutils/testharness/orders.go

// OrderRequest is the body that places an order for the customer, paid from
// the account, in the currency the test names. It sends nothing.
func OrderRequest(customer VerifiedCustomer, account Account, currency domain.Currency, items ...domain.Item) map[string]any {
	return map[string]any{"customer_id": customer.ID, "account_id": account.ID, "currency": currency, "items": items}
}

// PlaceOrder places an order as the tenant's app does. It checks the service
// took it, and returns what the app sees.
func (tn *Tenant) PlaceOrder(customer VerifiedCustomer, account Account, currency domain.Currency, items ...domain.Item) schemas.Order {
	tn.tb.Helper()

	resp := tn.Send(http.MethodPost, "/api/v1/orders", OrderRequest(customer, account, currency, items...))
	require.Equal(tn.tb, http.StatusCreated, resp.Code(), resp.BodyString())

	return Data[schemas.Order](tn.tb, resp)
}

// ReadOrder reads an order as the tenant's app does.
func (tn *Tenant) ReadOrder(id domain.OrderID) schemas.Order {
	tn.tb.Helper()

	resp := tn.Send(http.MethodGet, "/api/v1/orders/"+id.String(), nil)
	require.Equal(tn.tb, http.StatusOK, resp.Code(), resp.BodyString())

	return Data[schemas.Order](tn.tb, resp)
}

// PaidOrder is an order the provider has settled. It's only the steps in
// order, for a test that needs a paid order and isn't testing how it's paid.
func (tn *Tenant) PaidOrder(customer VerifiedCustomer, account Account, currency domain.Currency) schemas.Order {
	tn.tb.Helper()

	order := tn.PlaceOrder(customer, account, currency, domaintest.Item(tn.tb))
	tn.w.RunQueued(jobs.KindOrderCharge, "order_id", order.ID.String())
	tn.w.Provider().SettlesPayment(order)

	paid := tn.ReadOrder(order.ID)
	require.Equal(tn.tb, domain.OrderStatusPaid, paid.Status, "PaidOrder ends with the order paid")

	return paid
}
```

**Enforced by** `tools/archcheck`: a file under `test/flows` declares only tests and `TestMain`, and every function and method in test code is reached from a test, with every kind's tag set. Like `deadcode -test`, the check follows calls from every test binary's entry points. It doesn't report a method a type needs for an interface, because deleting it would break the build. Unused types, fields and constants, one thing per helper, no copies, and variations as arguments are for review.
