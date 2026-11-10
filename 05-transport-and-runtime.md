# 05 · HTTP, background work, context, concurrency, logging and telemetry

This covers how requests and jobs enter the service, what travels in a `context.Context`, who owns goroutines, and how the service logs and, when it needs them, traces and measures itself.

It builds on these documents:

- [01](01-project-layout.md): `Run(ctx) error` components, and the rule that transports make one call, to a service or a repository.
- [03](03-domain-and-abstractions.md): one `Service` per capability, with `XArgs` and `XResult`.
- [04](04-errors.md): `httperr.Map`, the per-runtime outcome mapping, and logging only at the roots.

"Parked" marks a decision that is still open; see [decisions.md](decisions.md).

## Rules at a glance

| ID | Rule | Enforced by |
|---|---|---|
| HTTP-1 | Handlers follow gofi's [GUIDELINES.md](https://github.com/michaelolof/gofi/blob/v0.10.4/GUIDELINES.md) (v0.10.4): an exported `<Operation>Handler` variable, its schema directly below, dependencies through `deps.MustGet(c)`. | arch test, review |
| HTTP-2 | One operation per file. The package is named after its OpenAPI tag. | arch test |
| HTTP-3 | The body follows gofi HND-5. "Do the work" is one call: a service method if the capability has a justified service, otherwise a repository method. No business rules in a handler. | depguard, review |
| HTTP-4 | Schemas use the shared envelopes, and every field has `json`, `validate`, `description` and `example` tags. | OpenAPI contract test |
| HTTP-5 | `Info` has one kebab-case tag and a `Description`. No `OperationId`: the method and path identify the route. Lists use `limit` and `cursor` and return `next_cursor`. | OpenAPI contract test |
| HTTP-6 | One `routes` package. Global middleware runs in a fixed order; authentication and authorization are mounted per route group. | server test, review |
| HTTP-7 | `gofi.OpenAPISpec` produces both the live docs and the committed spec, with no post-processing. CI checks for drift. | `task generate:check` |
| JOB-1 | A job handler is named for one action. `Handle` makes one call (a service method, or a repository method when there is no service) and returns its error. | arch test, review |
| JOB-2 | The runtime adapter owns outcomes: done, retry, drop, dead-letter, defer. One mapping per runtime turns kinds into outcomes; handlers never pick one. | registration test, review |
| JOB-3 | No lost work: record the intent in the same atomic write as the state change, publish after commit as a hint, and sweep for anything recorded but not done. Handlers re-read state and do nothing if the work is already done. | review |
| JOB-4 | Periodic work is claimed before it runs, and scans only enqueue. Stream consumers commit or ack only after a durable outcome, and dead-letter with a replay path. | review |
| CTX-1 | A ctx carries cancellation, deadlines, the trace, log attributes and adapter-private handles. Nothing else. | containedctx, review |
| CTX-2 | Context keys are unexported `struct{}` types, accessed through `WithX` and `XFrom`. Never mutate a ctx. | arch test |
| CTX-3 | Handlers use gofi's request context, `c.Context()`. Middleware replaces it only through `gofi.SetRequestContext`. Every request gets a deadline from `gofimw.ContextTimeout`. | forbidigo |
| CTX-4 | `context.Background` and `TODO` appear only in `main`, `run` and tests. Detached work uses `context.WithoutCancel` plus a timeout. | forbidigo |
| CON-1 | Every goroutine has an owner that waits for it, using `errgroup` with `SetLimit`. | arch test (`go` allowlist) |
| CON-2 | Each long-lived loop is its own `Run(ctx)` component under `supervise.Run`. No `time.Sleep`. Work requested by a request or by core is a durable job, never a `go` statement. | forbidigo, arch test |
| CON-3 | Both ends of a pipe run in one errgroup, and each side closes the pipe with its error. | review |
| LOG-1 | One process-wide logger, called as `logger.Info(stamp, ctx, event, key, value, …)`. Nothing injects a logger; nothing else calls `slog`. | forbidigo, the compiler |
| LOG-2 | Keys are snake_case string literals. An error goes under `error` as the error value. | review |
| LOG-3 | The event name is a constant, dotted name in the past tense. The stamp says where; the event name says what. | the compiler, review |
| LOG-4 | Levels have fixed meanings. Secrets are redacted at the type level and by the handler. | review, `ReplaceAttr` |
| OBS-1 | Telemetry is set up once. Instrumentation lives at the edges: HTTP middleware, the job runtime, and contrib clients in adapters. | depguard |
| OBS-2 | Metric names are OTel-style and dotted, with UCUM units and bounded attributes. | analyzer |
| OBS-3 | A capability with metrics declares its own typed `Metrics` struct, built from an injected `metric.Meter`. | review |

---

## HTTP (gofi)

