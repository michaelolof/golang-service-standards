# analyzers

The standard's structural checks: the rules golangci-lint's own linters can't express, each a `go/analysis` analyzer compiled into the same golangci-lint as `requiredtag` and `wrapgaps` (TEST-13). A project writes no checking code of its own, and keeps no `internal/architecture`. The checks that need the whole module, or files that aren't Go, are in [tools/archcheck](../../tools/archcheck).

## What each one checks

| Analyzer | Rules | Reports | Leaves to |
|---|---|---|---|
| `layers` | LAY-1, LAY-6, LAY-7, TEST-13 | Go outside `cmd/`, `internal/`, `pkg/`, `db/migrations` and `test/`; a package under `internal/` in no LAY-6 layer, or below a flat one; a constructor in `ports`; an import the layer table doesn't allow; an SDK outside its own adapter; test code imported outside tests | depguard: SDKs outside adapters and repositories, gofi outside `http`, `internal/config` outside `cmd`, `pkg/` importing `internal/` |
| `cmdshape` | LAY-2, LAY-3 | anything but `main` and `run` in `main.go`; a service with no `wire.go`, a `wire.go` with another function, a `wire` with another signature | gocritic `exitAfterDefer`, revive `deep-exit` |
| `names` | LAY-9, LAY-10, ABS-3, ABS-6, STY-2 | a package named `…util(s)` or `…helper(s)`; `helper(s).go`, `util(s).go`, `common.go`, `types.go`; `I<Name>` interfaces and `…Iface`, `…Interface`, `…Impl`, `…Adapter`, `…Manager`, `…Helper` types; a service method's input named `…Params` or `…Arg`; `GetX()` getters; `Build…` and `Make…` constructors; error variables that aren't `err` or `…Err` | revive: the exact names `util(s)`, `helper(s)`, `common` and the rest, and receiver names |
| `enums` | DOM-6 | an enumx enum in `domain` without a string kind, a `nil` fallback, `Parse<T>`, `Validate` and `Oneof`; a variant not named `<Type><Variant>`; wire values that aren't UPPER_SNAKE, once per type | review: whether `Oneof` sorts |
| `services` | ABS-5 | a core `Service` not built by exactly `func New(d Deps) (*Service, error)`, with exported fields, or with `With…` methods | review: no-op defaults, when to split |
| `adapters` | ABS-7 | `New` or `Open` not taking the package's own `…Config` and returning `(*T, error)`, or taking a `ctx` it never uses; an SDK type in an adapter's exported API, except a store adapter's database driver | `layers`: where each SDK is imported |
| `generics` | ABS-8 | a type switch on a value of a type parameter | review: when generics fit |
| `errkinds` | ERR-5 | an `errx.Kind` outside `foundation/errkinds`, not a package variable, not named `ErrKind<Name>`, or with a string that isn't a unique snake_case literal | `tools/archcheck`: kinds nothing creates or checks |
| `gofihandlers` | HTTP-1, HTTP-2, HTTP-5 | gofi's GUIDELINES.md: a handler not named `<Operation>Handler`, two in a file, fields out of order, its schema not an unexported `<operation>Schema` directly below, `c.Param`, `c.Query` or `c.HeaderVal` in it; route info without one kebab-case tag naming the package, without a description, or with an `OperationId` | `tools/archcheck`: field documentation in the OpenAPI document |
| `errmap` | ERR-7 | a handler, or a handler helper, returning an error that isn't `nil`, from `httperr`, from gofi's context, or from another helper | forbidigo: `UseIssueAsMsg` outside `httperr` |
| `jobs` | JOB-1, ERR-8 | a job handler's `Handle` inspecting an error; a handler named `Execute`, `Process`, `Worker` or `Processor`; a job kind that isn't `<capability>.<action>` | nilerr: a handler dropping an error |
| `ctxkeys` | CTX-2 | `context.WithValue` outside `context.go`; a key that isn't an unexported `struct{}` type of the package | revive `context-keys-type`: keys of basic types |
| `goroutines` | CON-1, CON-2 | a `go` statement outside `internal/foundation/supervise` and `pkg/` | forbidigo: `time.Sleep` |
| `repos` | DATA-1, DATA-2 | code in `internal/repository` outside one package per aggregate; no `repo.go` with `Repo` and `New`, or no `row.go`; an exported type other than `Repo`; an SDK type in a `Repo` method | depguard: drivers in `domain` |
| `repoverbs` | DATA-3 | a `Repo` method starting with a banned verb, or repeating the package's name | review: transition verbs |
| `constquery` | DATA-5, DATA-6, DATA-8, SEC-15 | a repository statement that isn't a constant; `now()`, `current_timestamp` or `current_date` in a statement, and `$currentDate`; `ON CONFLICT DO` with no target | gosec G201, G202: string formatting anywhere |
| `comments` | STY-5 | punctuation beyond ASCII; references to plans, phases, tasks or documents; `Step N:` narration; a `TODO` without `(TICKET-123):` | revive `exported`, `package-comments`; gocritic `commentedOutCode`, `docStub` |
| `metricnames` | OBS-2 | an instrument name that isn't a constant `<svc>.<capability>.<noun>`, a unit suffix, no `metric.WithUnit`, a duration that isn't a `Float64Histogram` in seconds | review: attributes |

Test files aren't read while restructuring tests is paused (TEST-14), and neither are test packages: `<pkg>test` packages, packages named `…tests`, and anything under `internal/testutils`.

## Nothing comes from the project

No analyzer takes settings. The module comes from the package path, the layer from the folder under `internal/`, and each SDK's home from the table in `internal/where/sdk.go`, built from ABS-7 and LAY-8 (LAY-7 prints it). A rename in a project changes nothing here.

## Exceptions

Each analyzer is its own golangci-lint linter, so an exception names the one it silences, on the line it's about, with the reason, agreed with the engineer (COL-7):

```go
import "example.com/myservice/internal/adapters/scylla" //nolint:layers // keeps its name until the rename to scyllastore (LAY-8)
```

There are no lists of exceptions inside the analyzers. Wire values frozen on an `existing` scope are reported once at the enum's type, so one `//nolint:enums` covers the enum.

## Build and test

```sh
go test ./...   # analysistest fixtures in testdata/: one stub tree, with a module per analyzer
cd .. && go run github.com/golangci/golangci-lint/v2/cmd/golangci-lint@v2.15.0 custom # reads lint/.custom-gcl.yml; writes lint/bin/golangci-lint
```

Every analyzer has fixtures with at least one case it must report and one it must not, so an analyzer that stops reading code fails its own test. The fixtures live in `testdata/src` as a GOPATH tree: stubs of gofi, errx, enumx, pgx, gocql, go-redis, franz-go and the OTel metric API, and one fixture module per analyzer, such as `example.com/layers`. `testdata/modules/layout` is a real module, for the LAY-1 cases that need one.

The module path is the standard's own, with this folder's path. `.custom-gcl.yml` builds it from the local `path`, so it doesn't need to resolve.
