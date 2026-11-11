# 04 · Errors

How errors are created, wrapped, classified, translated at boundaries, mapped to transports and logged. It covers `errx` usage.

It builds on [01](01-project-layout.md) (layers) and [03](03-domain-and-abstractions.md) (adapters translate SDK errors).

## The model in one paragraph

Every call that creates or wraps an error, and every log call, carries a **stamp**: a unique integer literal that identifies that line of code. A stamp replaces both stack traces and free-text wrap messages. An error carries a **kind** only when some caller branches on it. Repositories and adapters translate driver and SDK errors into kinds, keeping the cause. Each transport has exactly one place that turns kinds into a response (HTTP) or an outcome (workers). An error is logged once, where it stops travelling.

**Errors belong to a domain, one per audience:**

| Domain | Package and kinds | Audience | What it carries |
|---|---|---|---|
| Application | `errx`, with every kind in `errkinds` | the code | what went wrong, in business terms: `not_found`, `invalid_transition`, `destination_blocked` |
| HTTP | `httperr`, with its own kinds (`NotFoundErr`, `BadRequestErr`, …) | an API client, and the access log | status code, stable `error_code`, client-safe message, retryable flag, and the subject (`WithSubject`) for logs |
| A future transport, such as a CLI | `clierr`, with its own kinds | a person at a terminal | exit code, colour, output format |

Application errors are created everywhere. A transport's errors are created only in that transport, and each transport has one translator from application kinds to its own kinds: `httperr.Map` for HTTP. So application kinds never mention status codes, and httperr kinds never leave the HTTP layer.

## Rules at a glance

| ID | Rule | Enforced by |
|---|---|---|
| ERR-1 | Every errx, httperr and log call takes a stamp: the Unix time in milliseconds, as a plain literal. Nobody types one: write `__STAMP__` and run `task stamps`, which reads the clock. A stamp has no meaning, so it's never ordered, numbered or versioned. | the compiler, `task stamps:check` |
| ERR-2 | Create, wrap and classify errors with errx, and inspect them with stdlib `errors.Is/As/Join`. Only the errx functions ERR-2 names as forbidden are banned. | forbidigo |
| ERR-3 | Wrap every error you return from a call exactly once with `errx.Wrap(stamp, err)`. The errors from gofi's `c.Next()` and `c.Send()` go back unchanged. | wrapcheck, `wrapgaps` |
| ERR-4 | Create errors with `New*`, and add a kind only if someone branches on it. Messages are lowercase, describe the condition, use `key=%q`, and contain no PII. | forbidigo, review |
| ERR-5 | Every application kind is declared in `foundation/errkinds`, with no per-package kinds and no `<pkg>_` prefix. A kind is added only when code branches on it, and choosing a message isn't a branch. Each transport has its own error domain with its own kinds. | `errkinds`, `tools/archcheck`, review |
| ERR-6 | Driver and SDK errors are inspected and translated only in repositories and adapters, with `errors.Is/As` and `errx.WrapKind`. | depguard, errorlint |
| ERR-7 | HTTP handlers return `httperr.Map(stamp, err)`, for bind errors too. One mapper, one typed response body. | `errmap`, `layers`, forbidigo, project test (golden body) |
| ERR-8 | Each job runtime has one mapping from kind to outcome. Handlers never choose an outcome. | review |
| ERR-9 | Return an error or log it, never both. Log only at roots. | review |
| ERR-10 | Discard close errors on readers. Join close errors on writers. Don't `Join` on paths that will be classified. | errcheck, review |
| ERR-11 | No `panic` outside tests and `recover` roots. A recovered panic becomes an error with the panic's type and stack. | forbidigo |
| ERR-12 | `fmt.Errorf` and `errors.New` are allowed only in tests and in standalone library modules. | forbidigo |
| ERR-13 | Every logged error says who it affects, wherever the code can know, as `subject_id` and `subject_ref`. The encoder defaults to the signed-in caller, and decorators name anyone else. IDs must be verified, never personal data, and never set on a shared error. | project test, review |

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
- A stamp is the Unix time in milliseconds when the line is written: a plain integer literal, 13 digits long.
- **A stamp has no meaning.** It isn't ordered, numbered, grouped by file or versioned. Never work one out from the stamps around it.
- **Nobody types a stamp, people included.** Write `__STAMP__` where one belongs, then run `task stamps`:

  ```go
  return errx.Wrap(__STAMP__, err)
  ```

  ```sh
  task stamps
  ```

  `__STAMP__` is a valid Go identifier, so the file still parses until it's filled. `task stamps` replaces each `__STAMP__` in the project's Go code with its own clock reading. It never hands out a stamp the project already has, and it spaces the readings out and shuffles them, so neighbouring stamps have nothing in common.
