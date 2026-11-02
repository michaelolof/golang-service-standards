# 01 · Project layout, packages, bootstrap and config

How a service repository is laid out, how its packages are named and divided, how a binary starts and stops, and how configuration reaches code.

## Rules at a glance

| ID | Rule | Enforced by |
|---|---|---|
| LAY-1 | Application code lives in `cmd/` and `internal/`. `pkg/` holds only libraries being incubated for extraction, and they import nothing from `internal/`. | arch test, depguard |
| LAY-2 | `main` only sets up signals, calls `run(ctx, …) error`, and exits. | gocritic `exitAfterDefer`, revive `deep-exit`, arch test |
| LAY-3 | One `wire.go` per binary builds every dependency at startup and returns errors. Nothing looks dependencies up at request time. | arch test, review |
| LAY-4 | Long-running components expose `Run(ctx) error` and run under `supervise.Run` (CON-2). | review |
| LAY-5 | Migrations run only from a command in `cmd/tools/`. Services check the schema version and fail closed. | review |
| LAY-6 | `internal/` uses one taxonomy: foundation, config, domain, ports, core, repository, adapters, http, workers. | arch test |
| LAY-7 | A package imports only the layers its LAY-6 row allows. Each SDK has one home. | arch test, depguard |
| LAY-8 | The package name equals the directory name. Choose names so importers never need an alias. | staticcheck ST1003, arch test |
| LAY-9 | Name packages for what they do: never `util`, `helpers`, `common`, `types` or `models`. Place a helper by who uses it. | revive, arch test |
| LAY-10 | Name files after their type or use-case. Target ≤ 500 lines; hard limit 1,000. | revive `file-length-limit`, arch test |
| LAY-11 | Every package has a doc comment. No `README.md` under `internal/` or `cmd/`. | staticcheck ST1000, arch test |
| LAY-12 | Only `internal/config` reads the environment, through an injected lookup. | forbidigo, depguard |
| LAY-13 | Config is typed section structs with `Validate()`. Each binary composes its own sections, and each component takes its own small `Config`. | review, config test |
| LAY-14 | Taskfile has the standard targets. `check` is read-only and is exactly what CI runs. | CI |

---

## LAY-1 · Repository root

**Rule.**
- Application code lives in `cmd/` and `internal/`.
- `pkg/<name>` holds only code being incubated as a standalone library. Such code is written as if it were already external, on its way to becoming its own module like `errx`, `gofi` and `enumx`. A database-backed queue is a typical example.
- A `pkg/` package:
  - imports nothing from `internal/`;
  - takes everything it needs through its own API: its own `Config`, an injected `*slog.Logger`, and hooks. It never uses the service's config, logger or `errkinds`;
  - may use stdlib errors (`fmt.Errorf`, sentinel errors) instead of errx, because its future users won't all be services that follow this standard (ERR-12);
  - has its own `doc.go` or `README.md` and its own tests. If it owns a schema, it also has its own migrations (`pkg/<name>/migrations`).
- `pkg/` is not a place for shared application code. Anything that knows this service's domain belongs in `internal/`.
- The only other Go outside `cmd/`, `internal/` and `pkg/` is the one-file `//go:embed` shim in `db/migrations/`.

**Why.** A repo holds two kinds of code:

| Kind | Where | API stability | May know the service's domain? |
|---|---|---|---|
| Application code | `internal/` | Private; changes freely | Yes |
| Library code | `pkg/` | Stable | No |

Keeping them in separate folders makes the difference visible and lets a tool enforce it. Extraction later is then a `git mv`, not a refactor.

```
cmd/services/<role>/        deployable long-running processes (one per process type)
cmd/tools/<name>/           one-shot CLIs: migrate, openapi, seed, dev mocks
internal/                   all application code (LAY-6)
pkg/<name>/                 libraries being incubated for extraction; import nothing from internal/
db/migrations/              migration files + embed.go (package migrations, exports its embed.FS only)
docs/                       openapi/ (generated, committed), runbooks/, adr/
deploy/                     platform entrypoints, compose overrides, release scripts
scripts/                    shell helpers called only from the Taskfile
test/{e2e,contract,perf}/   cross-binary black-box suites (build-tagged)
web/                        built dashboard assets (embedded by the admin API)
go.mod  Taskfile.yml  .golangci.yml  Dockerfile
docker-compose.{local,test}.yml  .env.example  README.md
```

