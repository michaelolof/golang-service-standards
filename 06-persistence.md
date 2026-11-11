# 06 · Persistence and data access

This covers the shape of a repository package, transactions, queries, concurrency control, idempotency, cursors, caching and migrations.

Every rule here applies whichever database your project uses. Rules that apply to one database only are collected at the end. A project without that database ignores that section.

This document builds on:

- [01](01-project-layout.md): `repository/<aggregate>` may import `adapters/<primarydb>`, and migrations run at release.
- [03](03-domain-and-abstractions.md):
  - consumer-declared `Store` interfaces;
  - `Repo` with `New(db)`;
  - typed IDs;
  - `Parse<T>` for enums;
  - state-machine `Sources`.
- [04](04-errors.md): `Get`/`Find` semantics and `errx.WrapKind` at the boundary.
- [05](05-transport-and-runtime.md): `EnqueueTx` and the API-level `limit` and `cursor`.

## Rules at a glance

| ID | Rule | Enforced by |
|---|---|---|
| DATA-1 | One package per aggregate, with a fixed set of files: `repo.go`, `row.go`, and one file per use case. | `repos`, review |
| DATA-2 | Repository methods take and return domain types. A database schema type isn't a domain type: it's private to the repository, and its fields may use domain types. One column list and one scan function per table, in `row.go`. | depguard, `repos`, review |
| DATA-3 | Method verbs come from a closed set, and names don't repeat the package name. | `repoverbs` |
| DATA-4 | A change to several rows is one repository method named for the use case. Prefer one atomic statement (a CTE); otherwise the repository owns the transaction. Core never holds database semantics. | forbidigo, review |
| DATA-5 | Every query is a constant, declared inside the method that runs it. It moves to package level only when several methods share it. | `constquery`, gosec, review |
| DATA-6 | Time comes from the clock. SQL `now()` is never used. | `constquery` |
| DATA-7 | Optimistic concurrency uses a version column with an atomic compare-and-set. A CAS is never read-then-write. | project test (concurrent update), review |
| DATA-8 | Idempotency keys are unique per tenant in the primary store. `CreateOrGet` returns `created bool`. | `constquery`, review |
| DATA-9 | One `pagination` package handles keyset cursors with a tie-breaker and a filter fingerprint. Driver page tokens never leave the repository. | forbidigo, project test (stable paging) |
| DATA-10 | A cache is a decorator that implements the same port as the repository. All Redis code is one adapter package, with one file per role. Keys follow a fixed format: item keys have a TTL, and queue keys have a stated bound instead. | depguard, review |
| DATA-11 | Migrations are timestamped, forward-only and safe to re-run, live in `db/migrations`, and run only from the migrate tool. | `tools/archcheck`, project test (migrate twice) |
| DATA-12 | Split large files by aggregate, then by concern. | revive `file-length-limit`, review |

---

## DATA-1 · One package per aggregate

**Rule.** The package is `internal/repository/<plural-noun>`, and its files follow a fixed layout:

```
repository/transfers/
  repo.go         Repo, New, var _ transfer.Store = (*Repo)(nil)
  row.go          transferColumns, scanTransfer
  get.go          Get / Find
  list.go         List
  create.go       Create / CreateOrGet
  transitions.go  Cancel / Settle / BeginDispatch (status CAS)
```

**Why.** With a single repository package instead, one `ScyllaRepository` type grew to 186 methods and returned 35 exported `XxxRecord` types holding `gocql.UUID`.

**Enforced by** the `repos` analyzer: `internal/repository` holds one package per aggregate and nothing else, not even shared helpers, which belong in the store adapter; each has `repo.go`, declaring `Repo` and `New`, and `row.go`; and `Repo` is its only exported type. One file per use case is for review.

## DATA-2 · Repositories read and write domain types

**Rule.**
- **Repository methods take and return domain types.** That's the boundary. Other layers never see how a record is stored.
- **A database schema type isn't a domain type**, whatever the database. Domain types carry no database tags.
- **Scan straight into the domain value.** `row.Scan(&t.ID, &t.Status, …)` fills the entity's own fields. pgx and gocql scan by position, so no `db` tags are needed. Repositories with 93 `Scan` calls between them needed none.
- **Pass domain values as query arguments.** IDs, enums and times go to the driver as they are (DOM-1, DOM-4).
- **One column list and one scan function per table**, together in `row.go`:
  - `const transferColumns = …` is the only place the column order is written;
  - `scanTransfer` is the only function that reads it. Every query that selects `transferColumns` scans with `scanTransfer`.
