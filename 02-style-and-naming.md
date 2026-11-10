# 02 · Style, naming and formatting

This document covers how Go code looks line by line: formatting, names, function and struct shape, comments, the order of declarations in a file, modern Go idioms, and literals. It also explains what the shared lint config checks and what it can't.

The config itself is [lint/.golangci.yml](lint/.golangci.yml).

## The style in four lines

Let tools decide everything they can.
Name things for the reader who imports them.
Write each function as a short story: guard clauses, then the steps, in paragraphs.
Comment *why*, never *what*.

## Rules at a glance

| ID | Rule | Enforced by |
|---|---|---|
| STY-1 | Format with gofumpt and order imports with gci in three groups. Every project uses one shared lint config. | formatters, CI |
| STY-2 | Names follow the [naming table](#sty-2--naming). | revive, staticcheck, arch test |
| STY-3 | Guard clauses first. Split a function only where a part is its own idea. No length limit; complexity 20, nesting 4, 6 parameters, 3 results. A field tagged `lint:"required"` or `validate:"required"` must be set in every literal. | gocognit, revive, `requiredtag` |
| STY-4 | Structs: identity, then data, then timestamps. A named `mu` sits above the fields it guards. Tags in a fixed order, snake_case JSON, and lowercase kebab-case header names. | embeddedstructfieldcheck, tagliatelle |
| STY-5 | Doc-comment every exported identifier. Explain *why*. No step narration, plan references or bare TODOs. | revive, gocritic, CI grep |
| STY-6 | Declaration order in a file: consts, vars, type, `New`, exported methods, unexported methods, helpers. | funcorder |
| STY-7 | Use modern Go: `any`, `range n`, `new(v)`, `slices`/`maps`, `min`/`max`, `wg.Go`. | modernize, intrange |
| STY-8 | Name policy values and repeated literals. Use stdlib constants for HTTP methods and statuses, except the status in gofi's `c.Send`. | usestdlibvars, review |
| STY-9 | A function reads as a story: what it does, in steps, in paragraphs separated by blank lines, with names that say what each value is. | wsl, review |

---

## STY-1 · Formatting and the shared lint config

**Rule.**
- **Formatting:**
  - Format with **gofumpt**, using `extra: {group-params, clothe-returns}`.
  - Order imports with **gci** in three groups: `standard`, `default` (third party), `localmodule`.
  - `localmodule` reads the module path from `go.mod`, so one config file works unchanged in every project.
- **Commands:**
  - `task fmt` rewrites formatting and applies the linters' automatic fixes. It is the only task that writes files.
  - `task lint` only checks; formatting problems are reported as findings.
  - CI runs `task lint`, and so does the pre-push hook (ADO-4).
- **Import aliases** are allowed only when two imports collide, or when the package name differs from the last element of its import path.

**Why.**
- Without a CI workflow, lint gates nothing.
- One lint config failed `golangci-lint config verify`. It pinned `go: "1.24"` while its module was on 1.27.3, and 112 of that codebase's files used four import groups.
- A codebase with no lint config had:
  - 10 files that weren't gofmt-clean;
  - 475 of 653 files that mixed local and third-party imports in one group;
  - a `.githooks/pre-push` that was commented out.

**Don't.** A file that was never formatted:

```go
package cryptotest
import("context";"crypto/aes";"crypto/cipher";"crypto/rand";"crypto/sha256";"github.com/michaelolof/errx")
type Keys struct{}
func New()*Keys{return &Keys{}}
```

**Don't.** Four import groups, a redundant alias, and a stuttering name:

```go
import (
	"errors"

	repository "example.com/myservice/internal/repository"

	goredis "github.com/redis/go-redis/v9"

	apikeyscache "example.com/myservice/internal/cache/apikeys"
	// ...
)
	APIKeys() apikeyscache.APIKeysCache
```

**Canonical:**

```go
import (
	"context"
	"time"

	"github.com/michaelolof/errx"
	goredis "github.com/redis/go-redis/v9" // package "redis" collides with a local package of that name

	"example.com/myservice/internal/domain"
)
```

## STY-2 · Naming

The naming decisions from every area are collected in this one table. When a name isn't covered here, follow Go Code Review Comments.

| What | Convention | Example | Never | Rule |
|---|---|---|---|---|
| Package | Same as the directory: lowercase, one word | `transfers`, `mongostore` | `product_members`, an `apptime` package in directory `time` | LAY-8 |
| Adapter package | The external system, one package per system. A database or object store adds `store` | `redis`, `kafka`, `awskms`, `mongostore`, `s3store` | `rediscache`, `redislock` (one system split by role); `mongo` (collides with the driver) | LAY-8, ABS-7 |
| Helper packages | Name for what it provides. Shared by several packages: `foundation/<name>`. Incubating as a library: `pkg/<name>`. Used by one package: a file in that package. | `foundation/enumtag`, `foundation/ptr` | `util(s)`, `helper(s)`, `common`, `shared`, `misc`, `base`, `types`, `models`, `interfaces`, `*utils` | LAY-9 |
| Test-helper package | `<pkg>test` | `domaintest`, `kafkatest` | `testutil`, `testdata` (as Go code) | LAY-8, TEST-10 |
| File | snake_case, named after a type or use case | `cancel.go`, `transfer_transitions.go` | `helpers.go`, `utils.go`, `common.go`, `types.go` | LAY-10 |
| Exported name | No stutter with the package name | `apikeys.Cache`, `delivery.Pipeline` | `apikeys.APIKeysCache`, `kms.KMSProvider` | STY-2 |
| Initialisms | All in one case | `ID`, `URL`, `HTTP`, `JSON`, `ULID`, `KYC`, `DEK` | `HttpError`, `InvalidJsonErr`, `ById` | STY-2 |
| Typed ID | `<Entity>ID`, over `[16]byte` | `TransferID` | `string`, `gocql.UUID` in domain | DOM-1 |
| Interface | `-er` for one method; `Store` for persistence; a domain noun for a vendor role | `Publisher`, `transfer.Store`, `Custodian` | `I*`, `*Iface`, `*Interface`, `*Impl`, `*Adapter`, `*Manager`, `*Helper` | ABS-3 |
| Persistence implementation | `<aggregate>.Repo` | `transfers.Repo` | `TransferRepository` | ABS-3 |
| Use-case input and output | `<Op>Args`, `<Op>Result` | `CancelArgs` | `CancelWalletArg`, `Params` | ABS-6 |
| Constructor | `New`, or `New<Type>` when the package has several types | `transfer.New(deps)` | `Build*`, `Make*` | ABS-5 |
| Repository verbs | `Get`, `Find`, `List`, `Count`, `Create`, `CreateOrGet`, `Update`, `Upsert`, `Delete`, plus transition verbs | `Repo.Get` | `Insert`, `Fetch`, `Lookup`, `Load`, `Read`, `Save`, `Put`, `Search` | DATA-3 |
| Getter | Noun | `Minor()` | `GetMinor()` | STY-2 |
| Enum variable | `<Type><Variant>` | `TransferStatusPending` | `PendingTransferStatus` | DOM-6 |
| Enum wire value | UPPER_SNAKE. On a `greenfield` scope (COL-15) nothing is frozen, so rename freely. On an `existing` scope, a rename is a breaking change. | `"PENDING"` | | DOM-6 |
| Error kind | `ErrKind<Name>` = snake_case string, prefixed `<pkg>_` outside `errkinds` | `ErrKindNotFound = "not_found"` | `NotFoundErr`, kebab-case | ERR-5 |
| Error variable | `err`, or `<verb>Err` for a second error in scope | `err`, `rbErr` | `e`, `pe`, `ce` | STY-2 |
| Receiver | 1–2 letters from the type, the same on every method | `r *Repo`, `s *Service` | `self`, `this`, mixed names | STY-2 |
| Boolean | Adjective or participle for state; `Is`/`Has`/`Can` for predicate functions | `Active`, `IsExpired()` | | STY-2 |
| JSON tag | snake_case | `json:"transfer_id"` | camelCase | STY-4 |
| Header tag, in a gofi schema's `Header` | lowercase kebab-case, as the header is sent | `json:"idempotency-key"` | `json:"idempotency_key"`, `json:"Idempotency-Key"` | STY-4 |
| Log message | Dotted lowercase event, past tense | `job.attempt.failed` | `"Failed to process job %s"` | LOG-3 |
| Log key | snake_case string literal | `"tenant_id"` | `"err"`, `"tenantId"` | LOG-2 |
| Metric | `<svc>.<capability>.<noun>`, with a unit set and no suffix | `myservice.wallet.provision.duration` | `edge_request_duration_ms` | OBS-2 |
| Job kind | `<capability>.<action>` | `transfer.execute` | | JOB-1 |
| Migration file | `YYYYMMDDHHMMSS_<snake>.up.<ext>` | `20261021000700_auth_attempts.up.sql` | `07_x.cql` | DATA-11 |
| Test function | `Test<Func>` or `Test<Type>_<Method>[_<Scenario>]`; subtests are lowercase phrases | `TestRepo_Get_NotFound` | | TEST-7 |
| Fake | `fakeX` local, `xtest.FakeX` shared | `fakeTransferStore` | `stub*`, `spy*`, `mock*`, `recording*` | TEST-5 |
| Local variable | Length grows with scope | `i`, `tr` in a 5-line loop | 20-character locals | STY-2 |

**Why.** Across three codebases, most of these already held:
- 99% of receivers are one letter, and none is `self` or `this`.
- Initialism violations are a handful.

The drift is in package names, stutter, and error variables. revive flagged 16 stuttering names. One codebase had 155 package-name/directory mismatches. Another named an error `e` 620 times, against 1,678 uses of `err`.

**Don't.** Three error-naming styles and an `else` after `return`, all within 12 lines:

```go
scyllaSession, err = scylla.NewSessionFromConfig(scylla.ConfigFromEnv(env))
if err != nil {
	return nil, errx.Wrap(1793625654461, err)
} else {
	// ...
	if e := projectcleanup.CheckRestoreBarrier(context.Background(), env, repo); e != nil {
		return nil, errx.Wrap(1793625654927, e)
	}
	if kmsClient, kmsErr := kms.BuildKMSClient(env); kmsErr != nil {
```

## STY-3 · Function shape

**Rule.**
- **Guard clauses first.** Handle errors and edge cases first and return, so the happy path stays at the left margin. No `else` after `return`, `continue` or `break`.
- **Split for ideas, not for size.** Extract a function when a part of the body is its own idea: a business rule, a concept with a name, or a step that is reused. Don't split only to make a function shorter. A helper that is called once, whose name restates its body, adds a jump without adding meaning (Clean Code ch 3: "extract until you can't extract a function whose name isn't a restatement of its body").
- **Length is not checked.** A long function that reads top to bottom as one clear story (STY-9) is fine. What *is* checked is how hard a function is to follow:

| Check | Max | Linter |
|---|---|---|
| Cognitive complexity | 20 | gocognit |
| Nesting depth | 4 | revive `max-control-nesting` |
| Parameters, including `ctx` | 6 | revive `argument-limit` |
| Results | 3 | revive `function-result-limit` |

- **Parameters.**
  - `ctx` is always the first parameter. It never goes into a struct (CTX-1).
  - Many parameters usually mean the function is rebuilding a concept from its pieces. Pass the concept instead: the entity being worked on, a domain value, a tenant reference. Usually it already exists.
  - Never create a struct only to reduce the parameter count. `<Op>Args` exists because a use case's inputs are one concept, the request (ABS-6), not to satisfy a limit.
- **Required struct fields.** In Go every field is optional, and a field left out of a literal silently gets its zero value. That default stays. A field becomes required only when a struct tag says so. Each project chooses which tags count, in its lint config. The config in this standard uses:
  - `lint:"required"` on fields that no validator reads: use-case `Args`, repository `Params`, events, job args, and domain entities built in core;
  - `validate:"required"` on gofi schema fields, which already carry it. The same tag now also means "every literal of this type sets it".

  The `requiredtag` linter rejects any literal that leaves out a tagged field. No comments are involved. Which tags count as "required" is configured, in `key:option` form, so another codebase can use its own (`bson:"required"`, for example). For small value types, a `New…` constructor with positional parameters is the compiler-checked alternative (DOM-5).
- **Return shapes:**

| Shape | Use for |
|---|---|
| `(T, error)` | Domain values and DTOs, returned by value |
| `(*T, error)` | Constructed services and adapters |
| `(T, bool)` or `(T, bool, error)` | Lookups where a missing value is normal |

  Never `return nil, nil`.
- **Named results** only to distinguish results of the same type, or to capture an error in a `defer`. Never naked returns.
- **No nil checks on the receiver.** A nil `*T` is a bug in whatever constructed it. Its dependencies go into `New`, not into `WithX` setters (ABS-5).
- **A `// Step N` comment** means the function does several things. Extract the steps that are separate ideas, named after what the comment says (STY-5).

**Why.** Across three codebases:
- 29, 90 and 35 functions ran over 80 lines, and 8, 23 and 8 of them over 150 lines;
- 24, 68 and 96 functions had 5 or more parameters;
- 0, 16 and 70 methods nil-checked their own receiver.

The line counts are a place to look, not a list of functions to split.

**How the required-field check behaves.** `requiredtag` is the standard's own golangci-lint module plugin, in [lint/requiredtag](lint/requiredtag). Existing linters were checked first:

| Linter | Marks a field with | Configurable | golangci-lint v2 plugin | Why not |
|---|---|---|---|---|
| exhaustruct v5 | `//exhaustruct:` comments; all fields required in an enforced type | no | built in | comments; required by default |
| abhinav/requiredfield | a `// required` comment | no | yes | comments |
| colinking/go-reqfields | `require:"true"` | no | no (old `.so` plugin, last change 2021) | reads tags from source, so it misses types from other packages |
| juanrgon/structly v0.1.2 | `required:"true"` | no (settings ignored) | yes | fixed tag; a positional literal is reported as missing every required field |

So the standard has its own. Its whole configuration:

```yaml
custom:
  requiredtag:
    type: module
    settings:
      tags: [validate:required, lint:required] # key:option
      allow-empty-returns: true                # return T{}, err and return T{}, false
```

What it does:

```go
type CancelArgs struct {
	TenantID   domain.TenantID   `lint:"required"`
	TransferID domain.TransferID `lint:"required"`
	ActorID    domain.ActorID    `lint:"required"`
	Reason     string
}

svc.Cancel(ctx, CancelArgs{TenantID: t, TransferID: id})
// CancelArgs is missing required field ActorID (requiredtag)

return CancelResult{}, err   // allowed: the error says there is no value
return Transfer{}, false     // allowed: the lookup found nothing
```

- A marker matches when the tag's comma-separated options include it: `validate:"required,email"` matches, `validate:"required_if=Kind card"` doesn't.
- It reads tags from type information, so types from other packages, generic types and aliases are all checked.
- Positional literals are skipped, because the compiler already requires every field in them.
- `var t T`, `new(T)` and fields assigned after the literal involve no literal, so it can't see them. Build the value in one literal (STY-9).
- Test files are excluded, because tests deliberately build partial fixtures and use `T{}` in `reflect.TypeOf(T{})`.

**What it finds.** Across three codebases, run with only `validate:required`, a tag they already used 1,865 times, it reported 2, 5 and 1 findings in non-test code (tested):

- A handler sends `TenantSignupLinkInspectResponse{Usable: false}` when a link is unknown or can't be loaded. Five of its fields are `validate:"required"`, and gofi validates response bodies. A probe with the same success envelope and tags returns `500 "value is required (empty) at response Body(data.link_id)"` (tested on gofi v0.8.9). So an unknown signup link gets a 500 instead of `usable: false`.
- Five domain values were built partially and completed later, or never. For example, `domain.WebhookEnvelope{ProviderName: provider}` is built only to carry one field into `QuarantineMalformed`. Pass the concept the function needs instead.
- `TestPingResponseData` is built, then filled in field by field.

**Don't.** Eight parameters, three of them strings and two of them raw JSON. The function rebuilds an inflow from its pieces:

```go
func (s *Service) buildDetectEvent(inflowID, orgID, tenantID string, env domain.WebhookEnvelope, p domain.InflowCreditPayload, providerReferences, metadata json.RawMessage, now time.Time) (domain.EventEnvelope, error) {
```

**Do.** The caller already has the concept these pieces belong to: the inflow it is about to store. Build the event from that. No `Args` struct is needed:

```go
func (s *Service) detectedEvent(inflow domain.Inflow) (domain.EventEnvelope, error) {
```

**Don't.** A defensive receiver nil check on a post-construction setter. One file had twelve of them.

```go
func (c *DeliveryOutcomeCoordinator) WithBacklogTracker(tracker ports.WarmCheckBacklogTracker) *DeliveryOutcomeCoordinator {
	if c == nil {
		return nil
	}
	c.backlogTracker = tracker
```

## STY-4 · Struct design

**Rule.**
- **Field order:**
  1. embedded types, then a blank line;
  2. identity: `ID` and tenant keys;
  3. data, grouped with blank lines;
  4. lifecycle timestamps: `CreatedAt`, `UpdatedAt`, `…At`.
- **Mutexes.** A mutex is a named field, `mu sync.Mutex`, placed directly above the fields it guards. Never embed it, because that puts `Lock` on the type's API.
- **Exported vs unexported fields:**
  - DTOs, `<Op>Args`, `<Op>Result` and config structs have exported fields and should be usable at their zero value.
  - Services and adapters have unexported fields and are built by `New`.
- **Tags:**
  - Order: `json`, `bson`, `validate`, `lint`, `default`, `spec`, `pattern`, `description`, `example`.
  - A domain type may carry tags for HTTP schemas, such as `json` and `validate`, because it's the type used in schemas (DOM-4). It carries no database tags, whatever the database. `bson` tags go on a repository's private document type (DATA-2).
  - Don't hand-align tags.
  - JSON names are snake_case.
  - Header names are the exception: lowercase kebab-case, as the header is sent, because gofi binds a schema's `Header` fields by those names (gofi TAG-2):

    ```go
    Header struct {
    	ContentType    string `json:"content-type" validate:"required"`
    	IdempotencyKey string `json:"idempotency-key" validate:"required"`
    }
    ```

    tagliatelle can't tell a header struct from any other, so the shared lint config lets through any lowercase kebab-case name. A body field written in kebab-case therefore also passes, and is left to review. Across three codebases, all 54 kebab-case `json` tags were headers.
- **Optional struct fields in HTTP schemas** are pointers with `omitempty`, or values with `omitzero`, which works as in `encoding/json` since gofi v0.10.3. `omitempty` never omits a struct or array, so use `omitzero` or a pointer for an optional `[16]byte` ID.
- **No driver types** in any struct outside `repository/` and `adapters/` (DOM-1, DOM-4). An ID is a domain `[16]byte` type, not `gocql.UUID`.

**Why.** Across three codebases:
- JSON tags were already snake_case in 99.9% of cases.
- Tag order was already nearly uniform.
- Embedded-field problems numbered 6, 38 and 38.

**Don't.** A driver type in a transport schema, plus a commented-out field:

```go
type AuthUserResponseData struct {
	AccessToken string     `json:"access_token" validate:"required"`
	UserID      gocql.UUID `json:"user_id" validate:"required" spec:"cql-uuid"`
	// OrgSlug      string `json:"org_slug" validate:"required"`
	Role models.Role `json:"role" validate:"required,oneof@Oneof"`
}
```

**Canonical:**

```go
type Transfer struct {
	ID       TransferID
	TenantID TenantID

	Amount Amount
	Status TransferStatus

	CreatedAt time.Time
	SettledAt *time.Time
}

type limiter struct {
	mu     sync.Mutex // guards tokens and last
	tokens int
	last   time.Time
}
```

## STY-5 · Comments

**Rule.**
- **Doc comments.**
  - Every exported identifier has a doc comment that starts with its name.
  - Every package has a package comment (LAY-11).
- **What a comment says:** *why*, invariants, and units. Never what the next line does.
- **No "Step N:" narration.** If a function needs it, extract each step into a named function.
- **No references** to plans, phases, document sections or task numbers (`§6.2`, `Phase 3`, `improvements.md`). A comment must stand on its own after the plan is gone.
- **TODOs** are `// TODO(TICKET-123): <action>`. No bare TODOs.
- **No commented-out code.**
- **Punctuation:** plain ASCII.

**Why.** Across three codebases, doc comment coverage on exported identifiers was 96%, 86% and 48%.

Comments that simply restate the code are rare. The agent tics that do show up are:
- step narration inside long functions;
- references to planning documents: 10, 19 and 17 respectively;
- commented-out fields.

**Do:**

```go
// Package pagination provides deterministic cursor helpers for paginated
// API responses.
//
// Cursors are opaque, round-trippable, and reject tampering.
//
// This package may use standard library and core primitives. It must not
// import HTTP, Gofi, MongoDB, Kafka, repositories, workers, or concrete
// adapters.
package pagination
```

**Don't.** Step narration inside a method over 150 lines long:

```go
	// Step 1: Update transfer status.
	// ...
	// Step 2: Close the execution op (best-effort) and create the RECONCILE_REFERENCE op.
	// ...
	// Step 3: Insert outbox event.
```

**Don't.** Plan references:

```go
// matching §6.2's "must use separate... concurrency controls" requirement.
// adapter. See myservice-docs/improvements.md for the full design.
```

**Canonical:**

```go
// Park moves a transfer to AWAITING_RECONCILIATION and opens a
// RECONCILE_REFERENCE operation. It is idempotent: parking an already
// parked transfer returns the current state.
func (s *Service) Park(ctx context.Context, args ParkArgs) (ParkResult, error) {
	// ...
	// The provider may still settle the old operation; close it best-effort so
	// the reconciler, not this call, decides the outcome.
	s.closeExecutionOp(ctx, t.ID)
```

**Enforced by:**
- revive `exported` and `package-comments`;
- gocritic `commentedOutCode`, `todoCommentWithoutDetail` and `docStub`;
- a CI grep for the patterns golangci-lint can't express: `//.*(§|Phase [0-9]|Task [0-9]+|\.md\b)`, `// Step [0-9]+:`, and a `TODO` not followed by `([A-Z]+-[0-9]+):`.

## STY-6 · Declaration order in a file

**Rule.** Each file holds one primary type and is named after it. Declarations appear in this order:

1. package doc
2. imports
3. `const`
4. `var`, including `var _ Iface = (*T)(nil)`
5. the primary type, then its `<Op>Args`/`<Op>Result` types
6. `New`
7. exported methods, in the order a reader would call them
8. unexported methods
9. unexported free functions

**Do.** A repository file that follows this order: package doc → `type Repo` → `CreateArgs` → `New` → methods → document mapping helpers.

**Don't.**
- Three types in one 377-line file: `Producer`, `Consumer` and `Relay`.
- A free function and unexported methods placed before the exported `Stop`.

**Enforced by:**
- funcorder, for the constructor position and exported methods before unexported ones;
- review, for "helpers last" and "one type per file".

## STY-7 · Modern Go

**Rule.**
- **Go version.** All projects are on the same Go minor version, at least 1.26. The lint config has no `run.go` pin.
- **Use these forms:**
  - `any`, never `interface{}`;
  - `for i := range n`;
  - `new(v)`, instead of `ptr`/`strPtr` helpers;
  - `slices`, `maps` and `cmp`, instead of hand-written loops and `sort.Slice`;
  - `min` and `max`;
  - `wg.Go(f)`;
  - `strings.Cut`, `CutPrefix` and `SplitSeq`;
  - `reflect.TypeFor`.
- **Dereference helpers.** Never copy one into a file. If one is needed, define a single generic in `foundation`.
- **Iterators.** Use `iter.Seq` only for streaming or lazy APIs. Return slices otherwise.

**Why.**
- Across three codebases, modernize finds 70, 107 and 46 fixes.
- One codebase has 17 pointer-making helpers.
- It also has the same dereference helper five times under three names. One copy is misnamed:

```go
// also stringPtrVal ×2, stringPtrValue ×2 elsewhere
func stringPtr(value *string) string {
	if value == nil {
		return ""
	}

	return *value
}
```

**Canonical:**

```go
retry := new(cfg.MaxRetries) // was intPtr(cfg.MaxRetries)
for i := range workers {
	wg.Go(func() { run(ctx, i) })
}
if slices.Contains(allowed, kind) { /* ... */ }
maps.Copy(dst, src)
```

All of these fix automatically with `task fmt`.

## STY-8 · Literals and constants

**Rule.** Name a literal when it is any of these:
- **A policy value** (timeout, TTL, lease, limit, batch size). Make it a typed `const` at the top of the file that owns it, or a config field.
- **Used more than once,** including strings and keys. Make it a `const`, or a function such as a key builder.
- **A wire or database value.** Make it an enum constant (DOM-6).

Leave these as literals:
- `0`, `1` and `-1`;
- small arithmetic;
- errx stamps;
- test data.

Also:
- Use the `net/http` constants instead of `"GET"` or `200`. The one exception is the status passed to gofi's `c.Send`, which is an integer literal matching the schema field (`200` with `Ok`, `201` with `Created`), as gofi RES-3 requires.
- Use `iota` only for in-process values that are never persisted or sent over the wire.

**Why.** One codebase used `2*time.Minute` as the recovery lease in four separate files. One of its functions built the same Redis key string twice. The magic-number linters are too noisy to gate on: across three codebases, `mnd` reports 1,359, 2,675 and 3,365 findings, and `goconst` 213, 831 and 915. So this rule is enforced by review.

```go
const recoveryLease = 2 * time.Minute

func recoveryPublishKey(workID string, gen int64) string {
	return "{webhooks}:recovery-publish:" + workID + ":" + strconv.FormatInt(gen, 10)
}
```

---

## STY-9 · A function reads as a story

A reader should know what a function does from its first lines, then follow its steps top to bottom without jumping elsewhere. Names, the size of each step, blank lines and the order of statements all serve that. This matters more for readability than any length limit.

**Rule.**
1. **The title.** The name says the outcome as a verb phrase. The doc comment states the contract (STY-5).
2. **The early exits.** Guard clauses come first (STY-3).
3. **The steps, one paragraph each.** A paragraph is a statement that produces something, plus the check of its result:

   ```go
   event, err := s.detectedEvent(inflow)
   if err != nil {
   	return domain.Inflow{}, errx.Wrap(1793625655882, err)
   }
   ```

   - Separate paragraphs with one blank line. A closing `}` ends a paragraph.
   - Never put a blank line inside a paragraph, and never two in a row.
   - Put a blank line before the final `return`, unless the whole function is one paragraph.
   - The exact rules, with examples, are in [Spacing, rule by rule](#spacing-rule-by-rule) below.
4. **One level of detail per function** (Clean Code ch 3, the "step-down rule"). A use case reads as a list of business steps. The detail of each step (SQL, JSON shape, crypto, queue mechanics) lives in the functions it calls, placed below it in the file (STY-6). Don't mix "store the inflow" with building JSON field by field in the same body.
5. **Name each value for its role in the story** (`stored`, `cancelled`, `event`), not for its type or origin (`row`, `res`, `resp2`, `tmp`, `p`). When a condition's meaning isn't obvious, give it a name: `isTerminal := …` (Clean Code G19, "explanatory variables").
6. **Declare a value where it is first needed.** A block of copies at the top (`providerName := env.ProviderName`) means the story starts in the wrong place.

**Enforced by:**
- `wsl_v5` for the paragraphs (rule 3), with its default checks plus `after-block`. It fixes its own findings: `task fmt`.
- Review for the rest. This is the first thing a reviewer reads for, before naming details.

**What it finds.** Across three codebases, `wsl_v5` findings in non-test code (golangci-lint 2.15):
- findings with its default checks: 2,330, 3,770 and 3,864;
- findings with `after-block` added: 4,062, 6,013 and 6,497;
- files with at least one: 223 of 329, 384 of 574 and 343 of 460.

The most common findings are:
- a `return` with no blank line after a long block;
- an `if` cuddled with a statement it doesn't check;
- an assignment cuddled with unrelated statements.

A file minified onto a few lines crashes the linter, so it was excluded from the count.

### Spacing, rule by rule

The linter places blank lines by a small set of rules. Each rule below shows the message `wsl_v5` prints, the code it flags, and what `task fmt` writes instead. Every example was tested with golangci-lint 2.15 and this config, and the "Do" side is the linter's own output. These samples only show spacing, so they leave out things other rules require, such as typed enum values, wrapped errors and the repository verbs. Don't copy them for anything but the spacing.

| # | Rule | Message |
|---|---|---|
| 1 | An `if` sits directly under the one assignment it checks, and nothing else. A second `if` starts a new paragraph. | `invalid statement above if`, `too many statements above if` |
| 2 | No blank line between a call and its error check. | `unnecessary whitespace (err)` |
| 3 | A blank line after every closing `}`. The one exception is a `defer` that releases what the block just checked. | `missing whitespace below this line (after-block)` |
| 4 | A blank line before `return`, `continue` and `break` when the block above them has more than two lines. | `too many lines above return`, `too many lines above branch` |
| 5 | An assignment or a call sits only next to statements of its own step. | `invalid statement above assign`, `invalid statement above expr` |
| 6 | A loop sits directly under an assignment only when the loop uses it. | `no shared variables above range` |
| 7 | Declarations are never cuddled. Group them in one `var ( … )`. | `never cuddle decl` |
| 8 | No blank line at the start or end of a block. | `unnecessary whitespace (leading-whitespace)`, `(trailing-whitespace)` |

**Rules 1, 3 and 4.** Two checks in a row are two paragraphs:

```go
// Don't
t, err := s.repo.Get(ctx, id)
if err != nil {
	return wrap(err)
}
if t.Status != "PENDING" { // invalid statement above if
	return errNotPending
}
return nil                 // too many lines above return

// Do
t, err := s.repo.Get(ctx, id)
if err != nil {
	return wrap(err)
}

if t.Status != "PENDING" {
	return errNotPending
}

return nil
```

**Rule 2.** A call and its error check are one paragraph:

```go
// Don't
_, err := s.repo.Get(ctx, id)
                            // unnecessary whitespace (err)
if err != nil {

// Do
_, err := s.repo.Get(ctx, id)
if err != nil {
```

**Rules 1 and 5.** Only one statement may sit above an `if`, and it must be the one the `if` checks. Preparing the values and storing them are separate steps:

```go
// Don't
event := eventFor(t)
t.Status = "CANCELLED"
stored, err := s.repo.Save(ctx, t, event)
if err != nil {             // too many statements above if
	return Transfer{}, wrap(err)
}
publish(ctx, event)         // invalid statement above expr
return stored, nil

// Do
event := eventFor(t)
t.Status = "CANCELLED"

stored, err := s.repo.Save(ctx, t, event)
if err != nil {
	return Transfer{}, wrap(err)
}

publish(ctx, event)

return stored, nil
```

**Rules 3, 5 and 6.** A lock and its deferred unlock are one paragraph. A loop keeps the assignment it uses:

```go
// Don't
s.mu.Lock()
defer s.mu.Unlock()
total := 0                  // invalid statement above assign
for _, t := range ts {
	total += int(t.Amount)
}
publish(ctx, Event{})       // invalid statement above expr
count := len(ts)            // invalid statement above assign
return total + count

// Do
s.mu.Lock()
defer s.mu.Unlock()

total := 0
for _, t := range ts {
	total += int(t.Amount)
}

publish(ctx, Event{})

count := len(ts)

return total + count
```

**Rules 7 and 8.** Declarations go in one group, and blocks don't open or close with a blank line:

```go
// Don't
func average(ts []Transfer) int {
                            // leading-whitespace
	var total int
	var count int           // never cuddle decl
	for _, t := range ts {  // no shared variables above range

		total += int(t.Amount)
		count++

	}
	...

// Do: --fix grouped the declarations itself
func average(ts []Transfer) int {
	var (
		total int
		count int
	)

	for _, t := range ts {
		total += int(t.Amount)
		count++
	}
	...
```

**Rules 3 and 4 inside a loop:**

```go
// Do
for _, t := range ts {
	if t.Amount == 0 {
		out = append(out, t.ID)
		t.Status = "SKIPPED"

		continue
	}

	kind := t.Status
	switch kind {
	case "PENDING":
		out = append(out, t.ID)
	}
}

return out
```

**What `defer` may sit next to.** Acquire, check, release is one paragraph. `after-block` allows the `defer` straight after the check:

```go
release, err := acquire()
if err != nil {
	return wrap(err)
}
defer release()

event := Event{Kind: "x"}
go publish(ctx, event)
```

**A real function, before and after `task fmt`** (the first 40 lines). Only blank lines changed: 14 added in this function. As with the samples above, look only at the spacing.

```go
// Before
func (s *Service) RegisterBeneficiary(ctx context.Context, arg domain.RegisterBeneficiaryArg) (domain.Beneficiary, error) {
	if err := arg.Network.Validate(); err != nil {
		return domain.Beneficiary{}, err
	}
	if err := arg.AssetCode.Validate(); err != nil {
		return domain.Beneficiary{}, err
	}
	if arg.Recipient != nil {
		if err := arg.Recipient.Kind.Validate(); err != nil {
			return domain.Beneficiary{}, err
		}
		if err := arg.Recipient.Relationship.Validate(); err != nil {
			return domain.Beneficiary{}, err
		}
	}

	// 1. Resolve provider from the API key context (not from a wallet).
	providerName := s.resolveProviderName(ctx)
	prov, err := s.registry.Get(providerName)
	if err != nil {
		return domain.Beneficiary{}, errx.Wrap(1793625656526, err)
	}

	caps := prov.Capabilities()
	kind := classifyKind(caps)
	if kind == "" {
		return domain.Beneficiary{}, errx.NewKindf(1793625657671, errkinds.ErrKindInvalidInput,
			"provider %q does not require beneficiary registration", providerName)
	}

	// 2. Vendor routes: require an approved KYC subject → entity_id.
	var entityID string
	if kind == domain.BeneficiaryKindVendorRegistered {
		subject, kycErr := s.lookupApprovedSubject(ctx, arg, providerName)
		if kycErr != nil {
			return domain.Beneficiary{}, kycErr
		}
		entityID = subject.KYCReference
	}
```

```go
// After
func (s *Service) RegisterBeneficiary(ctx context.Context, arg domain.RegisterBeneficiaryArg) (domain.Beneficiary, error) {
	if err := arg.Network.Validate(); err != nil {
		return domain.Beneficiary{}, err
	}

	if err := arg.AssetCode.Validate(); err != nil {
		return domain.Beneficiary{}, err
	}

	if arg.Recipient != nil {
		if err := arg.Recipient.Kind.Validate(); err != nil {
			return domain.Beneficiary{}, err
		}

		if err := arg.Recipient.Relationship.Validate(); err != nil {
			return domain.Beneficiary{}, err
		}
	}

	// 1. Resolve provider from the API key context (not from a wallet).
	providerName := s.resolveProviderName(ctx)

	prov, err := s.registry.Get(providerName)
	if err != nil {
		return domain.Beneficiary{}, errx.Wrap(1793625656526, err)
	}

	caps := prov.Capabilities()

	kind := classifyKind(caps)
	if kind == "" {
		return domain.Beneficiary{}, errx.NewKindf(1793625657671, errkinds.ErrKindInvalidInput,
			"provider %q does not require beneficiary registration", providerName)
	}

	// 2. Vendor routes: require an approved KYC subject → entity_id.
	var entityID string

	if kind == domain.BeneficiaryKindVendorRegistered {
		subject, kycErr := s.lookupApprovedSubject(ctx, arg, providerName)
		if kycErr != nil {
			return domain.Beneficiary{}, kycErr
		}

		entityID = subject.KYCReference
	}
```

**What the linter can't do.** It separates blocks, but it can't tell which statements form one step of the story. That is rules 4 to 6 of STY-9, checked in review. In this function, the `// 1.` and `// 2.` comments mark steps that deserve names of their own (STY-3): `s.provider(ctx)` and `s.vendorEntityID(ctx, arg, kind)`. The validation at the top would read as one step as `arg.Validate()`.

**Don't.** A 360-line function with no paragraph breaks. Its main path sits inside `if p.lifecycle != nil`. In its first 45 lines it mixes four levels of detail: permits, ID parsing, lifecycle state rules and an anonymous-interface assertion.

```go
func (p *DeliveryPipeline) ProcessMessage(ctx context.Context, msg ports.StreamMessage) (processErr error) {
	if err := p.validateDependencies(); err != nil {
		return errx.Wrap(1793625658530, err)
	}
	parentCtx := ctx
	event := msg.Event
	if p.lifecycle != nil {
		bounded, release, e := projectguard.Enter(ctx, p.lifecycle.Repo, event.ProjectID, models.BusinessPermit)
		if e != nil {
			return errx.Wrap(1793625659623, e)
		}
		defer release()
		ctx = bounded
		pID, eID, e := parseEventIdentity(event)
		if e != nil {
			return errx.Wrap(1793625660008, e)
		}
		root, found, e := p.lifecycle.Repo.ReadLifecycle(ctx, pID, eID, models.EventRecord, "root")
		// ... 30 more lines in the same block
```

The same opening as a story. The rule "which lifecycle states may still be delivered" is its own idea, so it gets its own name:

```go
func (p *DeliveryPipeline) ProcessMessage(ctx context.Context, msg ports.StreamMessage) error {
	ctx, release, err := projectguard.Enter(ctx, p.lifecycle.Repo, msg.Event.ProjectID, models.BusinessPermit)
	if err != nil {
		return errx.Wrap(1793625659623, err)
	}
	defer release()

	job, deliverable, err := p.deliverableJob(ctx, msg.Event)
	if err != nil {
		return errx.Wrap(1793625660008, err)
	}

	if !deliverable {
		return p.ack(ctx, msg)
	}

	payload, err := p.openPayload(ctx, job)
	if err != nil {
		return errx.Wrap(1793625660412, err)
	}

	return p.deliver(ctx, msg, job, payload)
}
```

**Don't.** A function of 58 lines:
- It opens with five copies (`providerName := env.ProviderName`, `creditID := p.ProviderCreditID`, …).
- It declares `stored` long before it is used.
- It builds the event from eight loose pieces (STY-3).
- It uses a sentinel error to roll back a transaction when the inflow already exists.

**Do.** The same use case as a story: build the inflow, build its event, store both once.

```go
// detect records a provider credit as a DETECTED inflow and queues INFLOW_DETECTED
// with it. A credit seen before returns the stored inflow, and no second event.
func (s *Service) detect(ctx context.Context, wallet domain.Wallet, env domain.WebhookEnvelope, credit domain.InflowCreditPayload) (domain.Inflow, error) {
	inflow, err := domain.NewDetectedInflow(s.newID(), wallet, env.ProviderName, credit, s.clock().UTC())
	if err != nil {
		return domain.Inflow{}, errx.Wrap(1793625661562, err)
	}

	event, err := s.detectedEvent(inflow)
	if err != nil {
		return domain.Inflow{}, errx.Wrap(1793625655882, err)
	}

	stored, err := s.repo.Inflows().CreateDetected(ctx, inflow, event)
	if err != nil {
		return domain.Inflow{}, errx.Wrap(1793625662712, err)
	}

	return stored, nil
}
```

`CreateDetected` is one use-case-shaped statement: insert the inflow if it is new, and insert the event only if the inflow was inserted (DATA-4).

## What the lint config covers, and what it can't

[lint/.golangci.yml](lint/.golangci.yml) is one file for every project. Nobody edits it in a project: `task lint:config` writes `.golangci.yml` from it, with the project's module path (ADO-1).

**Checking it.** The file was verified with `golangci-lint config verify` on v2.15.0. A run across three codebases reported 2,049, 4,090 and 5,121 findings.

Between 36% and 57% of these were in test files, mostly `paralleltest`, which has since been removed from the config (TEST-8), so a run now reports fewer. Many are fixed automatically by `golangci-lint fmt` and `run --fix`.

**Before turning it on.**
- Run `task fmt` once. gci can crash on a file that was never formatted.
- Fix any `integration`-tagged test that no longer compiles. When no task builds tagged tests, a changed signature can break one with nobody noticing. The config's `build-tags: [integration]` will fail type-checking until it is fixed.

**Checks golangci-lint can't express** go into the shared `internal/architecture` test (TEST-13) or a small custom analyzer:

| Check | Rule | Where |
|---|---|---|
| Layer import table | LAY-6, LAY-7 | arch test |
| Root allowlist; no Go outside `cmd`/`internal`/`pkg`/`db/migrations`/`test`; `pkg/` imports no `internal/` | LAY-1 | arch test, depguard |
| `main.go` declares only `main` and `run`; wiring in `wire.go` | LAY-2, LAY-3 | arch test |
| Banned package suffixes (`*utils`) and banned file names | LAY-9, LAY-10 | arch test |
| No `README.md` under `internal/` or `cmd/` | LAY-11 | arch test |
| No database or cache driver imported by `domain` | DOM-4 | depguard |
| Every literal sets the fields tagged required | STY-3 | `requiredtag` (the standard's own plugin) |
| Interface assertion present on adapters and repositories; banned type-name suffixes; no stranded or duplicate interfaces | ABS-1, ABS-2, ABS-3 | arch test |
| No `With*` mutators or exported fields on services; `New` returns an error | ABS-5 | arch test |
| A stamp literal as the first argument of every `errx`, `httperr` and `logger` call | ERR-1, LOG-1 | the compiler (unexported `stamp` type) |
| A constant event name in every log call | LOG-3 | the compiler (unexported `eventName` type) |
| Log key names, and the format of event names | LOG-2, LOG-3 | review |
| Kind declarations: naming, uniqueness, used | ERR-5 | arch test |
| `go` statements only in `foundation/supervise` | CON-1 | arch test |
| `context.WithValue` only in `context.go` | CTX-2 | arch test |
| Repositories export only `Repo`; `repo.go` and `row.go` present | DATA-1, DATA-2 | arch test |
| Repository verb allow/deny list | DATA-3 | `repoverbs` analyzer |
| Queries are constant expressions | DATA-5 | `constquery` analyzer |
| Migration file names | DATA-11 | arch test |
| Tag honesty; fake hygiene; table and name rules; `time.Sleep` only in `synctest` | TEST-1, TEST-5, TEST-7, TEST-12 | arch test |
| Comment hygiene greps; no SQL `now()`; named `ON CONFLICT` targets | STY-5, DATA-6, DATA-8 | CI grep |
| One gofi handler per file, declared before its schema | HTTP-2 | arch test |
| A function reads as a story: title, guards, one level of detail, names | STY-9 | review (the paragraphs are `wsl_v5`) |