- **Editing a line?** You may replace its stamp with `__STAMP__` and run `task stamps` again. Leave the other stamps in the file alone.
- Copying a line means writing `__STAMP__` in the copy.
- No stamps in constants, variables or arithmetic.
- Tests use `0`. `task stamps` refuses a `__STAMP__` in a test file.
- `httperr` takes an unexported `type stamp int`, not `type tid = int`. An alias lets variables through, so the "literal only" property breaks there.
- Existing stamps don't need a sweep. They change only when their line is edited.

**Why a tool.** A model can't read a clock unless it runs something, and it tends to count upward from the stamp next to it. Agents told to run a clock command once for each stamp still did, so `task stamps` reads the clock for them.

**Don't.** These count upwards through a file, as if the number meant something (tested: across three codebases, 4,112 of 6,081 stamps were within 20 of the previous stamp in their file). `task stamps:check` reports each new one as not handed out by `task stamps`, or as a repeat if the number is already taken:

```text
1778601001, 1778601002, 1778601003 … 1778601028
1786800000030, 1786800000031, 1786800000032 … 1786800000057
1773500205, 1773500206, 1773500207 … 1773500241
```

**Do.** Each one came from `task stamps`, so it's a separate reading from a clock, with no relation to its neighbours:

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

**Enforced by** the compiler and `task stamps:check`.
- **The compiler:** only a literal can be passed to an unexported type, so a variable, a call or a missing stamp doesn't compile.
- **`task stamps:check`:** it's part of `task check:fast`, so the agent's finish hook, the pre-push hook and CI all run it (ADO-4, ADO-5). It fails on four things. The first two judge only *new* stamps, which are those the pushed code doesn't have, so existing stamps are left alone:

  | What | What it says |
  |---|---|
  | A new stamp that `task stamps` didn't hand out | `stamp 1793889041029 at internal/repository/accounts.go:52 wasn't handed out by task stamps` |
  | A new repeat, such as a copied line | `stamp 1793889041028 is used 2 times: internal/repository/hot_cleanup.go:16, internal/repository/accounts.go:52` |
  | A stamp from the future, so it can't have come from a clock | `stamp 1799999999999 at internal/repository/accounts.go:52 is in the future, so it didn't come from a clock` |
  | A `__STAMP__` nobody filled | `internal/repository/accounts.go still has __STAMP__: run task stamps` |

  Each message ends with what to do: replace the stamp with `__STAMP__` and run `task stamps`.

**How the check knows where a stamp came from.** `task stamps` writes each stamp it hands out to `.git/stamps-issued`, which is never committed. A new stamp that isn't in that file was typed.

**What "new" means.** The check compares the stamps in the working tree with the pushed code: the pushed commits that unpushed ones build on, or the current commit when nothing is unpushed. A project with no remote has nothing pushed, so its last commit stands in. So:
- old repeats aren't reported;
- a line that only moved keeps its stamp, because its stamp is already in the pushed code;
- a stamp in an unpushed commit is still new, so committing doesn't get past the check.

**What it can't catch.**
- **It only knows where a stamp came from before it's pushed.** The record is on one clone. In CI everything is pushed, so CI only catches a `__STAMP__` left in the code and stamps from the future.
- **Anyone can write to the record.** It stops a habit, not someone set on getting around it.
- **It doesn't check tests.** A stamp other than `0` in a test file is left to review.
- **Outside git it has nothing to compare with.** Then every stamp counts as new, so it reports every repeat, and it can't tell where a stamp came from.

## ERR-2 · Which errx functions to use

**Rule.**