**Do.** The library guards its own independence with a test:

```go
for _, imp := range f.Imports {
	importPath := strings.Trim(imp.Path.Value, `"`)
	// Block any service-internal imports.
	if strings.Contains(importPath, "example.com/myservice/internal/") {
		t.Errorf("%s imports service-internal package %q — core must be extraction-ready", path, importPath)
	}
}
```

The standard makes this check universal, as a depguard rule (`pkg/**` may not import `<module>/internal`) and an architecture-test row.

**Do.** An embed shim with no logic:

```go
package migrations

import "embed"

//go:embed *.up.json
var Files embed.FS
```

**Don't.** An `internal/config` package that holds only a doc comment, and whose name collides with the real config package at `internal/core/config`.

---

## LAY-2 · `main` does four things

**Rule.** `main` does exactly four things, and everything else happens in `run`:

1. Create the signal context.
2. Call `run(ctx, …) error`.
3. Log the error.
4. Exit non-zero on failure.

**Why.**

- `os.Exit` skips deferred calls. Calling it after `defer` leaks resources, or forces cleanup to be duplicated by hand.
- With `run` returning an error, there is a single error path.

**Don't.** `os.Exit` skips defers, so cleanup is copied by hand:

```go
archiver, err := archival.NewArchivalWorkerApp(env)
if err != nil {
	logger.WrapErrorf(1793625652700, err, "error setting up archival worker")
	worker.Close()
	scheduler.Close()
	os.Exit(1)
}
defer archiver.Close()
```

**Do:**

```go
// Command api serves the producer-facing HTTP API.
package main

func main() {
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, env.ToMap(os.Environ())); err != nil {
		logger.Error(1793433180003, ctx, "process.run.failed", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, environ map[string]string) error {
	cfg, err := config.Load[apiConfig](environ)
	if err != nil {
		return errx.Wrap(1791284920717, err)
	}

	logger.Initialize(cfg.Log)

	app, cleanup, err := wire(ctx, cfg)
	if err != nil {
		return err
	}
	defer cleanup()

	return supervise.Run(ctx, cfg.App.ShutdownTimeout,
		supervise.Component{Name: "http", Run: app.http.Run},
		// A binary with more components lists each one here (LAY-4, CON-2).
	)
}
```

Every command starts with a `// Command <name> …` doc comment.

---

## LAY-3 · Wire everything in one place, at startup

**What "composition root" means.** A service needs concrete objects created and connected to each other:

- a database client;
- the repositories that use it;
- the core services that use the repositories;
- the handlers and workers that use the services.

The composition root is the one place where this happens. Every other piece of code only *receives* what it needs through its constructor (ABS-5). It never looks a dependency up by itself.

**Rule.**
- Each binary has a `cmd/services/<role>/wire.go` file in package `main`. It holds one function:

  ```go
  func wire(ctx context.Context, cfg <role>Config) (*app, func(), error)
  ```

- `wire` builds things in dependency order:
  1. adapters;
  2. repositories;
  3. core services;
  4. handlers and workers;
  5. servers.

  It returns the runnable components, plus a cleanup function that closes what it opened.
- Every constructor `wire` calls returns an error, and `wire` returns that error. Nothing panics during wiring.
- HTTP handlers get their dependencies through gofi's `deps` accessor (HTTP-1, gofi DEP-1 to DEP-5). `wire` builds the `Deps` value completely and checks it before registering it: `deps.New(...)` returns an error if any field is nil. So `deps.MustGet(c)` can never hand a handler a missing dependency in a binary that started.
- Workers and other components receive their dependencies through their constructors.
- A startup sequence repeated across binaries (connect to Mongo, check the schema version) becomes one constructor in the adapter, such as `mongostore.Open(ctx, cfg)`. Each `wire.go` calls it.

**Why.** Compare what happens when a dependency is forgotten.

**Don't.** Nothing checks the container at startup, so each accessor checks its own field at request time:

```go
// fallback: an unwired store would silently accept writes that vanish.
func (d AppDeps) ProducerKeys() producerkeys.Store {
	if d.ProducerKeyStore == nil {
		panic("deps.AppDeps.ProducerKeys: ProducerKeyStore not wired at startup")
	}
	return d.ProducerKeyStore
}
// handler: d := deps.MustGet(c); d.ProducerKeys().Create(...)
```

One such container had 27 accessors like this. If one field is left unset, the binary still starts, passes its health check and deploys. The first customer request to that endpoint then panics.

The accessor isn't the problem; the missing startup check is. With `deps.New` validating every field, the same mistake fails at boot, before any traffic arrives. Tests build the same `Deps` from fakes (TEST-11).

**Don't.** A server that fails to start, for example because its port is taken, only logs the error. The process then sits unready until SIGTERM. Under LAY-4, `Run` returns that error and the process exits.

```go
go func() {
	if err := srv.Start(); err != nil {
		logger.Wrapf(1793625653716, err, "server stopped")
	}
}()

<-quit
```

**Canonical:**

```go
// cmd/services/api/wire.go
type app struct {
	http *server.Server
}

func wire(ctx context.Context, cfg apiConfig) (*app, func(), error) {
	db, err := mongostore.Open(ctx, mongostore.Config{URI: cfg.Mongo.URI, Database: cfg.Mongo.Database})
	if err != nil {
		return nil, nil, errx.Wrap(1792619979211, err)
	}
	cleanup := func() { _ = db.Close(context.WithoutCancel(ctx)) }

	keys := producerkeys.New(db)
	keySvc, err := apikeys.New(apikeys.Deps{Store: keys, Clock: clock.System(), IDs: ids.ULIDs()})
	if err != nil {
		cleanup()
		return nil, nil, errx.Wrap(1791684665718, err)
	}

	d, err := deps.New(deps.Config{APIKeys: keySvc, ProducerKeys: keys})
	if err != nil { // a nil field fails here, at startup
		cleanup()
		return nil, nil, errx.Wrap(1791522379440, err)
	}

	router := gofi.NewRouter()
	deps.Register(router, d)
	routes.Register(router)

	return &app{http: server.New(server.Config{Addr: cfg.HTTP.Addr}, router)}, cleanup, nil
}
```

`wire.go` is the only file that names concrete adapters, repositories and services together. An arch test checks this: no package outside `cmd/**` imports both `internal/config` and `internal/adapters/**`.

---

## LAY-4 · Components expose `Run(ctx) error`

**Rule.**

- Every long-running component (HTTP server, consumer, scheduler, worker pool) has `Run(ctx context.Context) error`. It:
  - blocks until `ctx` is done;
  - drains within the shutdown timeout, then returns `nil`;
  - returns an error if it fails;
  - never returns `nil` early.
- `run` starts components under `supervise.Run` (CON-2), so one failure stops the process.
- Signals come only from `signal.NotifyContext(ctx, os.Interrupt, syscall.SIGTERM)`.
- There is one `SHUTDOWN_TIMEOUT`, default `25s`. Keep it shorter than the time your platform waits before killing the process.
- The drain context is derived with `context.WithoutCancel(ctx)`.
- A binary that combines several processes is simply a `run` that starts several components.

**Why.**

- Without one shape, signal handling drifts: `NotifyContext` here, a `signal.Notify` channel there, and both somewhere else.
- Shutdown deadlines end up hard-coded at different values, such as 5, 10, 15 and 25 seconds.
- One lifecycle shape means one way to read every binary.

---

## LAY-5 · Migrations run at release

**Rule.**

- Schema migrations run only from a command in `cmd/tools/`, usually `cmd/tools/migrate`, as the release step.
- Services check schema-version compatibility at startup and refuse to start on a mismatch.

**Why.** Running migrations in every binary at startup means several processes race to migrate during a deploy. It also mixes deploy concerns into serving code.

---

## LAY-6 · One `internal/` taxonomy

Without one taxonomy, the same folder name drifts. `core/` can end up meaning any of three things:

- use-cases mixed with utilities (`clock`, `ids`, `coreutils`);
- utilities only, with the use-cases in `service/`;
- business engines mixed with infrastructure (`observability`, `supervise`).

Interfaces can also end up in three places: `internal/ports`, `domain/*_contracts.go` and `domain/ports`. The standard fixes one meaning per folder:

| Folder | Holds | May import (internal) |
|---|---|---|
| `foundation/<pkg>` | Business-agnostic building blocks: logger setup, telemetry, ids, clock, errkinds, pagination, canonicaljson | foundation |
| `config` | Turns the environment into typed config sections. The only reader of the process environment | foundation |
| `domain` (flat) | Entities, value objects, enumx enums, invariants, domain errors. No I/O, no infrastructure interfaces | foundation |
| `ports` (flat) | Interfaces consumed by **two or more** `core` packages, plus their argument and result types. No constructors | domain, foundation |
| `core/<capability>` | Use-cases and business rules for one capability. Declares the interfaces only it consumes | domain, ports, foundation, other core (acyclic) |
| `repository/<aggregate>` | Persistence for one aggregate in the service's primary database | domain, ports, foundation, `adapters/<primarydb>`, and the `core/<capability>` whose interface it implements (LAY-7) |
| `adapters/<tech><role>` | Clients for every other external system: broker, cache, object store, KMS, email, chains, custody | domain, ports, foundation, and the `core/<capability>` whose interface it implements (LAY-7) |
| `http/…` | Inbound HTTP: `server`, `routes`, `deps`, `middleware`, `httperr`, `schemas`, `<api>/<resource>` handlers | core, repository, domain, ports, foundation |
| `workers/<name>` | Inbound async work: queue jobs, stream consumers, schedulers | core, repository, domain, ports, foundation |
| `testutils/<name>` | Shared test infrastructure | anything; imported only from `_test.go` |
| `architecture` | Tests that encode this table | none |
| `cmd/**` | Composition root | anything |
| `pkg/<name>` (outside `internal/`) | Libraries being incubated for extraction (LAY-1) | other `pkg/`; never `internal/` |

**Interfaces live with their consumer.** `ports/` exists only for the shared case. Its package comment (`internal/ports/doc.go`) says so:

```go
// A contract belongs here only when several capability packages depend on the
// same business boundary and no single capability is its natural owner. ...
// single-consumer interfaces stay beside their consumer.
//
// This package contains contracts and their argument/result types only. It is
// flat and must not contain concrete implementations, constructors, wiring or
// infrastructure SDK types.
package ports
```

**Transports call a service or a repository, and hold no business rules.** A handler or worker makes one call: to the capability's service when it has a justified one (HTTP-3), otherwise to a repository method. Transports never import `adapters/*`: talking to an external system is a service's job. What goes wrong is not the import itself but business rules leaking into handlers:

**Don't.** A business rule inside a handler:

```go
now := time.Now().UTC()
if s.Request.Body.ExpiresAt.Before(now) {
	return httperr.New(1786665379648, httperr.InvalidKeyScopeErr, "expires_at must be in the future")
}
if s.Request.Body.ExpiresAt.After(now.Add(365 * 24 * time.Hour)) {
```

That expiry rule belongs in `domain` (a pure function the handler or service calls) or in the service, never inline in the handler. Plain reads don't need a `core` function: the handler calls the repository.

**Where folders with other names go:**

| Folder | Becomes |
|---|---|
| `service/*` | `core/*` |
| Utilities in `core/*`, such as `core/{clock,ids,…}` or `core/observability` | `foundation/*` |
| `*/core/httperr` | `http/httperr` |
| `edge`, `platform` (inbound HTTP) | `http/edge`, `http/platform` |
| `schedulers`, `archival`, `processor` | `workers/*` |
| `domain/*_contracts.go`, `domain/ports` | consumer package, or `ports/` if shared |

---

## LAY-7 · Imports follow the layer table, and SDKs stay at the edge

**What the rule means.** A package may import only the layers listed in its "May import" column in LAY-6. Two consequences matter most.

**1. Business logic never imports infrastructure.**
- `core/transfer` can't import `repository/transfers`, pgx or gofi.
- To persist a transfer, core declares the small interface it needs (ABS-1).
- The repository implements that interface, and `wire.go` connects the two:

```
internal/core/transfer/store.go        type Store interface { Get(...); UpdateStatus(...) }    declares the need
internal/repository/transfers/repo.go  var _ transfer.Store = (*Repo)(nil)  // uses pgx         imports core/transfer, implements it
cmd/services/api/wire.go               transfer.New(transfer.Deps{Store: transfers.New(db)})    imports both, connects them
```

The import arrow runs from `repository` to `core`, never the reverse. This is why the LAY-6 table lets repositories and adapters import the `core` package whose interface they implement, but never lets `core` import them.

It pays off in three ways:
- Core tests run against an in-memory fake, with no database (TEST-11).
- Changing a column, or switching database driver, touches only `repository/`.
- The compiler stops a handler or service from bypassing the business rules, so a reviewer doesn't have to spot it.

**2. Each SDK has one home.**

| Package | May be imported only in |
|---|---|
| mongo-driver, pgx, gocql, go-redis, franz-go, asynq, aws-sdk, minio | `adapters/**`, `repository/**`, `cmd/**` (plus `pkg/` libraries that wrap them, such as a database-backed queue over pgx) |
| gofi, fasthttp | `http/**`, `cmd/**` |
| `internal/config` | `cmd/**` and tests |
| `<module>/internal/...` | never from `pkg/**` (LAY-1) |

If `gocql` appears in an import list anywhere else, lint fails. The fix is always the same: convert at the boundary. The repository returns domain types with typed IDs (DOM-1), and the layer above never sees the driver.

**Why.** Without these rules, layers leak into each other:
- `domain` imports `core/coreutils` or `core/config`.
- `workers` imports `http/server`.
- Nine `core` packages import `repository`, and `domain/ports` imports `repository`.

A "port" that imports the concrete repository isn't a port. Its callers depend on the database's record shape and error values anyway.

**Don't:**

```go
import "example.com/myservice/internal/repository"

type SystemConfigStore interface {
	// Returns gocql.ErrNotFound when the key does not exist.
	GetConfig(ctx context.Context, key string) (repository.SystemConfigRecord, error)
```

**Do.** The same contract, declared by its consumer, in domain types and kinds only:

```go
// internal/core/systemconfig/store.go
type Store interface {
	// Get returns errkinds.ErrKindNotFound when the key does not exist.
	Get(ctx context.Context, key domain.ConfigKey) (domain.SystemConfig, error)
}
```

**Enforced by.**
- **depguard** for SDK confinement ([lint config](lint/.golangci.yml)).
- **One table-driven architecture test** over `go list -json ./...`, in `internal/architecture/boundaries_test.go` and copied into every project:

```go
var layers = []struct {
	pkg       string   // package prefix under the module
	mayImport []string // allowed internal prefixes (stdlib and third party are checked by depguard)
}{
	{"internal/foundation", []string{"internal/foundation"}},
	{"internal/domain", []string{"internal/foundation"}},
	{"internal/ports", []string{"internal/domain", "internal/foundation"}},
	{"internal/core", []string{"internal/core", "internal/ports", "internal/domain", "internal/foundation"}},
	{"internal/repository", []string{"internal/core", "internal/ports", "internal/domain", "internal/foundation", "internal/adapters"}},
	{"internal/adapters", []string{"internal/core", "internal/ports", "internal/domain", "internal/foundation"}},
	{"internal/http", []string{"internal/core", "internal/repository", "internal/ports", "internal/domain", "internal/foundation", "internal/http"}},
	{"internal/workers", []string{"internal/core", "internal/repository", "internal/ports", "internal/domain", "internal/foundation"}},
	{"pkg", []string{"pkg"}},
}
```

---

## LAY-8 · Package naming

**Rule.**

- The package name equals the last path element: lowercase, one word, no underscores.
- Name packages so importers never need an alias. Alias only when two imports in one file collide.
- If importers habitually alias your package, rename it to the alias.
- Adapter packages are named `<tech><role>` (`mongostore`, `kafkastream`, `redisqueue`, `s3store`), so they never collide with the SDK they wrap.
- The test-helper package for `x` is `xtest`, beside `x` (as `net/http/httptest` is).

**Why.** Habitual aliases show packages named for their author, not their importer:

- The alias `mongoadapter` appears on 47 imports.
- The alias `queuejobs` appears on 57 imports.
- `adapterredis` and `s3adapter` are both in use, with their words in opposite orders.

One codebase has 13 directories whose package name differs from the folder (`core/time` is `package apptime`). A typo becomes permanent API: `repository/executionchekpoints`, imported as `providerstepsrepo`.

```
internal/adapters/mongostore/                    package mongostore
internal/adapters/kafkastream/kafkastreamtest/   package kafkastreamtest
internal/repository/executioncheckpoints/        package executioncheckpoints
internal/http/admin/wallets/                     package wallets
```

---

## LAY-9 · Name packages for what they do

**Rule.**
- A package name says what the package provides.
- These generic names are banned: `util(s)`, `helper(s)`, `common`, `shared`, `misc`, `base`, `types`, `models`, `interfaces`, and any `*utils`.
- Place a helper according to who uses it:
  1. **One package uses it:** an unexported function in that package, in the file that uses it.
  2. **Several packages use it:** a small package named for its purpose, under `foundation/` (for example `foundation/enumtag` or `foundation/ptr`). All helpers sit in that one folder, so they're easy to find. They don't have to share a package.
  3. **Several repos need it:** each repo keeps it in its own `foundation/`. Moving it into a shared library is optional.
- Don't create a package for a single helper unless case 2 applies.

**Why.** A name like `utils` tells the importer nothing about what's inside. So every new helper lands there whatever its topic, and the package grows into something everything imports. One `core/coreutils` package holds one function and has 49 importers, and nothing about its name says what that function does.

A purpose-named package stays small and cohesive, and the call site reads well:

| Call site | Says what it does? |
|---|---|
| `enumtag.Oneof(values)` | Yes |
| `coreutils.JoinSpace(values)` | No |

**Example.** Two projects each have a `coreutils` package holding one function, and both functions do the same job: join enum values into a gofi `oneof` tag.

```go
func OneofValues[T stringable](values iter.Seq[T]) string {
func JoinSpace[K stringable, V any](i iter.Seq2[K, V]) string {
```

Within one repo, that helper becomes a purpose-named foundation package. It also sorts, because `Values()` comes from a map and has no fixed order (DOM-6):

```go
// Package enumtag builds gofi validation tags from enumx values.
package enumtag

// Oneof returns values as a sorted, space-separated list for a gofi `oneof` tag.
func Oneof[T ~string](values iter.Seq[T]) string {
	sorted := slices.Sorted(values)
	parts := make([]string, len(sorted))
	for i, v := range sorted {
		parts[i] = string(v)
	}
	return strings.Join(parts, " ")
}
```

---

## LAY-10 · File naming and size

**Rule.**

- Name a file after its main type or use-case (`create.go`, `wallet.go`, `transfer_transitions.go`).
- `<pkg>.go` holds the package's main type and its constructor.
- Split files by use-case, not by kind. `helpers.go`, `utils.go`, `common.go` and `types.go` are banned.
- Target 500 lines or fewer. The hard limit is 1,000 non-blank, non-comment lines.
- Test files mirror their source: `create.go` pairs with `create_test.go`.

**Why.** Across three codebases, only 6 of about 1,360 non-test files exceed 1,000 lines, so the limit costs little. The ones that do are the hardest code to review. One `repository/wallets/operations.go` is 2,315 lines, and a single function in it is about 286 lines.

**Do.**

- `core/wallet/{create,lifecycle,balance,routing}.go`.
- `http/admin/apikeys/{create,list,revoke,rotate}.go`. An architecture test can also check "one handler per file".

---

## LAY-11 · Package docs, not READMEs

**Rule.**

- Every package has one package comment. Put it in `doc.go` once it runs past one sentence.
- The comment states what the package owns and what it must not import.
- No `README.md` under `internal/` or `cmd/`. READMEs are allowed only at the repo root and in non-Go directories.

**Why.** `go doc` renders doc comments, and reviewers see them change in the same diff. READMEs rot:

- An `internal/core/README.md` says "Current use case: `verification/`" while `core/` has more than 20 subpackages.
- A `cmd/README.md` describes a "5-binary model" for 8 binaries.

Without this rule, package comment coverage varies widely. Across three codebases, 106 of 114, 78 of 123 and 11 of 91 packages had a doc comment.

**Do:**

```go
// Package domain contains pure domain types and invariants.
//
// This package is intentionally flat. It is split by files, not subpackages.
// It may use the standard library, foundation packages, errx, and enumx.
//
// It must not import HTTP, Gofi, MongoDB, Kafka, repositories, workers,
// or concrete adapters.
package domain
```

---

## LAY-12 · Only `internal/config` reads the environment

**Rule.**

- `internal/config` parses from an injected `map[string]string` or lookup function, never from `os.Getenv` directly.
- `.env` files are loaded by the Taskfile's `dotenv:` option, never by Go code.
- `godotenv` may appear only in `_test.go` files.

**Why.** An injected source makes config tests deterministic and keeps process-environment mutation out of tests. When `LoadEnv()` reads `os.Getenv` directly, its tests have to rewrite the process environment with `godotenv.Overload`, and that helper ends up shipping in non-test code.

**Do:**

```go
type Lookup func(key string) (string, bool)

func osLookup(key string) (string, bool) { return os.LookupEnv(key) }

func Parse(lookup Lookup) (Config, error) {
```

---

## LAY-13 · Typed config sections, composed per binary

**Rule.**

- **Sections:** config is split into section structs, one file per section (`mongo.go`, `kafka.go`, `http.go`). Each field declares its env name and default in a tag, and each section has `Validate() error`.
- **Real types:** use `time.Duration`, `[]string`, and enumx types that implement `UnmarshalText`. A unit belongs in the type, not the variable name (`WORKER_BLOCK_MS` as an `int`).
- **Per-binary composition:** each binary composes its sections in `cmd/services/<role>/config.go`. What is required follows from what is included, which replaces `ValidateFor(service)` switches.
- **Errors:** `Load` returns every error, joined. There are no `Must*` functions and no panics.
- **Secrets** implement `slog.LogValuer` and `String()` so they are redacted.
- **Injection:** components declare their own small `Config` struct, and `wire.go` maps sections onto it.
  - No component receives the whole app config.
  - Only `cmd/**` imports `internal/config`.

**Why.**

- One flat `Config` grew to about 190 fields in 1,101 lines, plus a 230-line `ValidateFor(service)` switch.
- Passing a whole `*config.Env` around spread it into 29 non-test files, including adapters and core.
- Parse helpers that swallow errors turn a typo into a wrong value.

**Don't.** `S3_SECURE=ture` silently becomes `false`:

```go
func parseBool(s string) bool {
	b, _ := strconv.ParseBool(s)
	return b
}
```

**Don't.** An adapter depends on the whole app config:

```go
func NewMailer(env *config.Env) (ports.Mailer, error) {
```

**Canonical.** This uses `caarlos0/env/v11`.

```go
// internal/config/mongo.go
type Mongo struct {
	URI            string        `env:"MONGO_URI,required"`
	Database       string        `env:"MONGO_DATABASE,required"`
	ConnectTimeout time.Duration `env:"MONGO_CONNECT_TIMEOUT" envDefault:"10s"`
}

func (m Mongo) Validate() error {
	if m.ConnectTimeout <= 0 {
		return errx.New(1792780856183, "MONGO_CONNECT_TIMEOUT must be positive")
	}
	return nil
}

// cmd/services/api/config.go
type apiConfig struct {
	App   config.App
	Log   config.Log
	HTTP  config.HTTP
	Mongo config.Mongo
	Kafka config.Kafka
	S3    config.S3
}

func (c apiConfig) Validate() error {
	return errors.Join(c.App.Validate(), c.Log.Validate(), c.HTTP.Validate(), c.Mongo.Validate(), c.Kafka.Validate(), c.S3.Validate())
}

// cmd/services/api/wire.go: map sections onto the component's own Config
store, err := s3store.New(ctx, s3store.Config{Endpoint: cfg.S3.Endpoint, Region: cfg.S3.Region})
```

**Enforced by.**

- forbidigo `^os\.(Getenv|LookupEnv|Environ|Setenv)$` outside `internal/config/` and `cmd/`.
- depguard on `internal/config` and `godotenv`.
- A config test asserting that every `env:"…"` tag appears in `.env.example`, and vice versa.

---

## LAY-14 · Taskfile with standard targets

**Rule.**

- Use Taskfile v3. Name a task for what it does (`lint`, `test`), and use `area:detail` when there's more than one of a kind (`test:integration`, `infra:up`). Give every task a `desc`.
- Only `fmt`, `generate` and the setup tasks (`setup`, `agents`, `lint:config`) write files.
- `check` is read-only and is exactly what CI runs.
- Pin tools with `tool` directives in `go.mod` and run them with `go tool …`, so a missing tool fails instead of degrading to a weaker check. The exception is golangci-lint: `Taskfile.standards.yml` pins and builds it, because it needs the standard's plugin and would add about 200 lines to a project's `go.mod` (tested).
- Include the standard's own tasks from `.standards/Taskfile.standards.yml`, instead of writing them (ADO-1). `check` depends on `standards:check`, and `check:fast` on `standards:check:fast`, so a check added to the standard reaches every project when it upgrades.
- `check:fast` is what the hooks run (ADO-4), so it stays quick.

**Why.**

- A `verify` task runs `format`, which rewrites files, and skips `lint`.
- A `lint` task runs `gofmt -w` first. Its `.golangci.yml` pins `go: "1.24"` while `go.mod` says 1.27.3, and its `revive.rules` block is mis-indented, so the rules it configures are never applied.
- Another `lint` task silently falls back to `go vet` when golangci-lint is missing, and `.githooks/pre-push` is entirely commented out.

```yaml
version: "3"

vars:
  STANDARDS_URL: <standards-repo-url>

includes:
  standards:
    taskfile: .standards/Taskfile.standards.yml
    dir: .
    flatten: true

tasks:
  arch:             {desc: Architecture rules,                      cmds: [go test ./internal/architecture/...]}
  test:             {desc: Unit tests, no infra,                    cmds: [go test -race ./...]}
  test:integration: {desc: Integration tests against infra,         cmds: [go test -race -tags integration ./...]}
  test:e2e:         {desc: Black-box suites in ./test,              cmds: [go test -tags e2e ./test/...]}
  build:            {desc: Build all binaries into bin/,            cmds: [go build -o bin/ ./cmd/...]}
  generate:         {desc: go generate + OpenAPI snapshots,         cmds: [go generate ./..., go run ./cmd/tools/openapi -out docs/openapi]}
  generate:check:   {desc: Fail if generated files are stale,       cmds: [go run ./cmd/tools/openapi -check -out docs/openapi]}
  vuln:             {desc: govulncheck,                             cmds: [go tool govulncheck ./cmd/...]}
  check:fast:       {desc: What the hooks run (the standard's local checks and unit tests), deps: [standards:check:fast, test]}
  check:            {desc: CI gate (read-only), deps: [standards:check, arch, test, build, generate:check, vuln]}
  infra:up:         {desc: Start local deps,   dotenv: [.env.local], cmds: [docker compose -f docker-compose.local.yml up -d --wait]}
  infra:down:       {desc: Stop local deps,    cmds: [docker compose -f docker-compose.local.yml down]}
  migrate:up:       {desc: Apply migrations,   dotenv: [.env.local], cmds: [go run ./cmd/tools/migrate up]}
  run:api:          {desc: Run api locally,    dotenv: [.env.local], cmds: [go run ./cmd/services/api]}
```

**Enforced by.**

- CI runs `task check` and nothing else.
- `golangci-lint config verify` catches mis-shaped configs.