- **NULL** scans into a pointer field. Never use `sql.Null*` types.
- **Enum columns** scan into the enum field, through the enum's `sql.Scanner` method, which rejects unknown values.
- **Values with unexported fields**, such as `domain.Amount` (DOM-2), scan into locals and go through their constructor. This is the only conversion in a scan function.
- **If your project uses gocql:** bind with `gocql.UUID(id)` and scan with `(*gocql.UUID)(&id)` (DOM-1).
- **If your project uses MongoDB:** Mongo maps by field name, so it needs a schema type. Without tags, it stores lowercased Go names (`tenantid`) and generates its own `_id`. Write a private document type with `bson` tags, whose fields use domain types where they fit, plus one function each way.
- **A private schema type** describes any other stored shape that isn't the entity: a join, a projection, or a denormalized table. It is unexported and lives in `row.go`. SQL and CQL don't need one for a plain table, because `Scan` fills the entity's fields by position.

**Don't.** Eighteen `sql.Null*` locals and 34 positional destinations, with the column list kept in two hand-synced copies:

```go
func scanWalletRow(row walletRowScanner) (domain.Wallet, error) {
	var wallet domain.Wallet
	var tenantIdentityRef sql.NullString
	var orgIdentityID sql.NullString
	// ... 16 more sql.Null* locals
	err := row.Scan(
		&wallet.ID,
		&wallet.OrgID,
		// ... 32 more positional destinations
```

The block is repeated 7 times in the same file. The column list exists twice (`walletColumnsForAlias` and `prefixedWalletColumns`). Across the codebase, `sql.Null*` appears 482 times.

**Don't.** A document type is right here, but it stores every ID as a plain `string`. Each read and write then has to convert them, and nothing stops two IDs being swapped. A codebase ended up with 19 repository packages built this way:

```go
type receiptDoc struct {
	ID        string    `bson:"_id"`
	EventID   string    `bson:"event_id"`
	// ...
	CreatedAt time.Time `bson:"created_at"`
}
```

**Canonical** (`repository/transfers/row.go`):

```go
const transferColumns = `id, org_id, tenant_id, status, amount_minor, asset_code, created_at, updated_at, settled_at`

// scanTransfer reads one row selected with transferColumns.
func scanTransfer(row pgx.CollectableRow) (domain.Transfer, error) {
	var (
		t     domain.Transfer
		minor int64
		asset domain.KnownAsset
	)

	err := row.Scan(&t.ID, &t.OrgID, &t.TenantID, &t.Status, &minor, &asset, &t.CreatedAt, &t.UpdatedAt, &t.SettledAt)
	if err != nil {
		return domain.Transfer{}, errx.Wrap(1791016309065, err)
	}

	t.Amount, err = domain.NewAmount(minor, asset)
	if err != nil {
		return domain.Transfer{}, errx.Wrap(1791085806807, err)
	}

	return t, nil
}
```

The same in Mongo. The document's fields use the domain's ID types, which implement `MarshalBSONValue` (DOM-1), and the domain type has no tags (tested: a document like this round-trips through BSON, with `_id` stored as binary):

```go
// internal/repository/receipts/row.go
type receiptDoc struct {
	ID        domain.ReceiptID `bson:"_id"`
	EventID   domain.EventID   `bson:"event_id"`
	// ...
	CreatedAt time.Time        `bson:"created_at"`
}

func (d receiptDoc) toDomain() domain.Receipt {
	return domain.Receipt{ID: d.ID, EventID: d.EventID, CreatedAt: d.CreatedAt}
}

// internal/repository/receipts/get.go
var doc receiptDoc

err := r.coll.FindOne(ctx, bson.D{{Key: "_id", Value: id}}).Decode(&doc)
if errors.Is(err, mongo.ErrNoDocuments) {
	return domain.Receipt{}, errx.WrapKind(1793624697958, errkinds.ErrKindNotFound, err)
}

if err != nil {
	return domain.Receipt{}, errx.Wrap(1792604471582, err)
}

return doc.toDomain(), nil
```

**Enforced by:**
- depguard: `internal/domain` imports no driver (DOM-4);
- the `repos` analyzer: `Repo` is the only exported type, and no exported `Repo` method takes or returns a type from an SDK, such as a `pgx.Tx`. `New` may take the driver's handle;
- review, for one column list and one scan function per table.