| Use | For |
|---|---|
| `errx.New`, `errx.Newf` | Creating an error nobody branches on |
| `errx.Kind` | Defining a kind |
| `errx.NewKind`, `errx.NewKindf` | Creating an error a caller branches on |
| `errx.Wrap` | Adding a stamp to an error you return |
| `errx.WrapKind` | Classifying at a boundary while keeping the cause |
| `errx.IsKind` | Branching on a kind |
| `errx.Cause` | Reading the deepest error in a chain. The standard library has no function for this, because `errors.Unwrap` goes one level. |
| `errx.CauseMessage` | Getting the deepest error's own text, with no stamps |
| `errx.DataKind`, `errx.FindData` | Only when a transport reads the payload |
| stdlib `errors.Is`, `errors.As`, `errors.Join` | Inspecting and combining errors |

`NewBuild` and `BuildFrom` are allowed only inside `http/httperr`.

**Forbidden.** Only these are banned:

| Forbidden | Why | Use instead |
|---|---|---|
| `Wrapf`, `WrapKindf` | They rewrite a shared error's message, and they put the wrapped error in the first `%` (results below) | `errx.Wrap`, `errx.WrapKind` |
| `errx.Is` | It also matches on text, so two different errors with the same message count as equal | `errors.Is` |
| `errx.As`, `errx.Join`, `errx.Unwrap` | Each one only calls the standard library function, so there's no reason to have two ways | `errors.As`, `errors.Join`, `errors.Unwrap` |
| `Contains` | It matches on text, so rewording an error changes what the code does | `errors.Is`, `errors.As`, `errx.IsKind` |
| `Must`, `Panic` | They panic (ERR-11) | Return the error |
| `UseLogger` | It has no effect: errx stores the logger and never reads it | The `logger` package (LOG-1) |
| `ParseStampedError` | It rebuilds an error from its text, so `errors.Is` no longer finds the original | Keep the error value |

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

`Cause` and `CauseMessage` are allowed, but each has one thing to watch for. Tested on errx v0.5.5:

```text
err := errx.Wrap(1793662430954, errx.WrapKind(1793662430949, kind, notFound))

err.Error()                                → "[ts 1793662430954]; [ts 1793662430949 kind missing]; not found"
errx.IsKind(err, kind)                     → true
errx.IsKind(errx.Cause(err), kind)         → false
errx.Wrap(1793662430959, errx.Cause(err))    → "[ts 1793662430959]; not found"

own := errx.Wrap(1793662430969, errx.NewKind(1793662430964, kind, "amount must be positive"))

own.Error()                                → "[ts 1793662430969]; [ts 1793662430964 kind missing] amount must be positive"
errx.CauseMessage(own)                     → "amount must be positive"
errx.CauseMessage(err)                     → "not found"
```

- **`Cause` returns only the deepest error.** The stamps and kinds above it are gone, as the sample shows. So don't return it, wrap it or branch on it. To branch, use `errors.Is`, `errors.As` or `errx.IsKind`, which search the whole chain. `errors.Is` and `errors.As` also search joined errors, where `Cause` stops at the join. `errx.IsKind` doesn't, so don't join on classified paths (ERR-10).
- **`CauseMessage` returns the deepest error's text, whoever wrote it.** That's what makes it right for `UseIssueAsMsg` (ERR-7), because `err.Error()` would send stamps to the client. It's also why ERR-7's condition applies: send it to a client only if that text was written for clients. On a joined error, it returns the joined text with its stamps.

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

**The two gofi methods that aren't wrapped.** `return c.Next()` and `return c.Send(200, s.Ok)` are correct as they are:
- **`c.Next()`.** gofi MDW-6 says to return its error unchanged. That error already has its stamp from further in. Wrapping adds the same stamps to every failure on the route, and `__sid` doesn't change. Here is one failing request through five middleware, with the middleware's stamps made up:

  ```text
  Bare:    [ts 1793803231681]; [ts 1793732054971]; gocql: no hosts available in the pool
  Wrapped: [ts 1000000000001]; [ts 1000000000002]; [ts 1000000000003]; [ts 1000000000004]; [ts 1000000000005]; [ts 1793803231681]; [ts 1793732054971]; gocql: no hosts available in the pool
  ```

  There's also a trap: `errx.Wrap(ts, nil)` returns a non-nil error, so `return errx.Wrap(ts, c.Next())` fails every successful request.
