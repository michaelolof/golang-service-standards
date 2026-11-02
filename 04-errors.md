# 04 · Errors

How errors are created, wrapped, classified, translated at boundaries, mapped to transports and logged. It covers `errx` usage.

It builds on [01](01-project-layout.md) (layers) and [03](03-domain-and-abstractions.md) (adapters translate SDK errors).

## The model in one paragraph

Every call that creates or wraps an error, and every log call, carries a **stamp**: a unique integer literal that identifies that line of code. A stamp replaces both stack traces and free-text wrap messages. An error carries a **kind** only when some caller branches on it. Repositories and adapters translate driver and SDK errors into kinds, keeping the cause. Each transport has exactly one place that turns kinds into a response (HTTP) or an outcome (workers). An error is logged once, where it stops travelling.

**Errors belong to a domain, one per audience:**

| Domain | Package and kinds | Audience | What it carries |
|---|---|---|---|
| Application | `errx`, with `errkinds` and capability kinds | the code | what went wrong, in business terms: `not_found`, `invalid_transition`, `crypto_key_wrapper_unavailable` |
| HTTP | `httperr`, with its own kinds (`NotFoundErr`, `BadRequestErr`, …) | an API client, and the access log | status code, stable `error_code`, client-safe message, retryable flag, and the subject (`WithSubject`) for logs |
| A future transport, such as a CLI | `clierr`, with its own kinds | a person at a terminal | exit code, colour, output format |

Application errors are created everywhere. A transport's errors are created only in that transport, and each transport has one translator from application kinds to its own kinds: `httperr.Map` for HTTP. So application kinds never mention status codes, and httperr kinds never leave the HTTP layer.

## Rules at a glance

| ID | Rule | Enforced by |
|---|---|---|
| ERR-1 | Every errx, httperr and log call takes a stamp: the Unix time in milliseconds, as a plain literal, read from a clock. A stamp has no meaning, so it's never ordered, numbered or versioned. | the compiler, review |
| ERR-2 | Use only `New`, `Newf`, `NewKind`, `NewKindf`, `Wrap`, `WrapKind` and `IsKind` from errx. For the rest, use stdlib `errors.Is/As/Join`. | forbidigo |
| ERR-3 | Wrap every error you return from a call exactly once with `errx.Wrap(stamp, err)`. | wrapcheck |
| ERR-4 | Create errors with `New*`, and add a kind only if someone branches on it. Messages are lowercase, describe the condition, use `key=%q`, and contain no PII. | forbidigo, review |
| ERR-5 | Application kinds: generic ones in `foundation/errkinds`, capability ones beside their owner with a `<pkg>_` prefix, and only if a caller branches on them. Each transport has its own error domain with its own kinds. | arch test |
| ERR-6 | Driver and SDK errors are inspected and translated only in repositories and adapters, with `errors.Is/As` and `errx.WrapKind`. | depguard, errorlint |
| ERR-7 | HTTP handlers return `httperr.Map(stamp, err)`. One mapper, one typed response body. | arch test, golden test |
| ERR-8 | Each job runtime has one mapping from kind to outcome. Handlers never choose an outcome. | review |
| ERR-9 | Return an error or log it, never both. Log only at roots. | review |
| ERR-10 | Discard close errors on readers. Join close errors on writers. Don't `Join` on paths that will be classified. | errcheck, review |
| ERR-11 | No `panic` outside tests and `recover` roots. A recovered panic becomes an error with the panic's type and stack. | forbidigo, arch test |
| ERR-12 | `fmt.Errorf` and `errors.New` are allowed only in tests and in standalone library modules. | forbidigo |

---

## ERR-1 · Stamps

**What a stamp is.**
- errx's stamp parameter has the unexported type `lint int`, so only an untyped integer literal compiles there.
- errx renders the stamp into `Error()` as `[ts N]`.
- `httperr` returns it to clients as `__sid`.
- When an error is logged as `"error", err` (ERR-9, LOG-2), its stamps go with it.
- One stamp therefore links a client's error response, the log line and the source line.
- Log calls carry a stamp of their own, as their first argument: `logger.Warn(1793603411123, ctx, "db.rollback.failed", …)`. The stamp says *where* the line was logged; the constant event name says *what* happened (LOG-3). Both are needed, because one event name can be logged from many places.