## DATA-3 · Method verbs

**Rule.** Repository methods use verbs from a closed set.

| Verb | Meaning |
|---|---|
| `Get` | Returns not_found if the record is missing |
| `Find` | Returns `(T, bool, error)` |
| `List` | Cursor-paged |
| `Count` | |
| `Create` | Returns conflict on a duplicate |
| `CreateOrGet` | Idempotent; returns `(T, created bool, error)` |
| `Update` | Version CAS |
| `Upsert` | |
| `Delete` | |
| Domain transition verbs (`Cancel`, `Settle`, `MarkX`) | Always a status CAS |
| `*CAS`, `Compare*` | A single atomic statement (DATA-7) |

- **Banned verbs:** `Insert`, `Fetch`, `Lookup`, `Load`, `Read`, `Save`, `Put`, `Search`.
- **No stutter:** don't repeat the package name. Write `transfers.Repo.Get`, not `TransferRepo.GetTransfer`.

**Enforced by** the `repoverbs` analyzer, on `Repo`'s exported methods: none starts with a banned verb, and none contains the package's name, plural or singular. Transition verbs are an open set, so only the banned verbs are reported.

## DATA-4 · Atomic work is one use-case-shaped repository method

**Rule.**
- When a use case changes several rows together (a transfer, its operation, its outbox event), core calls **one repository method named for the use case**: `CancelInitiated`, `Settle`, `CreateDetected`. That method makes the change atomic.
- Inside that method, use the first of these that works:
  1. **One statement.** A CTE in CockroachDB, a lightweight transaction or logged batch in ScyllaDB, or a single-document update in MongoDB. The guard (status, version, "no provider reference yet") goes in the `WHERE` clause or filter, so nothing can change between the check and the write.
  2. **A transaction the repository opens and closes**, when the database can't do it in one statement. For example, CockroachDB rejects two mutations of the same table in one statement. The transaction never leaves the method.
  3. **A transaction around several repository calls, opened by core.** Use this only when the change spans repositories that can't sensibly be one method. Write the reason at the call site. Core declares a one-method interface, `Within(ctx, fn func(ctx context.Context) error) error`. An adapter implements it and puts the transaction on the ctx it passes to `fn` (CTX-1), so the repositories pick it up and core still holds no handle.
- Core never sees SQL, connections, transaction handles or rollback sentinels. It asks for a business change and gets back the result or an error kind.
- Work outside the database (HTTP calls, cache invalidation, notifications) happens after the method returns.

**Why.**
- One statement means one round trip and no gap between the check and the write. CockroachDB also retries implicit single-statement transactions on the server.
- Database semantics stay in the repository. Core reads as business steps (STY-9).
- Prefer use-case-shaped CTE-backed repository methods over composing many small repository writes in a service transaction.
- Where a service composes writes in a transaction instead, business code ends up steering the database:
  - A use case returns a sentinel error, `errInflowAlreadyDetected`, from inside a `WriteWithOutbox` transaction only to roll back the outbox insert when the inflow already exists.
  - A `withTxn` helper ended up with six copies, two of them in HTTP handlers.

**Do.** Cancel the transfer and abandon its operation in one statement. The guards are part of the `WHERE` clause:

```go
func (r *Repo) CancelInitiated(ctx context.Context, args domain.CancelInitiatedTransferArgs) (domain.Transfer, error) {
	const q = `
		WITH cancelled AS (
			UPDATE transfers
			SET status = 'CANCELLED', cancelled_at = @cancelled_at, cancelled_by = @cancelled_by, cancelled_reason = @cancelled_reason
			WHERE org_id = @org_id AND tenant_id = @tenant_id AND id = @id
			  AND status = 'INITIATED'
			  AND provider_unique_reference IS NULL
			RETURNING ` + transferColumns + `
		),
		abandoned_op AS (
			UPDATE transfer_operations
			SET state = 'ABANDONED', terminal_reason = @terminal_reason
			WHERE org_id = @org_id AND tenant_id = @tenant_id AND id = @operation_id
			  AND state IN ('WAITING', 'RUNNING', 'RETRYING')
			  AND transfer_id IN (SELECT id FROM cancelled)
		)
		SELECT ` + transferColumns + ` FROM cancelled`
	// ...
```

No rows back means a guard failed. The method returns `ErrKindNotFound` (or another kind from `errkinds`, such as `invalid_transition`), and core maps it.

**Do** (create-or-get with its event, one statement). The event is inserted only if the inflow was:

