# Decisions

This records what was decided while writing the standard, and why it matters. The rule documents have the detail, so each row names its rule.

## Parked

These haven't been decided, and nothing enforces them yet.

| Question | Where it stands |
|---|---|
| How do dashboards receive metrics? (OBS-2) | Telemetry is ignored until a project explicitly needs it, then added where required. |
| How are database tests kept apart? (TEST-3) | The rule describes per-package databases with unique tenant IDs, but it isn't decided. |
| Do HTTP handler tests use fakes or a real database? (TEST-11) | Left as it is. Fakes are fast and can force failures that are hard to create for real, such as a timeout. They prove nothing about your SQL. |
| Should gofi's OpenAPI mark an optional pointer with a spec, and a `[]byte`, as `nullable`? (DOM-1) | gofi v0.10.4 writes `null` for a nil `*TransferID` or a nil `[]byte`, but the document says the field is never null. Clients have to handle `null` on those fields until gofi changes. |

## Scope and ownership

| Question | Decision |
|---|---|
| Which projects does the standard cover? | Any Go project. It isn't tied to any company, host or product. |
| How do projects consume it, and how is it mandatory for agents? | A pinned `git subtree` copy, an `AGENTS.md` that `task check` protects, hooks and required CI ([10-adoption.md](10-adoption.md)). |
| Can a subfolder have its own `AGENTS.md`? | Yes, if a team wants one. `task check` doesn't look at it, so it's checked in review (ADO-3). |
| Can a `CLAUDE.md` hold its own text? | No. It's only the line `@AGENTS.md`, or a symlink to `AGENTS.md`, so there's one set of instructions (ADO-3). |
| How strict are we on existing projects? | Fix everything first, leaving existing tests as they are (ADO-6, TEST-14). |
| Who owns the standard? | Michael Olofinjana (COL-13). |
| What's the difference between `completed` and `done`? | `completed` means fully implemented, tested, verified when possible, and meeting the requirement, and the agent marks it. `done` only comes from the engineer, by an explicit instruction, after reviewing a completion document. Neither depends on merging (COL-8, COL-9, COL-10). |
| How much work goes for review at once? | A whole workstream, implemented end to end, unless the engineer explicitly asks for part of it. A small change with no plan or task list entry gets a completion document for visibility, with no `done` step (COL-5, COL-9, COL-19). |
| Where do completion documents live, and in what format? | `completion/<topic>.html`, beside `implementation/`. Each is a standalone HTML file, because HTML presents and explains work better. Nothing depends on one, so the engineer can delete it at any time (COL-9, COL-18). |
| What must a completion document show for code and data? | Every code block has syntax highlighting, with the colours inside the page. Any change to stored data gets its own Database section, with the schema, why it changed, values that changed meaning, reads and writes, and what happens to existing data (COL-9). |
| Why do agents need Playwright MCP? | To check that a completion document displays properly before sending it. Playwright MCP blocks `file:` URLs by default, so the agent serves the folder locally first (COL-9). |
| Can a task depend on one further down the list? | No. A task can only depend on tasks above it. A dependency in another workstream that isn't `done` yet is a warning, not a blocker (COL-15). |
| How does an agent know whether work must account for existing users, deployments or data? | Each scope on the task list is marked `greenfield`, `existing` or `unconfirmed`. The engineer sets it when the task list or a task is written. The agent asks instead of assuming, and nothing is planned or changed while it's `unconfirmed` (COL-15). |
| How many subagents can a session use? | At most one at a time, only when it's justified, and the agent says why before starting it. Only the engineer's explicit instruction raises the limit (COL-20). |
| How often are published attacks and vulnerabilities checked? | At least every 7 days, against the project's primary dependencies and not their dependencies. Nothing runs on a timer, so `security-checks.md` records each run and the next work to start runs a check that's due (SEC-27). |
| Do implementation plans list their sub-tasks? | Yes. Every plan has a table in the task list's format, with temporary IDs that exist only in that plan (COL-17). |
| Who deletes implementation plans? | Only the engineer, on an explicit instruction. A finished plan and its completion document stay in the folder (COL-18). |

## Layout

| Question | Decision |
|---|---|
| Which config library? | `caarlos0/env/v11` (LAY-13). |
| What's the folder for business-agnostic packages called? | `foundation/` (LAY-6). |
| Where do shared helpers go? | In `foundation/`. They don't have to share a package (LAY-9). |
| When do migrations run? | At release only, with a schema-version check at startup (LAY-5). |
| What runs migrations? | A command in `cmd/tools/`. Each project chooses what it's built on (DATA-11). |
| How are binaries named? | Short roles: `api`, `admin-api`, `worker` (LAY-1). |
| Can one binary run several processes? | Yes (LAY-4). |
| Do transports call repositories directly? | Yes, when the capability has no service. A service exists only when it's justified and approved (LAY-6, HTTP-3). |
| What goes in `main.go`? | `main` and `run`, and nothing else. Configuration types go in `config.go`, wiring in `wire.go` (LAY-2, LAY-3). |
| How many adapter packages does one external system get? | One, named for the system. Each role (cache, lock, queue) is a file in it, over one client. A second package is only for a second implementation of the same port, such as a local fake (LAY-8, ABS-7, DATA-10). |