**Rule.**
- A stamp is the current Unix time in milliseconds when the line is written: a plain integer literal, 13 digits long.
- **A stamp has no meaning.** It isn't ordered, numbered, grouped by file or versioned. Never work one out from the stamps around it.
- **Every new stamp comes from a clock.** Agents can't know the time, so they must run a command. Each run is a fresh reading, so run it once for each stamp. Use whichever your system has:

  | Where | Command |
  |---|---|
  | Linux | `date +%s%3N` |
  | Linux, macOS | `python3 -c 'import time; print(time.time_ns() // 1000000)'` |
  | Anywhere with Node | `node -e 'console.log(Date.now())'` |
  | Windows PowerShell | `[DateTimeOffset]::UtcNow.ToUnixTimeMilliseconds()` |

  The Python and Node commands were tested. The other two come from their tools' documentation.

- **Editing a line?** You may overwrite its stamp with a newly generated one. Leave the other stamps in the file alone.
- Copying a line means taking a new stamp.
- No stamps in constants, variables or arithmetic.
- Tests use `0`.
- `httperr` takes an unexported `type stamp int`, not `type tid = int`. An alias lets variables through, so the "literal only" property breaks there.
- Nothing checks that stamps are unique. A repeated stamp costs little, because the log line's event name still says what happened, so it isn't worth a tool.
- Existing stamps don't need a sweep. They change only when their line is edited.

**Don't.** These count upwards through a file, as if the number meant something (tested: across three codebases, 4,112 of 6,081 stamps were within 20 of the previous stamp in their file):

```text
1778601001, 1778601002, 1778601003 … 1778601028
1786800000030, 1786800000031, 1786800000032 … 1786800000057
1773500205, 1773500206, 1773500207 … 1773500241
```

**Do.** Each one is a separate reading from a clock, with no relation to its neighbours:

```go
return errx.Wrap(1793606419171, err)
// ...
return errx.NewKind(1793611082554, errkinds.ErrKindInvalidInput, "amount must be non-negative")
```


**Why milliseconds, not nanoseconds.** `httperr` sends the stamp to clients as a JSON number, and JavaScript rounds large numbers (tested in Node):

```text
1793603411123        → 1793603411123         exact
1793603411123456789  → 1793603411123456768   rounded, so it no longer matches the log
```

**Enforced by** the compiler. Only a literal can be passed to an unexported type, so a variable, a call or a missing stamp doesn't compile.

## ERR-2 · The errx subset

**Rule.**

| Allowed | Use for |
|---|---|
| `errx.New`, `errx.Newf` | Creating an error nobody branches on |
| `errx.NewKind`, `errx.NewKindf` | Creating an error a caller branches on |
| `errx.Wrap` | Adding a stamp to an error you return |
| `errx.WrapKind` | Classifying at a boundary while keeping the cause |
| `errx.IsKind` | Branching on a kind |
| `errx.DataKind`, `errx.FindData` | Only when a transport reads the payload |
| stdlib `errors.Is`, `errors.As`, `errors.Join` | Everything else |

**Forbidden:**
- `Wrapf`, `WrapKindf`;
- `errx.Is`, `errx.As`, `errx.Join`, `errx.Unwrap`;
- `Contains`, `Must`, `Panic`, `UseLogger`, `ParseStampedError`.

`NewBuild` and `BuildFrom` are allowed only inside `http/httperr`.

**Why.** Tested on errx v0.5.5:

```text
Wrapf(2, "load user: %v", sharedErr); sharedErr.Error()       → "[ts 100] load user: shared failure"
Wrapf(2, "sign token user_id=%s", errors.New("boom"), "u-1")  → "sign token user_id=boom%!(EXTRA string=u-1)"
```

What these results mean:
- **`Wrapf` rewrites a shared errx error.** The first line is correct code, and the shared error's own message is changed for good. Each later call adds to it, and it's a data race if two requests do this together. This is a real errx bug.
- **`Wrapf` always puts the wrapped error in the first `%`.** errx's own tests use it this way, so it's by design. Most callers expect otherwise, as in the second line. A quick script over real code found 266 of 334 calls where the verbs and arguments don't line up.
- **`errx.Is` compares text when `errors.Is` says no.** That's a design choice. Use `errors.Is`, so the code has one way to do this.