```sql
WITH inserted AS (
	INSERT INTO inflows (...) VALUES (...)
	ON CONFLICT (org_id, provider_name, provider_unique_reference) DO NOTHING
	RETURNING <inflowColumns>
),
event AS (
	INSERT INTO outbox (...)
	SELECT ... FROM inserted
)
SELECT <inflowColumns> FROM inserted
UNION ALL
SELECT <inflowColumns> FROM inflows
WHERE org_id = @org_id AND provider_name = @provider_name AND provider_unique_reference = @provider_unique_reference
  AND NOT EXISTS (SELECT 1 FROM inserted)
```

**Per database:**

| Database | One statement | When one statement isn't enough |
|---|---|---|
| CockroachDB | CTE, guards in `WHERE`; `INSERT … ON CONFLICT DO NOTHING RETURNING` | `crdbstore.DB.WithinTx` inside the repository method; retries `40001` |
| MongoDB | `FindOneAndUpdate` with the guard in the filter | A session transaction inside the repository method |
| ScyllaDB | LWT (`UPDATE … IF status = ?`) for a guarded change to one partition; a logged batch for the same entity across several tables | No multi-partition transactions. Model the change as one guarded write plus a recovery scan (JOB-3). |

**Transactions when a repository needs one** (option 2):

```go
// adapters/crdbstore/tx.go
// WithinTx runs fn in one serializable transaction and retries SQLSTATE 40001,
// so fn must be safe to run again. Repositories call it; core doesn't.
func (db *DB) WithinTx(ctx context.Context, fn func(tx pgx.Tx) error) error {
	return db.retry(ctx, func() error {
		return pgx.BeginTxFunc(ctx, db.pool, db.txOpts, fn)
	})
}
```

**Enforced by:**
- forbidigo on `BeginTx`, `BeginTxFunc`, `StartSession` and `WithTransaction`, allowed only in `internal/repository/**` and `internal/adapters/*store/`;
- review for option 3: the call site must say why one repository method can't do it.

## DATA-5 · Queries are constants, declared where they run

**Rule.**
- **Where.** Declare a statement as `const q = …` inside the method that runs it. Move it to package level only when two or more methods run it, which should be rare. Shared fragments, such as a table's column list, are package-level constants.
- **Composition.** Build statements only by concatenating constants (for example `transferColumns`).
- **Optional filters** are written as `(@x::T IS NULL OR col = @x)`. No `WHERE` builders and no `Sprintf`. These return the same rows as adding each filter only when it's set, including on columns that can be NULL (tested on SQLite: 16 filter combinations, no differences).
- **Scope.** A use-case method touches the tables its use case changes (DATA-4): the transfer, its operations and its outbox row can be one statement.
- **If your project uses pgx:** use `pgx.NamedArgs` once a query has more than four parameters. Without it, positional parameters reached `$34`.

**Why.** Reading the method shows the query it runs, with no jump to another part of the file.

**Do.** Static SQL with nullable parameters and a keyset cursor:

```sql
FROM kyc_subjects
WHERE org_id = @org_id
	AND (@tenant_id::STRING IS NULL OR tenant_id = @tenant_id)
	AND (@tenant_identity_id::STRING IS NULL OR tenant_identity_id = @tenant_identity_id)
	-- ...
	AND ((@after_at::TIMESTAMPTZ IS NULL AND @after_id::STRING IS NULL) OR (created_at, id) < (@after_at, @after_id))
ORDER BY created_at DESC, id DESC
LIMIT @limit
```

**Don't.** SQL assembled at run time:

```go
nextArg := func(v any) string {
	argN++
	args = append(args, v)
	return fmt.Sprintf("$%d", argN)
}
where := "WHERE org_id = $1"
if params.TenantID != nil {
	where += " AND tenant_id = " + nextArg(*params.TenantID)
}
// ... 12 more branches
```

**Canonical:**

```go
func (r *Repo) Get(ctx context.Context, tenant domain.TenantRef, id domain.TransferID) (domain.Transfer, error) {
	const q = `SELECT ` + transferColumns + ` FROM transfers
		WHERE org_id = @org_id AND tenant_id = @tenant_id AND id = @id`

	rows, err := r.db.Query(ctx, q, pgx.NamedArgs{"org_id": tenant.OrgID, "tenant_id": tenant.TenantID, "id": id})
	if err != nil {
		return domain.Transfer{}, errx.Wrap(1792958803000, err)
	}

	transfer, err := pgx.CollectOneRow(rows, scanTransfer)
	if errors.Is(err, pgx.ErrNoRows) {
		return domain.Transfer{}, errx.WrapKind(1792543352727, errkinds.ErrKindNotFound, err)
	}
	if err != nil {
		return domain.Transfer{}, errx.Wrap(1793436374933, err)
	}

	return transfer, nil
}
```