## Style

| Question | Decision |
|---|---|
| Which Go version? | The same across a team's projects, at least 1.26. |
| How do we handle pointers? | `new(v)`, plus one shared `Deref` if needed. |
| Does the integer-overflow check (gosec G115) block merges? | Yes. |
| Which function-shape limits apply? | No length or statement limit. Gates are complexity 20, nesting 4, 6 parameters including `ctx`, and 3 results. Readability is checked by STY-9. |
| How are required struct fields marked? | With a struct tag on the field, never a comment. Fields stay optional unless tagged. The `requiredtag` plugin checks literals (STY-3). |
| Which tags mark a field as required? | Each project configures its own. The standard's config uses `validate:required` and `lint:required`. |
| Which naming conventions? | `Store` and `Repo`, `<Op>Args` and `<Op>Result`, and UPPER_SNAKE enum values. |
| How does a logged error say who it affects? | With `subject_id` and `subject_ref`. The HTTP encoder defaults to the signed-in caller, and decorators name anyone else. Where the code can't know, it stays empty (ERR-13). |
| Are existing enum wire values frozen? | Not on a `greenfield` scope. On an `existing` scope, renaming one is a breaking change (STY-2, DOM-6, COL-15). |
| How are header fields in gofi schemas tagged? | Lowercase kebab-case, as the header is sent, such as `json:"idempotency-key"` (STY-4, gofi TAG-2). The lint config lets any lowercase kebab-case name through, because tagliatelle can't tell a header struct apart. |

## Domain

| Question | Decision |
|---|---|
| How is money stored? | Always in its smallest unit, as an integer. Each project picks an integer type that fits (DOM-2). |
| What underlies typed IDs? | `[16]byte`, the original type of both ULIDs and UUIDs, with text only at the edges (DOM-1). |
| Are domain types the one type used across every boundary? | Yes, between layers. `domain` imports no driver (DOM-4). |
| Are database schema types domain types? | No, whatever the database. A schema type is private to the repository, and its fields may use domain types (DATA-2). |
| Does enumx guarantee an order? | No. `Values()` comes from a map, so code that needs an order sorts (DOM-6). |
| Where may an interface be declared away from its consumer? | In `ports/` when several packages share it, and in `foundation/` for the few every layer uses, such as `clock.Clock` (ABS-1). |
| Does gofi's `Deps` follow ABS-1's limit of 7 methods? | No. It has one accessor for each dependency, as gofi DEP-2 requires, and is the one interface with no limit. Its accessors may return interfaces. Its constructor returns a struct (ABS-1, ABS-2). |
| May a function return gofi's interfaces, such as `gofi.Router`? | Yes. ireturn allows them (ABS-2). |

## Errors and logging

| Question | Decision |
|---|---|
| How are stamps created? | By `task stamps`, which reads the clock, as the Unix time in milliseconds, wherever `__STAMP__` is written. Nobody types one, because agents told to read a clock still counted upward. A stamp has no meaning, so it's never ordered, numbered or versioned, and an edit may replace it (ERR-1). |
| Do we build linters for stamps and event names? | No linter. The compiler enforces a literal stamp and a constant event name (ERR-1, LOG-3). `task stamps:check` fails on a new stamp `task stamps` didn't hand out, or a new repeat. It only judges new stamps, so existing ones need no sweep (ERR-1). |
| Where do error kinds live? | All in `foundation/errkinds`, with no per-package kinds and no `<pkg>_` prefix. A kind is added only when code branches on it. A handler that wants its own client message keeps the general kind and adds `.WithMsg(...)` after checking it (ERR-5, ERR-7). |
| Does every error response include `retryable`? | Yes (ERR-7). |
| Are the errors from gofi's `c.Next()` and `c.Send()` wrapped? | No. Their errors go back unchanged, and lint's ignore list grows by those two methods only (ERR-3). |
| How are wrapcheck's gaps closed? | With the standard's own `wrapgaps` plugin, which reports direct returns in closures and generic calls, using wrapcheck's ignore lists. A check in the arch test wouldn't fail with the other lint, and wrapcheck skips closures on purpose (ERR-3). |
| What code does an unknown error get? | `UNHANDLED_SERVER_ERROR`, including a recovered panic. `INTERNAL_SERVER_ERROR` is for a 500 someone chose (ERR-7, ERR-11). |
| What does `httperr.Map` return? | `*httperr`, as `New` and `Wrap` do. There's no `HTTPError` interface (ERR-7). |
| How is a `ValidateAndBind` error returned? | Through `httperr.Map`, which keeps gofi's status and leaves gofi's text out of the error, because it can quote the request body (HTTP-3, ERR-7). |
| Does `httperr` have a `Newf`? | No. `fmt.Sprintf` stays banned in that package. `httperr.Wrap` with `errx.Newf` formats an issue (ERR-7, ERR-11). |
| What log level do 4xx errors get? | Warn (ERR-9). |
| Do log calls keep stamps? | Yes. The stamp is the first argument (where), and a constant event name follows `ctx` (what) (LOG-3). |
| How is the logger obtained? | It isn't. One process-wide logger, called through the `logger` package's functions. Nothing injects a `*slog.Logger`, except into `pkg/` libraries (LOG-1). |