- **`c.Send()`, `c.SendString()`, `c.SendBytes()` and `c.SendStream()`.** These are a handler's last line (HTTP-3). There's no later step for a stamp to tell apart.

**Enforced by** two linters, which share one ignore list: the errx and httperr constructors, and those two gofi methods.
- **wrapcheck** reports `return pkg.F()` and `err := pkg.F()` then `return err`.
- **`wrapgaps`**, the standard's own plugin in [lint/wrapgaps](lint/wrapgaps), reports the two forms wrapcheck skips. wrapcheck stops at a closure on purpose, and it can't read a call written with type arguments:

  ```go
  // A direct return in a closure.
  err := db.InTx(ctx, func(tx store.Tx) error {
  	return repo.Save(ctx, tx, w) // error returned from a closure should be wrapped
  })

  // A generic call, in any function.
  s, err := gofi.ValidateAndBind[createSchema](c)
  if err != nil {
  	return err // error returned from a generic function should be wrapped
  }
  ```

Neither linter checks a call within the same package, so review covers that.

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
- **One list.** Every application kind is declared in `foundation/errkinds`, and nowhere else. No package declares kinds of its own, and no kind string has a `<pkg>_` prefix.
- **A kind exists only for a branch.** Add one only when code checks it and does something different:
  - core or a worker retries, skips, stops or falls back;
  - a transport picks a status, `error_code` or job outcome that no other kind gives.

  Choosing a message isn't a branch. The error's message describes it in the log (ERR-4), and a handler's `.WithMsg(...)` describes it to the client (ERR-7).
- **Reuse before adding.** A missing API key is `not_found`, not `apikeys_key_not_found`. Before adding a kind, check whether a general one already gives callers the same branch.
- **The general kinds** every project starts with:

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

- **Naming.** The identifier is `ErrKind<Name>`. The string is snake_case, with no prefix. errx compares kinds by string, and one list is what keeps the strings unique.
- **Docs.** Each kind's doc comment says who branches on it, and what they do.
- **Why `foundation`.** Every layer checks kinds, `foundation` packages included (`pagination.Decode` returns `invalid_input`), and `foundation` imports only `foundation` (LAY-7). `errkinds` is the one `foundation` package that names business conditions.
- **Application kinds know nothing about transports.** They say what happened. The HTTP domain's own kinds (`httperr.NotFoundErr`, …) carry the status code, `error_code` and message, and `httperr.Map` translates between the two (ERR-7).
- **Where httperr lives.** `internal/http/httperr`, because only the HTTP layer uses it.

**Do.** A kind beyond the general ones, with the branch that justifies it in its doc comment:

```go
// internal/foundation/errkinds/errkinds.go

// ErrKindDestinationBlocked marks an outbound call the SSRF rules refused.
// Delivery checks it: a refused call never reached the destination, so delivery
// records a known failure instead of an unknown outcome.
var ErrKindDestinationBlocked = errx.Kind("destination_blocked")
```

**Don't.** A kind whose only reader is the HTTP mapper, there to pick a message. Core re-wraps the repository's `not_found`, and the mapper turns it back into a 404:

```go
// internal/core/apikeys/errors.go
var ErrKindKeyNotFound = errx.Kind("apikeys_key_not_found")

// internal/core/apikeys/revoke_org_key.go
key, err := s.orgKeys.Get(ctx, org, keyID)
if errx.IsKind(err, errkinds.ErrKindNotFound) {
	return domain.OrgAPIKey{}, errx.WrapKind(1793803142679, ErrKindKeyNotFound, err)
}

// internal/http/httperr/map.go
case errx.IsKind(err, apikeys.ErrKindKeyNotFound):
	return Wrap(ts, NotFoundErr, err).WithMsg("api key not found")
```

**Do.** Core passes the `not_found` on, and the handler that wants its own message adds it after checking the kind (ERR-7):

```go
// internal/core/apikeys/revoke_org_key.go
key, err := s.orgKeys.Get(ctx, org, keyID)
if err != nil {
	return domain.OrgAPIKey{}, errx.Wrap(1794361268839, err)
}

// internal/http/platform/apikeys/revoke_org_key.go
key, err := keys.RevokeOrgKey(ctx, org, keyID)
if errx.IsKind(err, errkinds.ErrKindNotFound) {
	return httperr.Map(1794361270373, err).WithMsg("api key not found")
}

if err != nil {
	return httperr.Map(1794361272298, err)
}
```