**If your project uses MongoDB:**
- Build filters, sorts and pipelines in private functions such as `filterForList(args) bson.D`.
- Use `bson.D` wherever order matters.

**Enforced by:**
- the `constquery` analyzer: in a repository, the statement passed to a pgx, `database/sql` or gocql query method is a constant expression. A migration runner may run the statements it reads from files.
- gosec G201 and G202 as a backstop.

## DATA-6 · Time comes from the clock

**Rule.**
- Timestamps are passed in from `clock.Clock` (DOM-3).
- Never use SQL `now()`, `current_timestamp` or `$currentDate`.

**Why.** A repository layer's SQL called `now()` about 180 times. Those rows can't be tested deterministically, and their timestamps disagree with the times the service itself records.

**Enforced by** the `constquery` analyzer: no constant statement passed to a query method contains `now()`, `current_timestamp` or `current_date`, and no string in the code is `$currentDate`.

## DATA-7 · Optimistic concurrency

**Rule.**
- An aggregate that is edited outside its state transitions carries `Version int64`. Others don't need one.
- `Update` writes `… AND version = @version` and sets `version = version + 1`. If zero rows are affected, it returns `ErrKindConflict`.
- State transitions keep the status compare-and-set from DOM-7.
- A method named `*CAS` or `Compare*` is a single atomic statement:
  - SQL `WHERE`;
  - a Mongo filter with `$inc`;
  - CQL `IF`.

  It is never read-then-write.

**Do.** A real fence using a lightweight transaction:

```go
const q = `UPDATE prepared_deliveries_by_event SET version = ?, encrypted_record = ? WHERE project_id = ? AND event_id = ? AND row_kind = 'attempt' AND record_id = 'current' IF version = ?`
// ...
applied, err := r.session.Query(q, next, record, projectID, eventID, expected).WithContext(queryCtx).
	Consistency(gocql.Quorum).SerialConsistency(gocql.Serial).ScanCAS(&existing)
```

**Don't.** A "CAS" that reads and then writes, so two concurrent rotations can both succeed:

```go
func (r *ScyllaRepository) RotateDeliveryReceiptConfigSecretCAS(ctx context.Context, projectID gocql.UUID, expectedCurrentVersion int, record DeliveryReceiptConfigByProjectRecord) (bool, error) {
	current, found, err := r.GetDeliveryReceiptConfigByProject(ctx, projectID)
	// ...
	if currentVersion != expectedCurrentVersion {
		return false, nil
	}
	if err := r.UpsertDeliveryReceiptConfigByProject(ctx, record); err != nil {
```

**Enforced by** a project test, `testutils.AssertConcurrentUpdateConflicts(t, repo)`, required for every repository that has an `Update`. The project writes the helper until the standard ships one (10-adoption).

## DATA-8 · Idempotency keys

**Rule.**
- Idempotency keys live in the primary store under a unique constraint scoped to the tenant: `(org_id, tenant_id, idempotency_key)`. A `request_hash` column sits next to them.
- `CreateOrGet` returns `(T, created bool, error)`. A replay that arrives with a different request hash returns `ErrKindConflict`.
- Always name the conflict target in `ON CONFLICT (…)`. Without one, `DO NOTHING` also swallows a violation of any other unique constraint, and that hides bugs.
- Redis may act as a fast first check, but never as the only guard.

**Enforced by** the `constquery` analyzer, on `ON CONFLICT DO` with no target. The rest is for review.

```go
func (r *Repo) CreateOrGet(ctx context.Context, t domain.Transfer, key domain.IdempotencyKey) (domain.Transfer, bool, error) {
	const q = `INSERT INTO transfers (` + transferColumns + `, idempotency_key, request_hash)
		VALUES (@id, @org_id, /* ... */ @idempotency_key, @request_hash)
		ON CONFLICT (org_id, tenant_id, idempotency_key) DO NOTHING`

	tag, err := r.db.Exec(ctx, q, namedArgs(t, key))
	if err != nil {
		return domain.Transfer{}, false, errx.Wrap(1793340589976, err)
	}

	if tag.RowsAffected() == 1 {
		return t, true, nil
	}

	existing, hash, err := r.getByKey(ctx, t.OrgID, t.TenantID, key)
	if err != nil {
		return domain.Transfer{}, false, errx.Wrap(1793624698138, err)
	}

	if hash != key.RequestHash() {
		return domain.Transfer{}, false, errx.NewKind(1791299727969, errkinds.ErrKindConflict, "idempotency key reused with a different request")
	}

	return existing, false, nil
}
```