## HTTP and background work

| Question | Decision |
|---|---|
| Which gofi version? | v0.10.4 or later. |
| Which handler style? | gofi's GUIDELINES.md: `<Operation>Handler` variables with `deps.MustGet(c)` (HTTP-1). |
| Is `OperationId` set? | No. The method and path identify the route (HTTP-5). |
| How do servers get limits? | One `r.Configure` call, with the usual `r.Listen` and `r.ShutdownWithContext` (SEC-17). |
| What's the default request timeout? | 30s, with longer limits on streaming and export routes (CTX-3). |
| Are inbound request IDs trusted? | No. The server generates every request ID and ignores `X-Request-ID` from the client. A reference the client chooses, such as an idempotency key, is a field in that endpoint's schema (HTTP-6). |
| What happens when a param can't be stored in its field? | A 500, and the handler doesn't run. |
| Does a failed assertion inside `Decode` that isn't on the input stay a 500? | Yes. |
| What's the background job contract? | One handler shape, `Handle(ctx, jobs.Job) error`, and each queue technology stays (JOB-1). |
| Where do job kinds and args live? | In `internal/jobs`, a flat layer beside `ports` that core, repositories, adapters and workers may import (JOB-1, LAY-6). |
| One producer port, or one per kind? | One: `jobs.Enqueuer`, taking a `jobs.Job` whose body is bytes. A kind's attempts, timeout and stream are registered once with the runtime (JOB-1). |
| Can a job carry its payload? | Only a hot-path job whose design needs it, said in its kind's doc comment. The payload moves as opaque bytes, sealed if sensitive, and is decoded once on the handler's side. The record stays the truth (JOB-1). |
| Can a sorted set index a business retry schedule? | Yes. The row stays the schedule, the sorted set is a hint the sweep repairs, and a schedule that runs out is a terminal state on the row, not a dead letter (JOB-2). |
| Where does the sweep live when the record and the queue are in different stores? | In core. It reads the record store through a repository and enqueues through `jobs.Enqueuer`. The queue adapter never reads the record store (JOB-3). |
| One stream per job kind? | No. Kinds share a stream when they share its claim idle time, delivery limit and concurrency, and a kind that needs different ones gets its own (JOB-4). |
| What is a dead-letter stream called? | `<service>:dlq:<queue>`, beside `<service>:ready:<queue>` and `<service>:due:<queue>` (JOB-4, DATA-10). |
| What stops a job writing to a tenant deleted while it ran? | A conditional write where one statement can check the tenant, otherwise a permit that deletion waits out. Drop and a cleanup that runs again aren't enough on their own (JOB-5). |

## Persistence

| Question | Decision |
|---|---|
| Can one statement write several tables, for example a transfer and its outbox row? | Yes, and it's the preferred form. Core composes writes in a transaction only as a last resort (DATA-4). |
| Where are queries declared? | Inside the method that runs them, unless several methods share them (DATA-5). |
| Can searches with many optional filters build SQL dynamically? | No need. Fixed queries return the same rows (DATA-5). |
| Are cursors signed? | They carry a filter fingerprint, with no signature (DATA-9). |
| How is "no cursor" written? | As the zero `Cursor`, checked with `IsZero()`, never a nil pointer. `Decode("")` returns the zero `Cursor` (DATA-9). |
| Do Redis queue keys have a TTL? | No. Item keys (cache, lock, rate limit, state) have one. A queue key (stream, schedule, dead-letter queue) states a bound instead, and versions its entries rather than its name (DATA-10). |
| Which records get a `Version` column? | Only those edited outside their state transitions. |

## Testing

| Question | Decision |
|---|---|
| Do we restructure existing tests? | Not for now. Existing tests are the guardrails during refactors (TEST-14). `t.Parallel()` is optional (TEST-8). |
| `require` or `assert`? | `require` only, for new projects. |
| How do table tests name an error kind? | A separate table per kind, for new projects. |
| How is test infrastructure started? | docker compose through `task test:up`, for new projects (TEST-3). |
| Where do flow tests live? | `tests/flows` (TEST-1). |
| Where is miniredis allowed? | Anywhere. |

## Deliberately not in the standard

| Topic | Why |
|---|---|
| Rejecting unknown JSON fields | gofi only reads fields in the schema, so unknown fields never reach a handler. |
| `default` tags in form bodies | No form needs them. gofi's guide says they're ignored. |
| Treating ID types as strings in gofi | Not needed. One spec per ID type works. |
| Fixing errx's two known bugs | The rules already avoid both ([04](04-errors.md#known-errx-bugs)), so the standard is complete either way. |
| Database settings such as Scylla tablets, and host details such as how to trust a provider's certificate | These belong to each deployment. The rule that TLS checks stay on in production stays (SEC-14). |