**Why.**
- Kinds must only be added when code needs to make a decision. Without that rule, kinds pile up wherever they're declared:
  - One codebase grew 108 kinds outside `errkinds`. 90 were read only by the HTTP mapper, most of them to pick a message for a status a general kind already gave, and 2 were read only by tests. `apikeys_key_not_found` above is one of them.
  - A central list grew to 40 kinds. `ErrKindValidation` was created 16 times and had no mapping case, so it surfaced as a 500. `ErrKindInternal` was created 33 times and never checked.
- Kinds declared package by package get declared twice. Two packages each declared "source not found": `sources_source_not_found` and `domain_source_not_found`. A caller that checks one misses the other. In one list, the second shows up next to the first.
- Kinds named `XErr` collide visually with httperr's `NotFoundErr`, and kebab-case and snake_case strings get mixed.

**Enforced by:**
- the `errkinds` analyzer, on every `errx.Kind` call in code that isn't a test:
  - it's in `internal/foundation/errkinds`, as a package variable;
  - its name is `ErrKind<Name>`;
  - its string is a snake_case literal that no other kind has.
- `tools/archcheck`, across the module: each kind is created somewhere (`errx.NewKind`, `NewKindf`, `WrapKind` or `WrapKindf`), and checked somewhere (`errx.IsKind`), outside `errkinds` and tests. A kind passed anywhere else, such as into a mapping table, counts as both.
- review, which checks that the check is a branch and not a choice of message, and that a string has no package prefix. A machine can't tell either apart.

## ERR-6 · Translate driver errors at the boundary

**Rule.**
- Only `repository/*` and `adapters/*` inspect driver and SDK errors.
- Inspect them with `errors.Is`/`errors.As`. Never use `==`, a type assertion or `errx.Is`.
- Translate with `errx.WrapKind(stamp, kind, err)`, so the cause survives.
- An optional lookup is `Find…` and returns `(T, bool, error)`. A required lookup is `Get…` and returns `ErrKindNotFound`.
- No package-level error variables; return kinds.

**Do:**

```go
return ports.GenerateDataKeyResult{}, errx.WrapKind(1787900200002, errkinds.ErrKindUnavailable, err)
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
- **Handlers.** A handler returns `httperr.Map(stamp, err)` for any error from `core`. It may add a client message with `.WithMsg(...)` after checking the kind, which stays a general one (ERR-5). `WithMsg` replaces the message whatever the status, so an unchecked `.WithMsg("api key not found")` also goes out on the 503 for a database outage. The handler names the party affected with `.WithSubject(...)` when that isn't the caller (ERR-13).
- **The mapper.** `Map` is the only switch from an error to an HTTP error. It checks gofi's own errors, then the most specific kind.
- **Unknown errors** become `UNHANDLED_SERVER_ERROR`, carrying only the stamp, so searching for that code finds every error nobody mapped. `INTERNAL_SERVER_ERROR` is for a 500 someone chose, such as a response that fails validation (ERR-11).
- **gofi's errors.** A `*gofi.HTTPError` keeps the status gofi chose, so a wrong content type stays a 415. A validation error follows ERR-11. Its text stays out of the error, because gofi's parser quotes the request body, and a token in that body would reach the log:

  ```text
  Body:   {"name":"a"} tok_example
  Error:  unexpected tail: "tok_example"
  Logged: validation failed location=Body field= rule=parser
  ```

- **The return type.** `Map`, `New` and `Wrap` return `*httperr`, and there's no `HTTPError` interface. ireturn rejects a function that returns a named interface, and an interface carrying `WithMsg` and `WithSubject` as well as the getters goes past interfacebloat's limit of 7. Callers can chain on `*httperr` without naming it.
- **The encoder.** An error that isn't an `*httperr` also goes through `Map`, so a route that doesn't exist gets the same body as anything else. It keeps its own stamp if it has one.
- **The response body** is a typed struct, encoded with `encoding/json`.
- **Detecting an existing `*httperr`** uses `errors.As`, not a type assertion. A type assertion fails once the error is wrapped.
- **`UseIssueAsMsg`** (send the error's own message to the client) is allowed only inside `Map`, and only for kinds whose errors are created with `NewKind*` and client-safe text. Never `WrapKind` a library error into such a kind, because the library's text would reach the client.
- **`httperr.New`** only when there is no underlying error. Used on an existing error, it drops the cause, which is what `Map` wants for a validation error and nothing else.
- **There is no `httperr.Newf`.** It needs `fmt.Sprintf`, which lint bans in this package so that nobody builds a JSON body by hand. To format an issue, create it with `errx.Newf` and pass it to `httperr.Wrap` (ERR-11).

**Do.** One mapping point:

```go
func Map(ts stamp, err error) *httperr {
	var (
		validation gofi.ValidationError
		framework  *gofi.HTTPError
	)

	switch {
	case errors.As(err, &validation):
		return fromValidation(ts, validation)
	case errors.As(err, &framework):
		return Wrap(ts, kindForStatus(framework.Code), err)
	case errx.IsKind(err, errkinds.ErrKindNotFound):
		return Wrap(ts, NotFoundErr, err)
	case errx.IsKind(err, errkinds.ErrKindInvalidInput):
		return Wrap(ts, BadRequestErr, err).UseIssueAsMsg()
	// ...
	default:
		return Wrap(ts, UnhandledServerErr, err)
	}
}