gofi ships its own style guide, [GUIDELINES.md](https://github.com/michaelolof/gofi/blob/v0.10.4/GUIDELINES.md), with rule IDs such as HND-5. **Handlers follow it.** This section adds only what gofi leaves to the service: how errors are mapped, when a handler calls a service rather than a repository, and the middleware this standard uses.

This standard needs gofi v0.10.4 or later. It changes behaviour that handler code relies on; see [Upgrading to gofi v0.10.4](#upgrading-to-gofi-v0104).

### HTTP-1 · Handlers follow gofi's guidelines

**Rule.**
- Each handler is an exported package-level variable named `<Operation>Handler`, defined with `gofi.DefineHandler` (gofi HND-1, HND-2).
- Its fields are in the order `Info`, `Schema`, `Meta`, `Handler`, with a blank line between each (HND-3).
- Its schema is an unexported `<operation>Schema` in the same file, directly below the handler (HND-4).
- Dependencies come from `deps.MustGet(c)`. `Deps` is an interface of accessors, registered once on the router's global store at startup (gofi DEP-1 to DEP-5, and LAY-3 here).

**Why.** gofi documents one style, and its own examples, tests and agent guidance use it. Writing handlers another way would put this standard at odds with the library it is built on. A missing dependency is a wiring bug whichever style is used, so the accessor panicking is not a reason to avoid it. `wire.go` should build `Deps` completely at startup, so the panic can't happen in practice.

### HTTP-2 · Files

**Rule.**
- One operation per file, named for the operation: `cancel.go`, `list.go` (gofi LAY-2).
- One package per resource, named after the resource's OpenAPI tag: tag `api-keys` → package `apikeys` (gofi LAY-3).
- Helpers shared by one package's handlers go in `support.go` (gofi LAY-4).

Architecture tests in `internal/architecture/boundaries_test.go` check two of these:
- `TestHTTPHandlerFilesContainOneHandler` fails if a file under `internal/http` declares more than one gofi handler.
- `TestHTTPHandlerDeclarationPrecedesSchemas` fails if, in a handler file, a type (the schema) is declared before the handler variable.

Every project has both tests.

### HTTP-3 · The handler body

**Rule.** The body follows gofi HND-5, in this order:

1. `gofi.ValidateAndBind[<schema>](c)` is the first statement. Return its error through `httperr.Map(stamp, err)`, because it isn't always a bad request: a wrong content type is a 415 (ERR-7).
2. Get dependencies with `deps.MustGet(c)`, and anything middleware stored, such as the principal.
3. Do the work, as **one call**:
   - a method on the capability's service, if the capability has one;
   - otherwise, a repository method.
4. Map any error with `httperr.Map(stamp, err)` (ERR-7).
5. Fill `s.<Status>.Body`, then `return c.Send(<code>, s.<Status>)` with the code as an integer literal (gofi RES-1 to RES-3).

Separate the steps with blank lines (STY-9). A handler contains no business rules, no transactions and no `go` statements.

**When a capability has a service.** A service is not created by default. It must pass the abstraction test in [03](03-domain-and-abstractions.md#the-test-every-abstraction-must-pass). In practice that means it does at least one of these:
- it coordinates several repositories or adapters;
- it holds rules that more than one transport uses, for example HTTP and a worker;
- it hides an external system.

A new service also needs the owner's approval. Don't add a new service or service-layer abstraction unless the owner has explicitly approved that new service. Without a service, the handler calls the repository directly, as gofi's own examples do.

**Don't.** The handler runs a use case itself: the clock, version allocation, building the access record, a repository write, and mapping kinds by hand. Either one use-case-shaped repository method does this (DATA-4), or it is a service's job.

```go
d := deps.MustGet(c)
now := time.Now().UTC()
controlVersion, err := d.ControlVersions().AllocateNextVersion(c.Context())
// ...
result, err := d.ProducerKeys().Revoke(c.Context(), producerkeys.RevokeArgs{
// ...
	if errx.IsKind(err, errkinds.ErrKindNotFound) {
		return httperr.New(1793625668231, httperr.NotFoundErr, "resource not found")
	}
```

**Canonical** (a capability with a service):

```go
// internal/http/api/transfers/cancel.go

// CancelTransferHandler cancels a transfer that has not reached the provider.
var CancelTransferHandler = gofi.DefineHandler(gofi.RouteOptions{
	Info: gofi.Info{
		Tags:        []string{"transfers"},
		Description: "Cancels a transfer that has not reached the provider. Cancelling a cancelled transfer returns it unchanged.",
	},

	Schema: &cancelTransferSchema{},

	Handler: func(c gofi.Context) error {
		s, err := gofi.ValidateAndBind[cancelTransferSchema](c)
		if err != nil {
			return httperr.Map(1793346400056, err)
		}

		d := deps.MustGet(c)
		caller := authn.MustPrincipal(c)

		cancelled, err := d.Transfers().Cancel(c.Context(), transfer.CancelArgs{
			TenantID:   caller.TenantID,
			ActorID:    caller.ActorID,
			TransferID: s.Request.Path.TransferID,
		})
		if err != nil {
			return httperr.Map(1793152110864, err)
		}

		s.Ok.Body.Data = cancelled

		return c.Send(200, s.Ok)
	},
})

type cancelTransferSchema struct {
	Request struct {
		Header schemas.IdempotencyHeader
		Path   struct {
			TransferID domain.TransferID `json:"transfer_id" validate:"required" spec:"transfer-id" description:"Transfer ID." example:"01HV2QZYZ3K8M4N5P6Q7R8S9T0"`
		}
	}
	Ok  schemas.SuccessSchema[domain.Transfer]
	Err schemas.ErrSchema
}
```

Without a service, step 3 is a repository call, for example `d.Transfers().Get(c.Context(), caller.TenantID, s.Request.Path.TransferID)`. Everything else is the same.

### HTTP-4 · Schemas

**Rule.**
- Use the shared generic envelopes `schemas.SuccessSchema[T]` and `schemas.ErrSchema` (gofi SCH-1, SCH-2).
- Every field carries a snake_case `json` tag, plus `validate`, `description` and `example`.
- Defaults go in the `default:` tag, not in handler code.
- List fields always encode as `[]`, never `null`. The envelope's constructor turns a nil slice into an empty one, so no handler has to remember.
- Request and response bodies use domain types directly (DOM-4): `SuccessSchema[domain.Transfer]`. The domain type's `json`, `validate`, `description` and `example` tags are the API contract, so a field that isn't part of it is `json:"-"`.
- A separate response struct exists only when the response's shape differs from the entity: fields combined from several entities, or computed. Its fields use domain types, so mapping copies fields and converts nothing.
- ID fields carry the ID type's `spec` tag (DOM-1).

**Why.** Without this rule, one codebase documented 522 of its 956 schema fields, and another 22 of 858, with no examples. SDKs generated from the spec have the same gaps.

### HTTP-5 · Route info and lists

**Rule.**
- `Info` has exactly one tag, the lowercase kebab-case tag the package is named after, and a `Description` written for API consumers (gofi DOC-1, DOC-2).
- Don't set `OperationId`. The method and path identify the route in the docs. gofi never generates an `operationId`, so documentation tools fall back to the method and path.
- `Summary` is optional.
- List endpoints:
  - take `limit` (`default:"50" validate:"omitempty,min=1,max=100"`) and an opaque `cursor`;
  - return `next_cursor`, omitted when empty;
  - never use offset paging, and never claim pagination that isn't implemented, such as `NewPaginated(items, "", n)`, which always sends an empty cursor.

### HTTP-6 · Routes and middleware

**Rule.**
- One `routes` package registers everything before the server starts. gofi seals the router when it starts serving (gofi RTE-4).
- When routes under one prefix need different middleware (read scope and write scope), write one `r.Route` block per middleware stack, with the same prefix (gofi RTE-3).
- Check the error from every middleware constructor that returns one, and fail startup if it is set (gofi MDW-9).
- Versioning uses the major version in the path, and nothing else. Admin APIs are separate binaries.

The middleware order is fixed. It extends gofi MDW-8:

```text
global:    Recover → RequestID → SecurityHeaders/CORS → AccessLog → ContextTimeout
per group: RateLimit (pre-auth, by IP) → Authenticate → Authorize
```

A project that adds tracing (OBS-1) puts its middleware right after RequestID.

- **RequestID** generates a new ULID for every request and ignores any `X-Request-ID` the client sends. It returns the ID in the `X-Request-ID` response header, so a client can quote it to support, and adds `request_id` to the context's log attributes.
  - A reference the client chooses, such as an `Idempotency-Key` or the caller's own order ID, isn't a request ID. The endpoint declares it in its schema under its own name, validates it, and gives it its own log key if it needs one.
- **ContextTimeout** is `gofimw.ContextTimeout(cfg.RequestTimeout)` (CTX-3).
- **Authenticate** puts the principal on the request context, where the handler, the access log and the error handler get it from. The handler passes tenant and actor to core in `XArgs` (CTX-1). Since gofi v0.10.4, values set after `ContextTimeout` stay visible to both, without the timeout's deadline (tested).

**Why.**
- Without one order, each server drifts, and the same bug has to be found in each.
- RequestID added per route group leaves operational routes with none.
- `request_id` exists to find every log line for one request. A client-chosen ID can't promise that. A caller can resend another request's ID, by mistake or on purpose, and one search for that ID then returns both requests. Checking the format doesn't help, because a reused ID is still valid.
- A client's reference means something to the application, such as "don't charge twice". Sending it as `X-Request-ID` hides it in a transport header that every route accepts and no schema documents.

**Don't.** Keep the client's ID when it looks valid:

```go
func requestID(c gofi.Context) string {
	if rid := string(c.HeaderBytes("x-request-id")); validRequestID.MatchString(rid) {
		return rid
	}

	return ulid.Make().String()
}
```

**Do.**

```go
// internal/http/middleware/request_id.go
// RequestID gives every request a new ID. It ignores any X-Request-ID the client sends.
func RequestID(c gofi.Context) error {
	rid := ulid.Make().String()
	c.Writer().Header().Set("X-Request-ID", rid)

	if err := gofi.SetRequestContext(c, logger.With(c.Context(), "request_id", rid)); err != nil {
		return errx.Wrap(1794328333309, err)
	}

	return c.Next()
}
```

The routes it runs in:

```go
// internal/http/routes/routes.go
r.UseErrorHandler(httperr.Encoder(authn.CallerSubject))
r.Use(recoverMW, requestIDMW, securityMW, accessLogMW, timeoutMW)

r.Route("/api/v1/transfers", func(r gofi.Router) {
	r.Use(authn.APIKey, authz.Require(domain.ScopeTransfersRead))
	r.Get("/:transfer_id", transfers.GetTransferHandler)
})

r.Route("/api/v1/transfers", func(r gofi.Router) {
	r.Use(authn.APIKey, authz.Require(domain.ScopeTransfersWrite))
	r.Post("/:transfer_id/cancel", transfers.CancelTransferHandler)
})
```

### HTTP-7 · OpenAPI

**Rule.**
- `gofi.OpenAPISpec(router, docsOptions)` is the only spec builder. The live `/api-docs` route and `cmd/openapi` both use it (gofi DOC-4).
- Don't post-process the generated document (gofi DOC-5). Security schemes are declared through `DocsOptions.SecuritySchemes`, `DocsOptions.Security` and `Info.Security`.
- The spec is committed as `docs/openapi/<binary>.json`.
- `task generate:check` fails the build if the spec has drifted from the code.

**Why.** Code that post-processed the document patched gofi's output in three ways: `:param` to `{param}`, `*/*` to `application/json`, and bearer security schemes. gofi has done all three itself since v0.10.0, so no patch is needed.

### Upgrading to gofi v0.10.4

A project on an older gofi must expect these changes when it moves to v0.10.4. They're from v0.10.0 unless marked, and they affect handler code (checked against the source, and tested where marked):

1. **`c.Context()` is a per-request `context.Context`**, no longer the fasthttp `RequestCtx`. Values set with `SetUserValue` on the fasthttp context are no longer visible through it (tested). Helpers that rely on that, such as a `SetOwnership` that stores its value this way, stop working silently. They must move to `gofi.SetRequestContext`.
2. **The request context is cancelled when the handler returns** (tested). A goroutine that captured `c.Context()` now sees `context canceled`.
3. **`c.GlobalStore()` is read-only during requests** (tested). Write it only through `r.GlobalStore()` at startup.
4. **Middleware constructors return `(MiddlewareFunc, error)` and take one config struct:** `CORS`, `CSRF`, `Cache`, `Timeout`. Existing `CORS` calls no longer compile. `CORS` also returns an error when `AllowOrigins` is empty, so a tool that builds a server with no config, such as an OpenAPI generator, fails at that call (tested).
5. **`TimeoutConfig` requires `MaxInFlight`**, and the timeout answers 503 instead of 408. Prefer `ContextTimeout`.
6. **The default error handler no longer sends raw error text** for 5xx responses (tested), and validation errors become 400 instead of 500.
7. **Registering a route after the server starts panics** (tested).
8. **Unknown `validate` rules** (`dive`, `required_if`) panic at registration, according to gofi's TAG-6. Not tested.
9. **The OpenAPI output changes** (`{param}` paths, `application/json` bodies, security fields), so committed specs will drift once.
10. **A `spec` tag that names an unregistered spec panics when the route is registered** (tested). Call `RegisterSpec` before registering routes.
11. **What `Decode` receives in a JSON body depends on the field's Go kind** (tested). A string field gets a `string`, and any other JSON type is a 400 before `Decode` runs. A number or bool field gets that kind, such as `int64`, so large integers stay exact; if the value doesn't fit, it gets the value as sent. Any other field, such as a `[16]byte` ID, gets the value as sent: a `string`, `float64` or `bool`, or the raw JSON bytes for an object or array. A spec that decodes from a string needs no change.
12. **Pointer path, query and header params start binding** (since v0.10.0, tested). On v0.7.18 to v0.8.11 they silently stay `nil`. Optional filters declared this way start working on upgrade, so clients that send them get narrower results than before.
13. **A failed type assertion on the input inside `Decode` is a 400** (tested). Any other failed assertion, and any other panic in `Decode`, is a logged 500. In a spec, check the type with `v, ok :=` and return your own error.
14. **Schema mistakes panic at registration** (tested): a path, query or header field that can't be read from text and has no spec, a `spec` tag or an unsupported type on a cookie field, a non-numeric `oneof` option on a number field, and a spec `default` that doesn't decode. A named param type that's string-based, and a `gocql.UUID` param with its spec tag, are fine.
15. **A value that can't be stored in a path, query or header field is a 500, and the handler doesn't run** (tested). Earlier versions logged it and ran the handler with an empty field.
16. **`Encode` receives the value a pointer points to, and a nil pointer is `null`** (tested). A spec that expects a pointer such as `*WalletScope` in `Encode` must change if it's ever used in a response.
17. **A `[N]byte` field with no spec is base64 in requests and responses** (tested). Before, valid base64 crashed the request and responses sent an array of numbers.

18. **`UnmarshalJSON` runs on request bodies** (v0.10.4, tested). A service with many of these methods has the most to check. A value one rejects becomes a 400 from `ValidateAndBind` before the handler runs, and the method's error text reaches the client. For example, `KnownAsset.UnmarshalJSON` now fails before the handler's `NormalizeInput` call gets to map the error. Read those methods' error text, and check the error codes clients get.
19. **JSON bodies are stricter** (v0.10.4, tested). `{"v":1}` sent for a struct field is a 400, where it used to bind an empty struct. `[null,"x"]` on `[]string` is a 400. On `[]*string` it binds two elements, the first nil.
20. **Nested JSON binds** (v0.10.4, tested). `{"v":{"a":{"b":1}}}` now works on `any`, `map[string]any` and `[]any`. A nested `metadata` reaches the handler, so the handler's own limits on it now matter.
21. **More bad input is a 400** (v0.10.4, tested): a missing `required` query or header param with a struct spec, an invalid cookie value, and a file sent to a field that isn't a file.
22. **Values that were dropped now arrive** (v0.10.4, tested): a `time.Time` in a form or multipart body, and `default` on JSON pointers, times and cookies. A handler that relied on the empty value now sees the real one.
23. **Responses change** (v0.10.4, tested). Unexported fields are no longer written. A pointer response header writes its value, not a memory address. A nil pointer body is `null`, not a 500. A spec with `Type: "integer"` writes a number, not a quoted string.
24. **More schema mistakes panic at registration** (v0.10.4, tested): the `,string` JSON option, a recursive type, a map key that isn't a string, a `default` that doesn't parse, a response header that isn't text, and an embedded pointer in a `Path`, `Query`, `Header` or `Cookie` block. Running the project's OpenAPI tool registers every route, so it finds these without booting the service, once the `CORS` calls in item 4 are changed (tested).
25. **OpenAPI marks pointers `nullable`, and `float64` is `format: double`** (v0.10.4, tested), so specs and generated clients change again.
26. **A `Config.BodyLimit` above `ServerOptions.MaxRequestBodySize` makes `Listen` return an error** (v0.10.4, tested). It used to lower the limit silently.
27. **Every server sets limits** (SEC-17, gofi SRV-5), with `r.Configure`.

Use v0.10.4 or later, not an earlier v0.10.x. The earlier ones have binding and server bugs it fixes, and a spec like the `*WalletScope` one in item 16 breaks on v0.10.0.

## Background work

A job runs on one of these runtimes:

| Runtime | What it gives you | Delivery |
|---|---|---|
| Database-backed queue | a queue inside the database, such as CockroachDB, with periodic jobs | at least once |
| asynq on Redis | jobs, which relays can enqueue from a MongoDB outbox | at least once |
| Redis streams | consumer groups, with ZSET schedulers and recovery sweeps over a database such as ScyllaDB | at least once |
| Kafka (franz-go) | an event stream | at least once |

The rules below apply to all of them. Each rule shows how it maps onto each runtime.

### JOB-1 · A job handler does one named action

**Rule.**
- **Name the handler and the job kind for the action:** `transferjobs.Initiate` with kind `transfer.initiate`, `erasure.EraseActor`, `exports.BuildCSV`, `delivery.SendWebhook`. Not `Execute`, `Process`, `Worker` or `Processor`, which don't say what is done.
- **`Handle(ctx, args) error` makes one call** and returns its error, wrapped with a stamp:
  - a method on the capability's service, when it has a justified one (HTTP-3);
  - otherwise a core function or a use-case-shaped repository method (DATA-4).
- **A handler doesn't choose its outcome.** It returns the error; the runtime adapter decides retry, drop or dead-letter (JOB-2). So there is no per-handler `classify`.
- **No business branching in a handler.** If a worker reads state, decides, and calls a provider, that logic is a use case and belongs in core.
- **Job args carry IDs only:** snake_case `json` tags, never whole entities, never secrets.

**Why.**
- Names like `Handle`, `Process`, `ProcessMessage`, `ProcessProbeJob`, `RunOnce`, `Run`, `Execute` and `Dispatch` don't say what the job does. A job called `ExecuteVendorTransfer` loads an INITIATED transfer, starts dispatch, calls the provider and binds the provider reference. In other words, it *initiates the transfer at the provider*, so `Initiate` is the name.
- Of 24 handlers in one codebase, 5 made one service call, 11 were scans, and the rest put repository calls, provider calls and domain branching inside the worker.

**Do, without a service.** One bounded unit of work, calling repositories directly:

```go
entry, err := p.Ledger.Record(ctx, erasureledger.RecordArgs{TenantID: payload.TenantID, ActorID: payload.ActorID /* ... */})
if err != nil {
	return errx.Wrap(1793624332703, err)
}
if err := p.WrappedDEKs.MarkErased(ctx, wrappeddeks.MarkErasedArgs{ /* ... */ ErasureLedgerSeq: entry.Seq}); err != nil {
	return errx.Wrap(1793624332927, err)
}
return nil
```

Two things to change in it:
- The two writes should be one use-case-shaped repository method, `EraseActor`, so they can't half-happen (DATA-4).
- The same handler drops a payload that doesn't decode, with a bare `return nil` and no log. It should be dead-lettered (JOB-2).

**Don't.** The worker reads the transfer, applies the terminal-state rule, looks up the provider, calls it, and decides to cancel. That is a use case running inside a transport.

**Canonical**, with a service:

```go
// internal/workers/transferjobs/initiate.go

// Initiate submits an accepted transfer to its custody provider.
type Initiate struct{ transfers *transfer.Service }

func (h *Initiate) Handle(ctx context.Context, args jobs.TransferInitiate) error {
	_, err := h.transfers.Initiate(ctx, transfer.InitiateArgs{TenantID: args.TenantID, TransferID: args.TransferID, OperationID: args.OperationID})
	if err != nil {
		return errx.Wrap(1791237019732, err)
	}

	return nil
}
```

**Canonical**, without a service (the erasure job above, after the fix):

```go
// internal/workers/erasure/erase_actor.go

// EraseActor records an actor's erasure and marks their data keys erased, in one write.
type EraseActor struct{ erasures *erasures.Repo }

func (h *EraseActor) Handle(ctx context.Context, args jobs.EraseActor) error {
	if err := h.erasures.EraseActor(ctx, args.TenantID, args.ActorID); err != nil {
		return errx.Wrap(1791370738426, err)
	}

	return nil
}
```

The handler shape is the same on every runtime. Only registration differs: `queue.Register` on a database-backed queue, an asynq `ServeMux` entry, or a stream consumer's dispatch table.

### JOB-2 · The runtime owns outcomes

**Rule.** Every job ends in one of five outcomes:

| Outcome | When | Database-backed queue | asynq | Redis streams | Kafka |
|---|---|---|---|---|---|
| **Done** | `nil`, including "already applied" | complete | return `nil` | `XACK` | commit |
| **Retry** | a transient failure; the default for an unknown error | return the error (backoff) | return the error | leave pending; `XAUTOCLAIM` redelivers | don't commit; seek back or pause |
| **Drop** | the target is gone, so there's nothing to do | log, return `nil` | log, return `nil` | log, `XACK` | log, commit |
| **Dead-letter** | poison (undecodable, `invalid_input`), or attempts exhausted | `queue.Cancel` plus a DLQ row | `asynq.SkipRetry` (archived, replayable) | `XADD <stream>.dlq`, then `XACK` | write a DLQ record, then commit |
| **Defer** | a known later time; not counted as an attempt | `queue.Snooze(d)` | re-enqueue with `ProcessIn`, return `nil` | `ZADD` to the schedule, then `XACK` | n/a |

- **Core returns errx kinds. Each runtime adapter has one mapping** from kind to native outcome, used by every handler on that runtime. This replaces per-handler `classify` functions.
- **Every job kind has bounded attempts.** After the last one, the job is dead-lettered, never dropped silently. Redis streams have no built-in limit, so the consumer reads the delivery count from `XPENDING` and dead-letters past N.
- **A dead-letter record** holds the original payload and headers, the error kind, the attempt count and the source coordinates. Each runtime has a replay tool in `cmd/tools`.
- **The runtime also owns:** a per-job timeout, panic recovery, backoff with jitter, the single failure log (ERR-9), and the consumer span and metrics if the project has telemetry. Handlers never sleep, retry or log their own failure.
- **A business retry schedule** (an attempt count and `next_attempt_at` on the row) is fine when the schedule is business data: the KYC re-query SLA, the webhook retry policy. The job then returns `nil` and a sweep drives the next attempt. Each job kind uses one scheme or the other, never both.

**Why.** Without one mapping per runtime:
- Handlers decide for themselves. Only 3 of 24 handlers ever returned `Cancel`, each with its own kind check, and `Snooze` was never used. A publish worker returned `nil` even when it couldn't record the failure.
- A `PermanentWrap` helper had no callers, and job handlers dropped a malformed payload with a bare `return nil`. An email relay detected its own `Permanent` type by type assertion, so a wrapped one was retried forever. Three relays used three different retry policies.
- A consumer can have the right order (below) and still not cap redeliveries. If the delivery count is read only for metrics, a message that keeps failing on infrastructure is reclaimed forever (inferred from the code; not tested).

**Do.** Dead-letter before acknowledging, and leave the message pending on error:

```go
updated, exhausted, err := a.receiptProcessor.Process(ctx, msg.Job)
if err != nil {
	logger.Error(1792317746208, ctx, "receipt.processing.failed", "receipt_id", msg.Job.ReceiptID, "error", err)
	return // not acked: XAUTOCLAIM redelivers after minIdle
}
if exhausted && a.receiptDLQQueue != nil {
	if dlqErr := a.receiptDLQQueue.Publish(ctx, updated); dlqErr != nil {
		return
	}
}
if ackErr := a.receiptQueue.Ack(ctx, a.receiptGroup, msg.MessageID); ackErr != nil { /* log */ }
```

**Canonical** (the one mapping for a database-backed queue):

```go
// internal/adapters/dbqueue/outcome.go

// outcome turns a handler's error into a queue outcome. It is the only place
// that decides retry, drop or dead-letter for jobs on this runtime.
func outcome(err error) error {
	switch {
	case err == nil:
		return nil
	case errx.IsKind(err, errkinds.ErrKindNotFound):
		return nil // drop: the runtime logs it before calling outcome
	case errx.IsKind(err, errkinds.ErrKindInvalidInput):
		return queue.Cancel(err) // dead-letter
	case errx.IsKind(err, errkinds.ErrKindRateLimited):
		return queue.Snooze(30 * time.Second) // defer
	default:
		return err // retry with backoff
	}
}
```

The asynq, Redis-stream and Kafka adapters each have the same function, returning their own native outcome.

### JOB-3 · No lost work

**Rule.**
1. **Record the intent in the same atomic write as the state change** (DATA-4). Use whatever the store offers:
   - **CockroachDB:** the same statement or transaction. That means `EnqueueTx` when the queue lives in the same database, otherwise an outbox row.
   - **MongoDB:** the same document (an embedded `dispatch` sub-document), or a session transaction inside the repository.
   - **ScyllaDB:** an LWT batch within one partition.
2. **Publishing after the commit is a hint, not the record.** If the publish fails, log it and move on.
3. **A sweep re-drives every intent that is recorded but not done.** It claims its work with a lease, LWT or atomic script, so replicas don't collide. The sweep is mandatory: without it, a crash between the commit and the publish loses the work.
4. **Every enqueue or publish carries a dedupe key** built from business identity, plus a generation where work can be re-issued. This limits copies.
5. **Every handler re-reads state and does nothing if the work is already applied.** Every runtime here delivers at least once, so this, not the dedupe key, is what makes a job correct. External side effects are keyed by a stable reference, such as the provider reference or the ledger sequence.

A registration test asserts that every job kind has bounded attempts and a dedupe mode.

**This rule isn't specific to a database-backed queue.** Only the name `EnqueueTx` is. It looks like this on each store and runtime:

| Store and runtime | Atomic record | Hint | Sweep | If the process crashes after the commit |
|---|---|---|---|---|
| CockroachDB, database-backed queue | operation row in the transfer's transaction | best-effort `Enqueue` after commit | periodic `ListDue` scan | the next scan enqueues it |
| MongoDB, asynq | job document with an embedded `dispatch` sub-document, one `InsertOne` | the relay enqueues with `asynq.TaskID` = document ID | the relay claims undispatched rows by lease | the lease expires and the relay re-enqueues the same task ID |
| ScyllaDB, Redis streams | root, job record and archive obligation in one LWT batch | best-effort `Publish` to the Redis stream | a recovery loop over 256 shards, each claimed by LWT | the sweep republishes Ready and Scheduled jobs |

**Do.** Commit first; the publish is only a wake-up:

```go
// Transport failure after commit is recoverable from the registry.
if !accepted.Duplicate {
	if err := s.eventQueue.Publish(ctx, accepted.Event.WithAttempt(1)); err != nil {
		logger.Warn(1791443338883, ctx, "event.publish.deferred", "event_id", accepted.Event.ID, "error", err)
	}
}
```

**Don't.** The export handler inserts the job, then `Dispatch` runs it in a goroutine rooted in `context.Background()`. No sweep looks for exports left in `created` or `processing`, so a crash or deploy leaves the export stuck forever:

```go
go func() {
	defer func() { <-d.pool }()
	d.run(context.Background(), job, userEmail)
}()
```

Fixed, the handler's insert also records the intent (rule 1), and `exports.BuildCSV` runs as a job (JOB-1).

**Also watch for:**
- A relay that never handles asynq's `ErrTaskIDConflict`: a task that is still queued blocks its row until the task finishes (inferred).
- A task type that is registered, but that no production code enqueues.

### JOB-4 · Periodic work and stream consumers

**Periodic work.**
- It takes one of two forms:
  - **(a) A periodic job.** One leased leader enqueues it into the durable queue, and any worker runs it. A database-backed queue's periodic job does this, with a `scheduler:<queue>` lease, a durable `next_fire`, and coalescing of missed runs. For asynq, this is `asynq.PeriodicTaskManager`.
  - **(b) A `Run(ctx)` component in every replica that claims each unit of work before acting:** a row lease, an LWT, or an atomic Lua script. A per-shard LWT claim works this way.
- **A scan only finds due rows and enqueues one job per row, with a dedupe key.** It never does the item's work inline.

**Don't.** Three ticker relays are started with `go func` from `main` in every replica, on `context.Background()`, outside any lifecycle. Per-row leases keep them correct, but nothing stops them at shutdown or notices if they die. They become components (CON-2).

**Stream consumers** run one loop inside `Run(ctx) error`:
1. Fetch a bounded batch.
2. Process each partition or key in order, with concurrency bounded by `errgroup.SetLimit`.
3. Turn each record into an outcome (JOB-2).
4. For done, drop and dead-letter: write the durable outcome first, *then* commit or ack.
5. For retry: don't commit.
   - Kafka: seek back to the failed offset, or pause the partition and back off.
   - Redis: leave the message pending. `XAUTOCLAIM` redelivers it, and it is dead-lettered past N deliveries.

**Do:**
- Records are processed per partition, in order.
- Each outcome is written durably: persisted, duplicate, rejected or quarantined.
- Coverage is advanced.
- Only contiguous offsets are committed.
- Poison records go to a quarantine collection with the reason and the topic, partition and offset.

**Don't:**
- On a transient error the code "stops this partition" and expects the next poll to re-read the record. But nothing seeks back, and franz-go doesn't redeliver records it already returned, so the record isn't retried until a restart or rebalance (inferred from franz-go's behaviour; not tested).
- Quarantined records have no replay tool, even ones that are always safe to retry, such as `kms_unavailable`.
- An undecodable message is written to `<stream>:quarantine` but not acked. It stays pending and is quarantined again on every reclaim (inferred).

## Context

### CTX-1 · What a ctx may carry

**Rule.** A ctx carries only:
- cancellation and deadlines;
- the OpenTelemetry span and baggage, if the project has telemetry;
- log attributes (LOG-1);
- adapter-private handles, such as an open database transaction.

Tenant, actor and principal are business inputs to core, so they go in `XArgs` fields. HTTP middleware may keep the principal on the request context for handlers to read (CTX-3), but core never reads it from there. Loggers, dependencies, config and optional parameters never go in a ctx. `ctx` is the first parameter, except that a logger call's stamp comes before it (LOG-1). It's never stored in a struct field.

**Don't.** The ctx is mutated in place, and a business input is hidden in it:

```go
func SetOwnership(ctx context.Context, ownership OwnershipContext) context.Context {
	if setter, ok := ctx.(userValueSetter); ok {
		setter.SetUserValue(ownershipKey{}, ownership)
		return ctx
	}
	return context.WithValue(ctx, ownershipKey{}, ownership)
}
```

### CTX-2 · Keys and accessors

**Rule.**
- Keys are unexported `type xKey struct{}`.
- The package that owns the value type exports `WithX(ctx, v) context.Context` and `XFrom(ctx) (T, bool)`.
- `context.WithValue` appears only in a file named `context.go`.

### CTX-3 · The request context and deadlines

Since gofi v0.10.0, `c.Context()` is a per-request context. These were tested on a running server:
- it is cancelled when the handler finishes;
- it is cancelled when the server shuts down;
- it is **not** cancelled when the client disconnects.

gofi's GUIDELINES.md (CTX-1) and README have said the same since v0.10.3.

**Rule.**
- Handlers and middleware pass `c.Context()` to core.
- Middleware that adds a value or a deadline derives a new context from `c.Context()` and installs it with `gofi.SetRequestContext(c, ctx)`, checking the error (gofi CTX-5, MDW-11). There is no `httpctx` bridge package.
- Every request gets a deadline: `gofimw.ContextTimeout(cfg.RequestTimeout)` in the global chain. The default is 30 s, kept below `SHUTDOWN_TIMEOUT`. Outbound calls set tighter timeouts of their own.
- **Where request values go:**
  - values that travel with the request (the principal, log attributes) go on the request context, through `WithX` (CTX-2) and `SetRequestContext`;
  - values only HTTP code reads go in `c.DataStore()` under unexported struct keys, using `gofi.StoreSet` and `gofi.StoreGet` (gofi MDW-4). gofi's string-key `Set`/`Get` methods are deprecated and banned.
- Never write `GlobalStore` after startup (gofi DEP-5).
- A handler starts no goroutines (CON-2). If detached work is unavoidable, take `context.WithoutCancel(c.Context())` before the handler returns (gofi CTX-7). `c.Copy().Context()` is `context.Background()` and carries no values.

**Why.**
- One codebase kept the request span ctx in the DataStore. It read the ctx back 20 times, against 198 calls to `c.Context()`, so most work ran outside the request's trace. With `SetRequestContext`, `c.Context()` is always the context with the span.
- An API that roots every request in `context.Background()` loses its log attributes and its deadline.

```go
// internal/http/middleware/principal.go
func withPrincipal(c gofi.Context, p domain.Principal) error {
	ctx := domain.WithPrincipal(c.Context(), p)
	ctx = logger.With(ctx, "tenant_id", p.TenantID, "actor_id", p.ActorID)

	return gofi.SetRequestContext(c, ctx)
}
```

### CTX-4 · No new root contexts

**Rule.**
- `context.Background()` and `context.TODO()` appear only in `main`/`run` and in tests.
- Detached work uses `context.WithoutCancel(ctx)` plus a timeout. This covers releasing a permit, a best-effort write, and finalizing a job during shutdown.

Without this rule they spread: three codebases had 18, 19 and 63 `context.Background()` calls outside `main`, and none used `context.WithoutCancel`.

**Don't.** Fire-and-forget on a fresh root, losing the trace and log attributes:

```go
go func() {
	defer func() { <-d.pool }()
	d.run(context.Background(), job, userEmail)
}()
```

**Canonical:**

```go
release := func() {
	ctx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 5*time.Second)
	defer cancel()
	_ = s.permits.Release(ctx, permit)
}
```

**Enforced by:**
- forbidigo `^context\.(Background|TODO)$`, excluding `cmd/**` and `_test.go`;
- staticcheck SA1019 on gofi's deprecated string-key store methods;
- `containedctx`, `contextcheck` and `noctx`.

---

## Concurrency

### CON-1 · Every goroutine has an owner

**Rule.**
- Use `errgroup.WithContext` when one failure should cancel the siblings. Use a plain `errgroup.Group` when it shouldn't.
- Bound concurrency with `SetLimit(n)`.
- Write results by index, so no mutex is needed.
- A raw `go` statement is allowed only in `foundation/supervise` (CON-2), which also holds the `goSafe` recover root (ERR-11), and in `pkg/` libraries such as a database-backed queue (LAY-1).

**Do.** The shape is right, though the code belongs in core:

```go
group, groupCtx := errgroup.WithContext(ctx)
group.SetLimit(deletionStateReads)
for i, productID := range productIDs {
	group.Go(func() error {
		records, err := repo.ListProjectsByProductNewest(groupCtx, productID, projectType, before, limit+1)
		// ...
		perProduct[i] = records
		return nil
	})
}
if err := group.Wait(); err != nil {
```

**Don't.** An errgroup rebuilt by hand from a mutex, a WaitGroup, a semaphore channel and `firstErr`, with no panic recovery.

### CON-2 · Loops are components, and nothing is fire-and-forget

**Rule.**
- **A process is `supervise.Run(ctx, shutdownTimeout, components...)`**, called from `run`. Each long-lived loop is its own component with `Run(ctx) error`: an HTTP server, a consumer, a reclaimer, a sweep, a relay, a scheduler tick. If one fails or stops early, the others are cancelled and the process exits, so the host restarts it rather than running half of it.
- **A component never starts other long-lived loops.** It is registered at the composition root (LAY-3), not started by another component's `Run` or by core.
- **Config is validated and defaulted in the constructor**, never lazily inside `Run`.
- **Waiting uses a ticker or timer selected against `ctx`**, through `foundation/ticker.Every(ctx, d, fn)`. No `time.Sleep` outside tests, and no `time.After` inside loops.
- **Work that a request or core wants done later is a durable job** (JOB-3), never a `go` statement. An in-memory goroutine is lost on a crash or deploy.
- **Parallelism inside one batch** is an errgroup with `SetLimit`, joined before the function returns (CON-1).

**Why.** Counts outside tests, across three codebases: 47 `go` statements, 11 of them in `main` files; 2 `time.Sleep` calls, one of them dev-only; and errgroup in only 10 files, all in one codebase.

**Do.** `foundation/supervise` starts each component, cancels the others when one fails or stops unexpectedly, and enforces the shutdown deadline. `foundation/supervise` and `foundation/ticker` aren't libraries you can import. Write them once in your project, from this description:

```go
err = supervise.Run(runCtx, shutdownTimeout,
	supervise.Component{Name: "worker", Run: worker.Run},
	supervise.Component{Name: "scheduler", Run: scheduler.Run},
	supervise.Component{Name: "archival", Run: archiver.Run},
)
```

**Don't.** `RetrySchedulerApp.Run` is itself a small supervisor. It starts five sub-loops, each with its own WaitGroup, and applies defaults lazily inside them. If one sub-loop dies, nothing notices: the scheduler keeps running with part of its work stopped. `DeliveryWorkerApp.Run` does the same with three loops, plus a reclaimer that core starts itself.

The fix is to register five components with `supervise.Run`, each built and validated in `wire.go`:

```go
supervise.Component{Name: "retry.promote", Run: retryPromoter.Run},
supervise.Component{Name: "retry.recover", Run: recovery.Run},
supervise.Component{Name: "warmcheck.schedule", Run: warmChecks.Run},
// ...
```

**Don't, fire-and-forget:**
- `go p.emitFailureAlert(...)` sends the failure-alert email from a goroutine in core. A WaitGroup tracks it, but only in memory, so a deploy loses it. It should be an `alerts.SendFailureEmail` job enqueued with the attempt's outcome.
- A relay loop says "Log error but continue", then continues without logging:

  ```go
  if err := r.tick(ctx); err != nil {
  	// Log error but continue
  	continue
  }
  ```

- A retry backoff with `time.Sleep`, which ignores `ctx`, so shutdown waits for it. Use a timer in a `select` with `ctx.Done()`.

### CON-3 · Pipes

**Rule.** The producer and consumer of an `io.Pipe` run in one errgroup. Each side calls `CloseWithError` with its own result, so neither side can block the other.

**Don't.** If `ObjectStore.Put` returns early without draining `pipeReader`, the CSV writer blocks on the pipe. `<-writeErrCh` then waits forever, because the reader is closed only after that receive.

```go
go func() {
	writeErrCh <- p.streamCSV(ctx, job, digest, &rowCount)
	_ = pipeWriter.Close()
}()
putResult, putErr := p.ObjectStore.Put(ctx, ports.PutArgs{ /* ... */ Body: pipeReader})
writeErr := <-writeErrCh
if writeErr != nil {
	_ = pipeReader.CloseWithError(writeErr)
```

**Canonical:**

```go
pr, pw := io.Pipe()
g, ctx := errgroup.WithContext(ctx)
g.Go(func() error {
	err := s.writeCSV(ctx, job, pw)
	pw.CloseWithError(err) // nil closes with io.EOF
	return err
})
var put PutResult
g.Go(func() error {
	var err error
	put, err = s.store.Put(ctx, PutArgs{Key: job.Key, Body: pr})
	pr.CloseWithError(err) // unblocks the writer if Put returns early
	return err
})
if err := g.Wait(); err != nil {
	return Result{}, errx.Wrap(1791316535662, err)
}
```

---

## Logging

### LOG-1 · One logger for the process, called through package functions

**Rule.**
- Logging goes through the `logger` package's functions. There is no logger object to build, inject or pass. Like `slog` itself, there is one per process, set up once in a command's `run`.
- Every call has the same shape: the stamp first, then `ctx`, then the event name, then key–value pairs.

  ```go
  logger.Warn(1791740215936, ctx, "job.attempt.failed", "job_kind", kind, "attempt", n, "error", err)
  ```

  | Argument | What it gives the line |
  |---|---|
  | stamp | *where* it was logged: one source line (ERR-1). It comes first, as in errx and httperr, and is a plain integer literal. |
  | `ctx` | the `request_id`, `tenant_id` and `actor_id` that middleware attached once with `logger.With`, plus `trace_id` if the project has tracing |
  | event name | *what* happened (LOG-3) |
  | key–value pairs | the values, each one queryable (LOG-2) |

- The package has six functions: `Debug`, `Info`, `Warn`, `Error`, `With(ctx, key, value, …) context.Context`, and `Initialize(cfg)`, which only a command's `run` calls.
- Nothing in `internal/` calls `slog` directly or holds a `*slog.Logger`. The exception is a `pkg/` library, which can't import the service's logger. It takes a `*slog.Logger` in its `Config`, and `wire.go` passes `slog.Default()` (LAY-1).
- The package lives in `internal/foundation/logger`, so every layer, core included, may import it (LAY-6).

**What this logger deliberately doesn't have.** The logger is package-level functions over `slog.Default`, with the stamp first. An older logger of the same shape may have any of these five things. This one leaves them out.

| | This logger has | Why |
|---|---|---|
| Instead of `type tid = int`, an alias | `type stamp int`, unexported | With an alias, any `int` variable compiles in the stamp position. A defined, unexported type accepts only an integer literal, as errx's stamp does (ERR-1). |
| Instead of `event string` | `event eventName`, unexported | A variable or a `fmt.Sprintf` no longer compiles as an event name, and call sites look the same (LOG-3). |
| Instead of calls with no `ctx` | `ctx` second | Without `ctx`, no log line carries `request_id`, `trace_id` or `tenant_id`, except the HTTP access log. Finding the lines for one failed request means matching timestamps. |
| Instead of `Wrap`, `Wrapf`, `WrapError`, `WrapErrorf`, `Infof`, `Warnf`, `Errorf` | `Error(stamp, ctx, event, "error", err, …)` and the other levels | These put the error text or `%s` values into the message, so every line is unique. Lines can't be grouped, counted or alerted on, and the values can't be queried. |
| Instead of `InfoT`/`WarnT`/`ErrorT` and `InfoAttrs`/`WarnAttrs`/`ErrorAttrs` | `Info`/`Warn`/`Error` | Those have the right shape, a constant topic plus key–value pairs, under other names. `Info`, `Warn` and `Error` take the same arguments, plus `ctx`. |

Across three codebases, about three quarters of the log calls in non-test code (382 of 526) used the `Wrap` or `Infof` style.

**Canonical:**

```go
// Package logger is the process's one logger. A command's run calls Initialize once;
// everything else calls Debug, Info, Warn or Error.
package logger

// stamp identifies the source line of one log call (ERR-1). It is unexported,
// so a call site can pass only an integer literal.
type stamp int

// eventName names what happened (LOG-3). It is unexported, so a call site can
// pass only a constant.
type eventName string

// Info logs event at info level. args are key-value pairs.
func Info(ts stamp, ctx context.Context, event eventName, args ...any) {
	write(ctx, slog.LevelInfo, ts, string(event), args)
}

// Debug, Warn and Error are the same at their own levels.

// With returns a ctx whose log lines all carry args. Middleware uses it for
// request_id, tenant_id and actor_id.
func With(ctx context.Context, args ...any) context.Context {
	prev, _ := ctx.Value(argsKey{}).([]any)

	return context.WithValue(ctx, argsKey{}, append(slices.Clip(prev), args...))
}

func write(ctx context.Context, level slog.Level, ts stamp, event string, args []any) {
	l := slog.Default()
	if !l.Enabled(ctx, level) {
		return
	}

	fromCtx, _ := ctx.Value(argsKey{}).([]any)

	all := make([]any, 0, 2+len(fromCtx)+len(args))
	all = append(all, "__tid", int(ts))
	all = append(all, fromCtx...)
	all = append(all, args...)

	l.Log(ctx, level, event, all...)
}

type argsKey struct{}
```

`Initialize` builds the handler: JSON in production, the pretty handler in development, the `service` and `version` attributes, the redacting `ReplaceAttr` (LOG-4), and, if the project has tracing, a wrapper that adds `trace_id` and `span_id` from the ctx's span. It then calls `slog.SetDefault`, so third-party libraries log through the same handler.

The stamp key is `__tid`.

### LOG-2 · Keys

**Rule.**
- Keys are snake_case string literals, written at the call: `"tenant_id", id`.
- An error goes under the key `error`, as the error value: `"error", err`. Never `err.Error()`, which drops errx's structured value and its stamps.
- Use the canonical name when one exists:

```text
__tid service env version request_id trace_id span_id tenant_id org_id actor_id principal_type
method route status_code duration_ms error error_code component
job_kind job_id attempt queue topic partition offset
```

**Why string literals.** A `logkey` package of typed constructors (`logkey.TenantID(id)` returning a `slog.Attr`) would stop key names drifting. That adds a package, a function per key, and a second call shape. A literal key, checked in review, is enough.

**Don't.** The error is stringified under a non-canonical key:

```go
slog.Error("api key auth cache event", "cache", "api_key_auth", "event", "read_error", "__tid", 1778303210, "err", err.Error())
```

**Do:**

```go
logger.Warn(1793624333150, ctx, "auth.cache.read_failed", "cache", "api_key_auth", "error", err)
```

### LOG-3 · Constant event names

**Rule.**
- The event name is a constant, lowercase, dotted name, `<area>.<object>.<event>`, with the event in the past tense: `http.request.completed`, `job.attempt.failed`, `db.rollback.failed`.
- Values go only in key–value pairs.
- The stamp says *where* the line was logged and the event name says *what* happened. Both are needed, because one event name is logged from many places: `db.rollback.failed` is logged wherever a rollback can fail.
- A logged error also brings its own stamps, in the `error` value.

**Don't.** The fields are buried in the message, so they can't be queried:

```go
logger.WrapErrorf(1793625668913, err, "delivery receipt processing failed receipt_id=%s event_id=%s message_id=%s", msg.Job.ReceiptID, msg.Job.EventID, msg.MessageID)
```

**Do:**

```go
logger.Error(1793208864371, ctx, "receipt.processing.failed",
	"receipt_id", msg.Job.ReceiptID, "event_id", msg.Job.EventID, "message_id", msg.MessageID, "error", err)
```

**Enforced by** the compiler. The logger's parameters use unexported types, so only a literal stamp and a constant event name compile. Call sites look exactly the same:

```go
// internal/foundation/logger/logger.go
type (
	stamp     int
	eventName string
)

func Warn(ts stamp, ctx context.Context, event eventName, args ...any) { /* … */ }
```

```text
logger.Warn(1793603411123, ctx, "db.rollback.failed", "error", err)   compiles
logger.Warn(s, ctx, "db.rollback.failed")                             cannot use s (variable of type int) as logger.stamp value
logger.Warn(1793603411123, ctx, name)                                 cannot use name (variable of type string) as logger.eventName value
logger.Warn(1793603411123, ctx, fmt.Sprintf("x.%s", id))              cannot use fmt.Sprintf(...) (value of type string) as logger.eventName value
logger.Warn(ctx, "db.rollback.failed")                                not enough arguments in call to logger.Warn
```

The format of the event name and the keys is checked in review. If review keeps correcting the same mistake, that's when to add a check (COL-13).

### LOG-4 · Levels and secrets

**Rule.** Levels:

| Level | Use for |
|---|---|
| Debug | Diagnostics; off in production |
| Info | Lifecycle events, one access-log line per request, scan summaries |
| Warn | 4xx responses at the encoder, retryable job attempts, degraded best-effort paths |
| Error | 5xx responses, discarded or DLQ'd jobs, failures at goroutine roots |

Secrets:
- Secret types implement `slog.LogValuer` and return `"[REDACTED]"`.
- The handler's `ReplaceAttr` blanks a deny-list of keys: `authorization`, `password`, `token`, `api_key`, `secret`, `cookie`.
- Never log request bodies, raw headers or recovered panic values.

**Enforced by:**
- forbidigo: no `slog` logging functions, `slog.New` or `slog.Default` outside `internal/foundation/logger` and `cmd/`;
- the compiler, for the stamp and the event name (LOG-3);
- the handler's `ReplaceAttr` for the deny-listed keys.

---

## Telemetry

**Telemetry is ignored until a project explicitly needs it.** Don't add tracing, metrics or exporters by default. When a project needs them, it adds them where they're required, and these rules apply to what it adds. How dashboards receive metrics is undecided (parked: see [decisions.md](decisions.md)).

### OBS-1 · Set up once, instrument at the edges

**Rule.**
- **Setup.** `foundation/telemetry.Init(ctx, cfg) (shutdown func(context.Context) error, error)` sets the global tracer and meter providers and the TraceContext + Baggage propagator. The resource carries `service.name`, `service.version` and `deployment.environment`. Shutdown runs last during the drain.
- **Spans** are created at the edges:
  - HTTP middleware creates the server span and installs its ctx with `gofi.SetRequestContext`.
  - The job runtime creates a consumer span from the `traceparent` in job metadata or Kafka headers. Enqueue injects `traceparent`.
  - Adapters use the OTel contrib clients: `otelpgx`, `redisotel`, `kotel`, `otelmongo`, a gocql observer, and `otelhttp.Transport`.
- **Core** imports only the OTel API packages (`otel`, `trace`, `metric`, `attribute`, `codes`). It starts spans only for multi-step use cases, with the scope named by the Go import path.
- **Span names:**
  - HTTP spans are `"{METHOD} {route}"`.
  - Job spans are `"{job_kind} process"`.
  - On error, call `span.RecordError(err)` and `span.SetStatus(codes.Error, kind)`.

**Why.**
- A project that needs metrics and has no telemetry ends up writing them as Info log lines.
- Without `traceparent` in jobs and Kafka messages, a trace stops at the enqueue.
- Without the contrib clients, adapter calls get no spans.

### OBS-2 · Metric naming

**Rule.**
- **Names:** `<svc>.<capability>.<noun>`, lowercase and dotted.
- **Units:** set with `WithUnit`, in UCUM (`s`, `By`, `{job}`).
- **Durations:** `Float64Histogram`s named `….duration`, measured in seconds.
- **No unit suffixes:** no `_total`, `_ms` or `_seconds`. The Prometheus exporter adds them.
- **Attributes** have bounded cardinality and use semconv keys. Never put `tenant_id`, `request_id` or entity IDs on metrics; those go on spans and logs. (parked: see [decisions.md](decisions.md))

**Don't.** A Prometheus-style name, which the exporter would suffix a second time:

```go
requestDurationMs, err := meter.Float64Histogram("edge_request_duration_ms", apiMetric.WithUnit("ms"), /* ... */)
```

### OBS-3 · Capability metrics

**Rule.**
- Each capability declares a typed `Metrics` struct in `core/<cap>/metrics.go`, built from an injected `metric.Meter`.
- Tests use the noop meter.
- Don't add a separate `Recorder` abstraction. The OTel API already is one.

```go
// internal/core/wallet/metrics.go
type Metrics struct {
	provisions metric.Int64Counter
	duration   metric.Float64Histogram
}

func NewMetrics(m metric.Meter) (Metrics, error) {
	p, err := m.Int64Counter("myservice.wallet.provision.attempts", metric.WithUnit("{attempt}"))
	if err != nil {
		return Metrics{}, errx.Wrap(1793260564872, err)
	}
	d, err := m.Float64Histogram("myservice.wallet.provision.duration", metric.WithUnit("s"))
	if err != nil {
		return Metrics{}, errx.Wrap(1793330107456, err)
	}
	return Metrics{provisions: p, duration: d}, nil
}
```

**Enforced by:**
- depguard: the OTel SDK and exporters are importable only in `foundation/telemetry` and `cmd/**`;
- an arch test: core imports only the OTel API packages;
- a small analyzer that checks instrument-name literals.