## DATA-9 · Cursors

**Rule.**
- **Package.** One `foundation/pagination` package. A cursor is a keyset position `(At time.Time, ID string)`.
- **Wire format.** `base64.RawURLEncoding` of `{"v":1,"at":<unix-nanos>,"id":…,"f":<filter-fingerprint>}`.
- **No signature.** A cursor carries the filter fingerprint, and it isn't signed.
- **Decoding.** The transport decodes the cursor string. A malformed cursor, or one whose fingerprint doesn't match the current filters, is `invalid_input`.
- **Request and response.** Core passes `pagination.Request{Limit, After Cursor}` to the repository and gets back `pagination.Page[T]{Items, Next Cursor}`.
- **No cursor is the zero `Cursor`.** `After` is zero on the first page and `Next` is zero on the last one. Check with `IsZero()`. `Decode("")` returns the zero `Cursor`, and `Encode` turns the zero `Cursor` into `""`. A pointer would make `Decode("")` return `nil, nil`, which STY-3 forbids and nilnil reports.
- **Repository query.**
  1. Fetch `limit+1` rows.
  2. Order by `(sort_col DESC, id DESC)`.
  3. Filter with a row-value predicate: SQL `(created_at, id) < (@at, @id)`, a Mongo `$or`, or a CQL clustering slice.
- **Limits.** The defaults and maximum are set at the API (HTTP-5). Repositories receive a limit that has already been validated.
- **Driver page tokens** (Scylla `PageState`) never leave the repository.

**Do.** A keyset tie-break, with the cursor bound to the filters it was issued for:

```go
// Transport: a malformed cursor, or one issued for other filters, is invalid_input.
after, err := pagination.Decode(s.Request.Query.Cursor, filters.Fingerprint())

// Repository: the keyset tie-break, here in Mongo.
if !args.After.IsZero() {
	andClauses = append(andClauses, bson.M{"$or": bson.A{
		bson.M{"occurred_at": bson.M{"$lt": args.After.At}},
		bson.M{"occurred_at": args.After.At, "_id": bson.M{"$lt": args.After.ID}},
	}})
}
```

**Don't.**
- **Paging on `created_at` alone.** Rows that share a timestamp are skipped.
- **Hand-written cursor codecs inside handlers.** They reached 14 pairs. Keep cursor encoding in `foundation/pagination`.
- **Base64-decoding the client's cursor straight into a Scylla `PageState`.** Clients then hold driver internals.

**Canonical:**

```go
package pagination

// Cursor is a keyset position. The zero Cursor means none: the first page's
// After, and the last page's Next.
type Cursor struct {
	At time.Time
	ID string
}

func (c Cursor) IsZero() bool { return c.ID == "" }

type Request struct {
	Limit int
	After Cursor
}

type Page[T any] struct {
	Items []T
	Next  Cursor
}

func Encode(c Cursor, fingerprint string) string           { /* zero -> ""; else v1 JSON -> RawURLEncoding */ }
func Decode(s, fingerprint string) (Cursor, error)         { /* "" -> zero; mismatch -> ErrKindInvalidInput */ }
func Trim[T any](items []T, limit int, key func(T) Cursor) Page[T] // drops the sentinel row, derives Next
```

**Enforced by:**
- forbidigo on `PageState` under `internal/http/**` and `internal/core/**`. `base64` itself isn't banned, because webhook signature checks legitimately use it.
- review, to keep cursor encoding inside `foundation/pagination`;
- a project test, `pagination.AssertStablePaging(t, list)`, required for every `List`. The project writes the helper until the standard ships one (10-adoption). It inserts N rows with the same timestamp and pages through them with a small limit, checking there are no duplicates and no gaps.

## DATA-10 · Redis and caching