// fromValidation describes a gofi validation error without its text, which can
// quote the request body.
func fromValidation(ts stamp, validation gofi.ValidationError) *httperr {
	location, field, rule := string(validation.SchemaType()), validation.SchemaValue(), validation.Rule()
	issue := "validation failed location=" + location + " field=" + field + " rule=" + rule

	if validation.Type() == gofi.ResponseErr {
		return New(ts, InternalServerErr, issue)
	}

	return New(ts, BadRequestErr, issue).WithMsg("Invalid request").
		WithData(validationData{Location: location, Field: field, Rule: rule})
}

type validationData struct {
	Location string `json:"location"`
	Field    string `json:"field"`
	Rule     string `json:"rule"`
}

// kindForStatus turns the status gofi chose into the project's kind.
func kindForStatus(status int) Kind {
	switch status {
	case 400:
		return BadRequestErr
	case 404:
		return NotFoundErr
	case 405:
		return MethodNotAllowedErr
	case 413:
		return PayloadTooLargeErr
	case 415:
		return UnsupportedMediaTypeErr
	// ...
	default:
		return UnhandledServerErr
	}
}
```

**Do.** The encoder's way in. An error no handler mapped goes through `Map`:

```go
e, ok := errors.AsType[*httperr](err)
if !ok {
	e = Map(1793869292495, err)

	var stamped errx.StampedErr
	if errors.As(err, &stamped) {
		e.sid = stamped.Stamp()
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

`__sid` is a fixed name, not snake_case. The shared lint config excludes tagliatelle's finding for that field and no other.

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
- the `errmap` analyzer: every error a handler returns is `nil`, a call into `httperr` such as `httperr.Map(stamp, err).WithMsg(...)`, a gofi `c.Send` or `c.Next`, or the error of a handler helper that takes the `gofi.Context`, whose own returns are checked the same way. Wrapping one of these once more with `errx.Wrap` is fine, because the encoder still finds the `*httperr`;
- forbidigo on `fmt.(Appendf|Sprintf)` in `httperr`, and on `UseIssueAsMsg` outside it;
- ireturn, on anything returning a named interface;
- the `layers` analyzer: only `internal/http` imports `httperr`, because no other layer may import `http` (LAY-7);
- a project test that compares the error body with a golden file.

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
- A log line has a constant event name and the error under the key `error`, as the error value (`"error", err`), so errx's structured value is kept (LOG-2). It says who the error affects, when the code can know (ERR-13).

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
- **Request validation:** 400 `BAD_REQUEST_ERROR` with a generic message, and `data: {"location","field","rule"}` taken from gofi's validation error. `Map` does this (ERR-7).
- **Response validation:** 500 `INTERNAL_SERVER_ERROR`.

**Do.** This is close to the target, but it loses the stack:

```go
func recoveryMiddleware(c gofi.Context) (handlerErr error) {
	defer func() {
		if recover() != nil {
			logger.ErrorAttrs(1793625666959, "platform panic recovered", "method", c.Method(), "path", c.Path())
			handlerErr = httperr.New(1793625667805, httperr.UnhandledServerErr, "panic recovered")
		}
	}()
	return c.Next()
}
```

**Canonical:**

```go
defer func() {
	if r := recover(); r != nil {
		err = httperr.Wrap(1791855812662, httperr.UnhandledServerErr,
			errx.Newf(1793869292528, "panic recovered type=%T\n%s", r, debug.Stack()))
	}
}()
```

A panic has no underlying error, so `errx.Newf` creates one to carry the detail, and `httperr.Wrap` gives it an HTTP kind. ERR-3's "never wrap an error you created" is about `errx.Wrap`, which would only add a second stamp. A panic is an unknown error, so its code is `UNHANDLED_SERVER_ERROR` (ERR-7).

**Enforced by** forbidigo, outside tests: `panic`, `errx.Must` and `errx.Panic` are banned, and so is `recover`, except at a root. Each root says so where it recovers:

```go
if r := recover(); r != nil { //nolint:forbidigo // the outermost middleware: a panic becomes an UNHANDLED_SERVER_ERROR (ERR-11)
```

## ERR-12 · Stdlib error constructors

**Rule.**
- `fmt.Errorf` and `errors.New` are allowed only in `_test.go` files and in `pkg/` libraries (LAY-1), such as a database-backed queue. Those are written for users outside the project and may not depend on project conventions.
- Everywhere else, use errx.

**Enforced by** forbidigo `^fmt\.Errorf$` and `^errors\.New$`, excluding `_test.go` and `pkg/`.

## ERR-13 · Say who an error affects

**Rule.** Every logged error says who it affects, wherever the code can know. When someone reports a problem, this is how an engineer finds their errors: by searching for `subject_id`.

| Log key | Set with | What it holds |
|---|---|---|
| `subject_id` | `WithSubjectID(id)` | The stable internal ID of the party affected, such as a tenant, organisation, user or project. It must be verified: it comes from a checked credential or a record the code loaded. |
| `subject_ref` | `WithSubjectRef(ref)` | The reference people outside the service use, such as the client's own reference for a request, or an ID the caller sent that nothing has checked yet. |

`WithSubject(id, ref)` sets both, and an empty argument changes nothing. The subject is for logs only, so it never goes in the response. It's never an email, phone number, name or token, even as a ref (SEC-8).

**What to do depends on what the code knows when the error happens:**

| The situation | What to do |
|---|---|
| A signed-in caller acting on their own data | Nothing in the handler. The encoder uses the caller as the subject. |
| Staff or a service acting on someone else's data | Decorate each error once the code knows whose data it is. The affected party replaces the caller. |
| No sign-in, but the code has identified a record, such as an inbound webhook, or a password reset after its token is found | Decorate each error from that point on. |
| A job, consumer or goroutine root | Put the same two keys in the log call, from the job's payload. |
| Nothing is known yet: a missing or bad credential, an unmatched route, an oversized body, a login for an unknown account, a health check, startup | Leave it empty. Don't write a placeholder such as `unknown`. |

"Wherever the code can know" means what's already in scope. Never run an extra query just to fill in a subject, because that adds work and a new way to fail while handling an error.

**Decorate only an error created in this request.** The decorators change the error in place, so a package-level error is shared by every request. Create a new error instead.

**Do.** The encoder is the one place that logs an HTTP error (ERR-9), so one change here covers every signed-in route. A subject set on the error wins:

```go
// internal/http/httperr/encoder.go
// Encoder writes the response and logs the error. A subject set on the error
// wins. Without one, subject says who the request belongs to.
func Encoder(subject func(c gofi.Context) (id, ref string)) func(err error, c gofi.Context) {
	return func(err error, c gofi.Context) {
		encoded := EncodeError(err)
		if encoded.SubjectID == "" && encoded.SubjectRef == "" {
			encoded.SubjectID, encoded.SubjectRef = subject(c)
		}

		c.Writer().Header().Set("Content-Type", "application/json")
		_ = c.SendBytes(encoded.StatusCode, encoded.Body)

		if encoded.StatusCode >= 500 {
			logger.Error(1793829773381, c.Context(), "http.error", encoded.LogAttrs(err)...)
			return
		}
		logger.Warn(1793829773305, c.Context(), "http.error", encoded.LogAttrs(err)...)
	}
}
```

The project supplies the function that reads its principal. The auth middleware has put the principal on the request context (CTX-2), and the encoder can read it there. A request rejected before that point has no principal, so the subject stays empty:

```go
// internal/http/authn/subject.go
// CallerSubject says who the request belongs to. Before auth succeeds there's
// no principal, so it returns nothing.
func CallerSubject(c gofi.Context) (id, ref string) {
	principal, ok := PrincipalFrom(c.Context())
	if !ok {
		return "", ""
	}
	return principal.UserID.String(), ""
}
```

```go
// internal/http/routes/routes.go
r.UseErrorHandler(httperr.Encoder(authn.CallerSubject))
```

**Do.** Staff freeze a tenant's wallet. The caller is a staff member, but the person affected is the tenant:

```go
if err := wallets.Freeze(ctx, wallet.ID); err != nil {
	return httperr.Map(1793829836716, err).WithSubject(wallet.TenantID.String(), wallet.ClientRef)
}
```

**Do.** A job root has no `httperr`, so it logs the same keys:

```go
logger.Error(1793829836658, ctx, "wallet.sync.failed", "subject_id", tenantID.String(), "subject_ref", wallet.ClientRef, "error", err)
```

**Don't.** An ID nobody has checked goes in `subject_id`. A caller could send another customer's ID, and whoever debugs from that log would be misled:

```go
return httperr.Wrap(1793829836541, httperr.BadRequestErr, err).WithSubjectID(req.ProjectID)
```

**Do.** Until it's checked, it's a ref:

```go
return httperr.Wrap(1793829836427, httperr.BadRequestErr, err).WithSubjectRef(req.ProjectID)
```

**Don't.** An email in the log:

```go
return httperr.Wrap(1793829836486, httperr.UnauthorizedErr, err).WithSubjectRef(req.Email)
```

**Don't.** Decorate a shared error. Here, one tenant's ID stays on the error and gets logged for later requests from other tenants (tested):

```go
var errInvalidCursor = httperr.Wrap(1793829836593, httperr.BadRequestErr, errx.New(1793829836807, "invalid cursor"))

return errInvalidCursor.WithSubjectID(wallet.TenantID.String())
```

**Do.** Create one for each request:

```go
return httperr.Wrap(1793829836767, httperr.BadRequestErr, err).WithSubjectID(wallet.TenantID.String())
```

**Why.** A user says "my transfer failed at about 3pm". Without a subject, an engineer has only a time and a path to search with, across every customer's errors. With one, they can search for that customer and see exactly what happened to them. Putting the default in the encoder means nobody has to remember it on each error, and a decorator is only needed where the person affected isn't the caller.

The encoder can read the principal because gofi runs the error handler on the same request. On gofi v0.10.5, it saw a principal set with `SetRequestContext` by a route group's middleware, including after a recovered panic. It saw none when that middleware rejected the request, or when no route matched (tested).

**Enforced by:**
- a project test on the encoder: a signed-in route logs the caller, a decorator replaces the caller, a rejected request logs no subject, and the response body never contains one;
- review for everything else, because no linter can tell what the code knew when the error happened. A handler that returns an error without a subject, after loading the record that names the affected party, goes back.

---

## Known errx bugs

Only two things in errx misbehave when the code calling it is correct. The standard's rules already avoid both, so nothing here depends on these fixes.

| What errx does | A fix would lift |
|---|---|
| `IsKind`, `IsDataKind` and `FindData` don't look inside joined errors. A repository joins a `NotFound` with a rollback failure, and `IsKind` then returns false, so the client gets a 500 instead of a 404. errx's own `Join` and `JoinWrap` lose the kind too (tested on errx v0.5.5). | The "don't Join on classified paths" rule (ERR-10) |
| `Wrapf` and `WrapKindf` change the error they wrap. A shared errx error's message is rewritten on every call (tested on errx v0.5.5). | Half of the `Wrapf` ban (ERR-2) |