[Known errx bugs](#known-errx-bugs), at the end of this document, lists this bug and what a fix would lift.

## ERR-3 · Wrap once, stamp only

**Rule.**
- Every error returned from a call you made is wrapped exactly once with `errx.Wrap(stamp, err)`.
- Add no message; the stamp is the context.
- Never wrap an error you created in the same function.
- Use `WrapKind` only when classifying at a boundary (ERR-6), and never re-apply a kind the error already has.

```go
w, err := s.wallets.Get(ctx, id)
if err != nil {
	return domain.Wallet{}, errx.Wrap(1791799737179, err)
}
if w.Status != domain.WalletStatusActive {
	return domain.Wallet{}, errx.NewKindf(1793401945029, errkinds.ErrKindInvalidTransition,
		"wallet not active wallet_id=%q status=%q", id, w.Status)
}
```

Stamp-only wrapping means a wrap site can't add identifiers.

**Enforced by** wrapcheck, with `ignoreSigs` set to the errx and httperr constructors.

## ERR-4 · Creating errors and writing messages

**Rule.**
- Use `New`/`Newf` when no caller branches on the error. Use `NewKind`/`NewKindf` when one does.
- Messages:
  - are lowercase, with no trailing punctuation and no package prefix;
  - describe the condition ("wallet not active"), not "failed to …";
  - render identifiers as `key=%q`;
  - never contain emails, tokens, secrets or payload contents.

**Do:**

```go
return errx.NewKindf(1793625663882, errkinds.ErrKindInvalidInput, "invalid BeneficiaryKind %q", k)
```

**Don't.** A misordered `Wrapf` that also puts an email address into logs:

```go
return false, errx.Wrapf(1793625664546, "check users_by_email email=%s", err, record.Email)
```

## ERR-5 · Kinds

**Rule.**
- **Generic kinds.** `foundation/errkinds` holds the classes transports distinguish:

  | Kind string | Meaning |
  |---|---|
  | `not_found` | |
  | `conflict` | Duplicate |
  | `invalid_input` | |
  | `invalid_transition` | |
  | `unauthenticated` | |
  | `forbidden` | |
  | `unavailable` | Transient dependency failure |
  | `rate_limited` | |

- **Capability kinds** live beside their owner (`core/<cap>/errors.go`), and only when a caller branches on them.
- **Naming.** The identifier is `ErrKind<Name>`. The string is snake_case. Outside `errkinds`, the string is prefixed `<pkg>_`. errx compares kinds by string, so the prefix is what keeps them unique.
- **Docs.** Each kind's doc comment says who branches on it.
- **Application kinds know nothing about transports.** They say what happened. The HTTP domain's own kinds (`httperr.NotFoundErr`, …) carry the status code, `error_code` and message, and `httperr.Map` translates between the two (ERR-7).
- **Where httperr lives.** `internal/http/httperr`, because only the HTTP layer uses it.

```go
// ErrKindKeyWrapperUnavailable marks a KMS outage; ingestion maps it to 503 and workers retry.
var ErrKindKeyWrapperUnavailable = errx.Kind("crypto_key_wrapper_unavailable")
```

**Why.**
- Kinds must only be added when code needs to make a decision. Without that rule:
  - A central list grew to 40 kinds. `ErrKindValidation` was created 16 times and had no mapping case, so it surfaced as a 500. `ErrKindInternal` was created 33 times and never checked.
  - Kinds named `XErr` collide visually with httperr's `NotFoundErr`, and kebab-case and snake_case strings get mixed.

**Enforced by** an arch test. It collects every `errx.Kind` declaration and asserts:
- unique strings;
- the naming pattern;
- at least one creation site and at least one non-test `IsKind` reference.

## ERR-6 · Translate driver errors at the boundary

**Rule.**
- Only `repository/*` and `adapters/*` inspect driver and SDK errors.
- Inspect them with `errors.Is`/`errors.As`. Never use `==`, a type assertion or `errx.Is`.
- Translate with `errx.WrapKind(stamp, kind, err)`, so the cause survives.
- An optional lookup is `Find…` and returns `(T, bool, error)`. A required lookup is `Get…` and returns `ErrKindNotFound`.
- No package-level error variables; return kinds.

**Do:**

```go
return ports.GenerateDataKeyResult{}, errx.WrapKind(1787900200002, crypto.ErrKindKeyWrapperUnavailable, err)
```

**Canonical:**

```go
if errors.Is(err, pgx.ErrNoRows) {
	return domain.Wallet{}, errx.WrapKind(1792750063048, errkinds.ErrKindNotFound, err)
}
return domain.Wallet{}, errx.Wrap(1793426755793, err)
```

**Don't.** The repository returns raw `gocql.ErrNotFound`, and 51 handler files test for it:

```go
if errors.Is(err, gocql.ErrNotFound) {
	return httperr.Wrap(1793625665015, httperr.NotFoundErr, err).WithMsg("api key not found")
```

**Don't.** The shape is right, but `NewKindf` drops the driver cause:

```go
if stderrors.As(err, &pgErr) && pgErr.Code == "23505" {
	return domain.APIKey{}, errx.NewKindf(1793625665557, errkinds.ErrKindDuplicateRequest, "api key hash collision")
}
```

**Enforced by:**
- depguard, which keeps driver packages inside `repository/**`, `adapters/**` and `cmd/**`;
- errorlint (`comparison`, `asserts`);
- forbidigo on `errx.Is`.

## ERR-7 · One HTTP mapper, one response body

**Rule.**
- **Handlers.** A handler returns `httperr.Map(stamp, err)` for any error from `core`. It may add a client message with `.WithMsg(...)`.
- **The mapper.** `Map` is the only switch from kind to HTTP error. It checks the most specific kind first. Unknown errors become `UNHANDLED_SERVER_ERROR`, carrying only the stamp.
- **The response body** is a typed struct, encoded with `encoding/json`.
- **Detecting an existing `HTTPError`** uses `errors.As`, not a type assertion. A type assertion fails once the error is wrapped.
- **`UseIssueAsMsg`** (send the error's own message to the client) is allowed only inside `Map`, and only for kinds whose errors are created with `NewKind*` and client-safe text. Never `WrapKind` a library error into such a kind, because the library's text would reach the client.
- **`httperr.New`** only when there is no underlying error. Used on an existing error, it drops the cause.

**Do.** One mapping point:

```go
func Map(ts stamp, err error) HTTPError {
	switch {
	case errx.IsKind(err, errkinds.ErrKindNotFound):
		return Wrap(ts, NotFoundErr, err)
	case errx.IsKind(err, errkinds.ErrKindInvalidInput):
		return Wrap(ts, BadRequestErr, err).UseIssueAsMsg()
	// ...
	default:
		return Wrap(ts, InternalServerErr, err)
	}
}
```

**Do.** A typed body:

```go
type errorBody struct {
	Status    string          `json:"status"`
	ErrorCode string          `json:"error_code"`
	Message   string          `json:"message"`
	Retryable bool            `json:"retryable"`
	Data      json.RawMessage `json:"data"`
	SID       int             `json:"__sid"`
}
```

**Don't.** Hand-built JSON. A message containing a `%q`-quoted value produces an invalid body:

```go
responseBody = fmt.Appendf(responseBody, `{"status": "error", "error_code": "%s", "message": "%s", "data": %s, "__sid": %d }`, errorCode, message, data, traceID)
```

**Don't.** `WithMsgf` does nothing:

```go
func (e *httperr) WithMsgf(msg string, v ...any) *httperr {
	if msg != "" {
	}
	return e
}
```

**Canonical handler path:**

```go
res, err := h.wallets.Activate(ctx, in)
if err != nil {
	return httperr.Map(1792900491461, err)
}
```

**Enforced by:**
- forbidigo on `fmt.(Appendf|Sprintf)` in `http/httperr`;
- a golden test of the error body;
- an arch test that only `internal/http/**` imports `httperr`.

## ERR-8 · One outcome mapping per job runtime

**Rule.**
- Each job runtime adapter (a database-backed queue, asynq, Redis streams, Kafka) has one function that maps error kinds to that runtime's outcomes: done, retry, drop, dead-letter, defer (JOB-2).
- Handlers return errors; they never choose an outcome.
- An unknown error is retried up to the maximum attempts, then dead-lettered.
- Don't use marker wrapper types such as `queue.Permanent`, found by type assertion. A wrapped marker is missed, and the job is retried forever.

**Why.** Without one mapping, the decision is spread across handlers. Only 3 of 24 handlers returned `queue.Cancel`, each with its own kind check, for example:

```go
if errx.IsKind(err, errkinds.ErrKindProviderRejected) {
	return queue.Cancel(err)
}
return err
```

The canonical mapping is in [05 · JOB-2](05-transport-and-runtime.md#job-2--the-runtime-owns-outcomes).

## ERR-9 · Handle once

**Rule.**
- Return an error or log it, never both.
- Log only at roots:
  - the HTTP error encoder (5xx at Error, 4xx at Warn);
  - the worker runner;
  - goroutine roots;
  - best-effort or post-commit paths that deliberately swallow an error.
- A log line has a constant event name and the error under the key `error`, as the error value (`"error", err`), so errx's structured value is kept (LOG-2).

**Why.**
- Central logging means an error is logged once, where it stops travelling.
- Without one log shape, the error key ends up as `err`, `error`, `issue` or `cause`, and holds `err` or `err.Error()`.
- A logger call that puts `err.Error()` in the message makes every log line unique. That defeats grouping and alerting.

## ERR-10 · Close and Join

**Rule.**
- **Read-only resources** (rows, cursors, response bodies): `defer rows.Close()`, or explicitly discard the error.
- **Writers and owners:** return the close error through a named result with `errors.Join`.
- **Classified paths:** errx's `IsKind` can't see through `errors.Join`, so don't join there. Return the primary error wrapped, and log the secondary one.

**Do:**

```go
func (s *Server) Run(ctx context.Context) (runErr error) {
	defer func() {
		closeCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		runErr = errors.Join(runErr, s.deps.Close(closeCtx))
	}()
```

**Don't.** A `NotFound` from `fn` becomes a 500 once it is joined with the rollback error:

```go
if err := fn(txCtx, tx); err != nil {
	rollbackErr := tx.Rollback(ctx)
	if rollbackErr != nil && !stderrors.Is(rollbackErr, pgx.ErrTxClosed) {
		return errx.Wrap(1793625666704, stderrors.Join(err, rollbackErr))
	}
```

**Canonical:**

```go
if rbErr := tx.Rollback(ctx); rbErr != nil && !errors.Is(rbErr, pgx.ErrTxClosed) {
	logger.Warn(1793544466554, ctx, "db.rollback.failed", "error", rbErr) // logged, not returned (LOG-1)
}

return errx.Wrap(1792391358387, fnErr)
```

## ERR-11 · Panics and recovery

**Rule.**
- No `panic`, `errx.Must` or `errx.Panic` in non-test code.
- `recover` appears only at roots:
  - the outermost HTTP middleware;
  - the worker dispatcher;
  - one shared `goSafe` helper for goroutines.
- A recovered panic becomes an error carrying the panic's type and `debug.Stack()`. Never `%v` the panic value, because it may contain secrets. Return that error so the encoder logs it once.
- **Request validation:** 400 `BAD_REQUEST_ERROR` with a generic message, and `data: {"location","field","rule"}` taken from gofi's validation error.
- **Response validation:** 500.

**Do.** This is close to the target, but it loses the stack:

```go
func recoveryMiddleware(c gofi.Context) (handlerErr error) {
	defer func() {
		if recover() != nil {
			logger.ErrorAttrs(1793625666959, "platform panic recovered", "method", c.Method(), "path", c.Path())
			handlerErr = httperr.New(1793625667805, httperr.InternalServerErr, "panic recovered")
		}
	}()
	return c.Next()
}
```

**Canonical:**

```go
defer func() {
	if r := recover(); r != nil {
		err = httperr.Newf(1791855812662, httperr.UnhandledServerErr, "panic recovered type=%T\n%s", r, debug.Stack())
	}
}()
```

## ERR-12 · Stdlib error constructors

**Rule.**
- `fmt.Errorf` and `errors.New` are allowed only in `_test.go` files and in `pkg/` libraries (LAY-1), such as a database-backed queue. Those are written for users outside the project and may not depend on project conventions.
- Everywhere else, use errx.

**Enforced by** forbidigo `^fmt\.Errorf$` and `^errors\.New$`, excluding `_test.go` and `pkg/`.

---

## Known errx bugs

Only two things in errx misbehave when the code calling it is correct. The standard's rules already avoid both, so nothing here depends on these fixes.

| What errx does | A fix would lift |
|---|---|
| `IsKind`, `IsDataKind` and `FindData` don't look inside joined errors. A repository joins a `NotFound` with a rollback failure, and `IsKind` then returns false, so the client gets a 500 instead of a 404. errx's own `Join` and `JoinWrap` lose the kind too (tested on errx v0.5.5). | The "don't Join on classified paths" rule (ERR-10) |
| `Wrapf` and `WrapKindf` change the error they wrap. A shared errx error's message is rewritten on every call (tested on errx v0.5.5). | Half of the `Wrapf` ban (ERR-2) |