**Rule.** Any cache follows the cache-aside, failure and invalidation bullets. The other bullets apply if your project uses Redis. A project without Redis ignores them.
- **Cache-aside is a decorator.** It implements the same consumer port as the repository and is wired in `cmd`. Core doesn't know the cache exists.
- **One package:** all Redis code is in `adapters/redis`, over one client (ABS-7). Each role is a file: `cache.go`, `lock.go`, `ratelimit.go`, and `queue.go` with `queue_outcome.go` for a job runtime, whose engine is in `pkg/` (JOB-4). Every key family is built in `keys.go`, so code that spans families, such as deleting a project's keys, needs nothing exported.
- **Keys** come in two kinds, and each key family is one or the other:

  | | Item keys | Queue keys |
  |---|---|---|
  | What | One entry each: a cached value, a lock, a rate-limit counter, a piece of state | One long-lived key per queue: a stream, a schedule (sorted set), a dead-letter queue, and the sets that index their entries |
  | Format | `<service>:<family>:v<N>:<id>` | `<service>:<family>:<queue>`. The families are `ready` (the stream), `due` (its schedule) and `dlq` (its dead letters): `myservice:ready:payouts`, `myservice:due:payouts`, `myservice:dlq:payouts` (JOB-4) |
  | Lifetime | Every key has a TTL | No TTL, because it would delete queued work. Each queue states its bound instead (below) |
  | Versioning | Values are private JSON structs. Bump `vN` whenever their shape changes | Each entry carries its own version field. Renaming the key would strand everything queued under the old name |

  - Each key family is built by one unexported function, or one constant for a queue.
  - Use the `{<service>}` hash tag only for key families that multi-key Lua scripts touch.
  - **A queue's bound** is written in the doc comment where the queue is declared, and the code keeps to it:
    - acknowledged entries are deleted, or the stream is trimmed with `MAXLEN` or `MINID`;
    - a dead-letter queue is trimmed by length or age;
    - an index entry is removed by the same script that removes its work.
- **Failures:**
  - A failed cache read logs and falls through to the store. A failed write logs.
  - Neither fails the request.
- **Invalidation** happens after commit.
- **Locks** use `SET NX PX` with an owner token, and Lua compare-then-delete or compare-then-renew.
- **`redis.Nil`** is checked only with `errors.Is`, and only inside the adapter.

**Do.** One package, one client, a file per role:

```text
internal/adapters/redis/
  redis.go       package doc, Config, and New, which builds the one client
  keys.go        every key family, unexported
  cache.go       the cache decorators
  lock.go        Lock
  ratelimit.go   RateLimiter
  queue.go       the job runtime on the stream engine, with each queue's bound (JOB-4)
```

```go
// internal/adapters/redis/redis.go

// Client is the one connection every type in this package uses.
type Client struct {
	rdb *goredis.Client
}

// New builds the client. It performs no network call.
func New(cfg Config) (*Client, error) {
	opts, err := goredis.ParseURL(string(cfg.URL.Reveal()))
	if err != nil {
		// The URL carries the password, so the parse error isn't kept.
		return nil, errx.New(1794333571966, "redis URL is not a valid redis:// or rediss:// URL")
	}

	opts.PoolSize = cfg.PoolSize

	return &Client{rdb: goredis.NewClient(opts)}, nil
}

// internal/adapters/redis/lock.go

// NewLock builds a Lock on the shared client.
func NewLock(c *Client, ttl time.Duration) *Lock {
	return &Lock{rdb: c.rdb, ttl: ttl}
}

// cmd/services/api/wire.go
rdb, err := redis.New(redis.Config{URL: cfg.Redis.URL, PoolSize: cfg.Redis.PoolSize})
if err != nil {
	return errx.Wrap(1794333577763, err)
}

locks := redis.NewLock(rdb, 30*time.Second)
limits := redis.NewRateLimiter(rdb)
```

**Do.** A read-through decorator that implements the port, where a cache failure never fails the request:

```go
var _ ports.DeliveryReceiptConfigResolver = (*DeliveryReceiptConfigCacheAdapter)(nil)

func (a *DeliveryReceiptConfigCacheAdapter) ResolveConfig(ctx context.Context, projectID domain.ProjectID) (domain.DeliveryReceiptConfig, bool, error) {
	if cached, ok := a.getCached(ctx, projectID); ok {
		return cached.Config, cached.Found, nil
	}
	config, found, err := a.store.GetConfig(ctx, projectID)
```

**Do.** A lock with an owner token:

```go
var releaseMigrationLock = goredis.NewScript(`
if redis.call("GET", KEYS[1]) == ARGV[1] then
	return redis.call("DEL", KEYS[1])
end
return 0`)
```

**Don't.**
- Six Redis packages split by role (`rediscache`, `redislock`, `redisqueue` and three more), plus a seventh, `redisclient`, that only shares the one client between them. The queue package deletes the cache's keys through an exported `rediscache.ProjectKeys`.
- Key families with mixed prefixes: `{webhooks}:…`, `rl:`, `cb:` and `worker:`.
- Comparing with `err == goredis.Nil`. Use `errors.Is` instead.

## DATA-11 · Migrations

**Rule.**
- **Layout.** Every project has `db/migrations/` containing `embed.go` (`package migrations`, `var FS embed.FS`).
- **File names** follow `YYYYMMDDHHMMSS_<snake>.up.<sql|cql|json>`. Timestamps avoid number collisions between parallel branches and agents.
- **Change rules:**
  - Forward-only.
  - Every statement is safe to re-run.
  - On an `existing` scope (COL-15), destructive changes are split across two releases: expand, then contract. On a `greenfield` scope no old release has to keep working, so one migration is enough.
  - Manual operations live in `db/manual/` and are not embedded.
- **Runner.** Migrations run through a command in `cmd/tools/`, usually `cmd/tools/migrate`, as the release step (LAY-5). The test harness runs the same code. It keeps a version table and takes a lock. Each project chooses what it's built on.

**Do.** The runner documents the CockroachDB DDL autocommit trap and enforces re-runnability with a test, `TestRunMigrations_FreshDatabase`. The trap: `autocommit_before_ddl`, on by default since v25.1, commits each schema change on its own. A file that fails partway can leave its earlier statements applied with no version record, and the next run applies the whole file again.

**Don't.** With no version table, every file is re-applied on every release.

**Enforced by:**
- `tools/archcheck`: every file in `db/migrations`, other than Go files, is named `^\d{14}_[a-z0-9_]+\.up\.(sql|cql|json)$`, and no two share a timestamp;
- a required "migrate twice on a fresh DB" test.

## DATA-12 · Splitting large repository files

**Rule.**
1. Split by aggregate first, giving each aggregate its own package.
2. Within a package, split by concern, one file per concern.

**Why.**
- A single transfers file grew to 1,986 lines and spanned three aggregates: transfers, batches and source nonces. One method in it was 416 lines.
- A single wallets file grew to 2,315 lines.
- For comparison, the largest repository file in a codebase split by aggregate is 456 lines.

```
repository/transfers/          repo.go  row.go  get.go  list.go  create.go  transitions.go  execution_context.go
repository/transferbatches/    repo.go  row.go  batches.go  intents.go
repository/sourcenonces/       repo.go  row.go  reserve.go
```

---

## Database-specific rules

### If your project uses CockroachDB and pgx v5

- Map rows with `pgx.CollectRows`/`CollectOneRow` and the table's scan function (DATA-2). Represent NULL with pointer fields.
- Store IDs in `UUID` columns. pgx reads a named `[16]byte` from `UUID` natively, but can't scan one from `STRING` (DOM-1).
- Use the default serializable isolation level. `WithinTx` always retries `40001`.
- Write error helpers once in `crdbstore`, using `pgerrcode` constants: `IsUniqueViolation` and `IsRetryable`. Without them, the literal `"23505"` reached 10 copies. Use `errx.WrapKind` to keep the cause.

### If your project uses MongoDB v2

- Set write and read concern on the collection in `New` (majority + journal).
- `mongostore.WithinTx` joins the session from `SessionFromContext`. Every operation uses the `ctx` it was given.
- Translate `mongo.IsDuplicateKeyError` to conflict, and `ErrNoDocuments` to not_found (or to `false` from a `Find`).
- `_id` holds the domain ID as binary subtype 4, written by the ID type's `MarshalBSONValue` (DOM-1).
- Validators and indexes are defined in the JSON migrations.

### If your project uses ScyllaDB and gocql

- One table per query.
- A method that writes N projections uses one LOGGED BATCH.
- CAS uses only lightweight transactions (`IF …`, `IF NOT EXISTS`, `SerialConsistency(Serial)`) and checks `applied`.
- No `ALLOW FILTERING`.
- Retry only idempotent statements.
- Classify errors with `errors.As` on gocql error types, never by matching substrings.
- IDs are converted only at the call: `gocql.UUID(id)` to bind, `(*gocql.UUID)(&id)` to scan (DOM-1).
