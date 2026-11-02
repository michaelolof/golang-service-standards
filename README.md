# Go service standards

These are the instructions for writing Go services, for engineers and for the coding agents that work with them. A rule applies when a project has the thing it talks about, so a project with no HTTP server skips the HTTP rules.

They don't depend on any company, host, cloud, CI product, git forge, operating system or coding agent. A project needs git, Go and [Task](https://taskfile.dev), and any agent that can read a file can follow them.

**Owner:** Michael Olofinjana (COL-13).

It covers how a repository is laid out, how code is named and shaped, how business concepts are modelled, and how errors, HTTP, background work, persistence and tests are written.

The libraries [`errx`](https://github.com/michaelolof/errx), [`gofi`](https://github.com/michaelolof/gofi) (v0.10.4 or later) and [`enumx`](https://github.com/michaelolof/enumx) are part of the vocabulary. The standard says how to use them; it doesn't replace them.

## Install

This isn't a package you download. A project keeps its own copy of [this repo](https://github.com/michaelolof/golang-service-standards) in a folder called `.standards/`, pinned to a version, and that copy brings the rules and the checks with it.

If your projects can't reach GitHub, mirror this repo to any git host, and use that address in the steps below and as `STANDARDS_URL` in `Taskfile.yml`.

### 1. Add it to a project

Start at the project's root. The project needs a `go.mod`, at least one Go package, and everything committed, because `git subtree` refuses to run over uncommitted files. These commands use `v0.1.1`. If there's a newer [tag](https://github.com/michaelolof/golang-service-standards/tags), use that instead.

```sh
git subtree add --prefix=.standards https://github.com/michaelolof/golang-service-standards.git v0.1.1 --squash
echo v0.1.1 > .standards-version
cp .standards/templates/Taskfile.yml Taskfile.yml
```

If the project already has a Taskfile, don't overwrite it. Copy the `vars`, the `includes`, `check` and `check:fast` into it from the template.

Then finish the setup:

```sh
task setup
git add -A && git commit -m "Pin standards v0.1.1"
task check
```

`task setup` writes `AGENTS.md` and `.golangci.yml` at the project's root, and turns on the git hook. If the project already has an `AGENTS.md`, it's set aside for review ([see below](#if-the-project-already-has-an-agentsmd)).

`task check` runs every check. The first run takes longer because it builds the linter. An existing project will probably fail it at first, so turn the failures into a task list and fix them ([ADO-6](10-adoption.md#ado-6--new-and-existing-projects)).

### 2. Make it block merges

In your CI, add a job that installs Go and Task, and runs `task check`. Then mark that job as required on the default branch, in your git host's settings. This step can't be done from inside the repo, and without it a failing change can still merge.

### 3. Everyone else

Anyone who clones the project, person or agent, runs `task setup` once. It changes no tracked files on an up-to-date project. It only turns on the git hook, which git doesn't copy between clones.

### What changes in the project

Here's a small sample service called `orders`, before and after the steps above.

**Before:**

```text
orders/
├── .gitignore
├── README.md
├── cmd/
│   └── services/
│       └── api/
│           └── main.go
├── go.mod
└── internal/
    └── domain/
        └── order.go
```

**After:**

```text
orders/
├── .gitignore
├── .golangci.yml        # new: the lint config, with your module path filled in. Don't edit it.
├── .standards/          # new: this repo at v0.1.1: rules, lint config, hooks, templates. Don't edit it.
├── .standards-version   # new: contains v0.1.1
├── AGENTS.md            # new: what agents read. Your own rules go under its last heading.
├── README.md
├── Taskfile.yml         # new: the tasks below. Add your own here.
├── cmd/
│   └── services/
│       └── api/
│           └── main.go
├── go.mod
└── internal/
    └── domain/
        └── order.go
```

Your code, `go.mod`, `README.md` and `.gitignore` don't change, and `go list ./...` still lists only your own packages. A few things change that the tree doesn't show:

- **Git history** gains three commits: two from `git subtree`, and your "Pin standards v0.1.1".
- **On each machine,** `task setup` turns on the git hook, and the first `task lint` builds the linter into `.standards/lint/bin/`, which git ignores.
- **New commands** come with it:

| Command | What it does |
|---|---|
| `task check` | Runs every check. This is what CI runs. |
| `task check:fast` | Runs the checks that don't need the network, plus unit tests. This is what the hooks run. |
| `task fmt` | Fixes formatting, and whatever else the linters can fix on their own. |
| `task setup` | Writes `AGENTS.md` and `.golangci.yml`, and turns on the git hook. |

**If the project already has a Taskfile,** add to it by hand.

Before:

```yaml
version: "3"

tasks:
  run: {desc: Run the service, cmds: [go run ./cmd/services/api]}
```

After:

```yaml
version: "3"

vars:
  STANDARDS_URL: https://github.com/michaelolof/golang-service-standards.git

includes:
  standards:
    taskfile: .standards/Taskfile.standards.yml
    dir: .
    flatten: true

tasks:
  run:        {desc: Run the service, cmds: [go run ./cmd/services/api]}
  test:       {desc: Unit tests, no infra, cmds: [go test -race ./...]}
  build:      {desc: Build everything, cmds: [go build ./...]}
  check:fast: {desc: What the hooks run (the standard's local checks and unit tests), deps: [standards:check:fast, test]}
  check:      {desc: CI gate (read-only), deps: [standards:check, test, build]}
```

### If the project already has an `AGENTS.md`

Nothing is lost, but the old text doesn't go live until someone has checked it. An old instruction can contradict the standard, and in our tests agents sometimes followed the old line, even when `AGENTS.md` said the standard wins ([ADO-3](10-adoption.md#ado-3--one-agentsmd-at-the-root-of-each-code-repo)).

So `task setup` renames the old file and writes a new one:

```text
Before                        After task setup
shop/                         shop/
├── AGENTS.md  # your notes   ├── AGENTS.md      # the standard's text, and an empty section for your rules
└── go.mod                    ├── AGENTS.old.md  # your notes, unchanged. Agents don't load this file.
                              └── go.mod
```

`task check` fails until `AGENTS.old.md` is gone. To finish:

1. Ask any agent to find the conflicts:

   ```text
   Compare AGENTS.old.md with .standards/RULES.md. List each line that conflicts with a rule, with the rule's ID.
   ```

2. Decide what happens to each line:

   | The line... | Do this |
   |---|---|
   | conflicts with a rule | Delete it. If the project really needs it, keep it as a named exception. |
   | repeats a rule | Delete it. |
   | is specific to this project | Move it under `## This project's own rules` in `AGENTS.md`. |

3. Delete `AGENTS.old.md`, and commit.

The end of `AGENTS.md` then looks like this:

```markdown
## This project's own rules

The ledger package is the only one that writes balances.

Exception to TEST-4: in tests, use testify assert, because this project reports every failed check in one run.
```

A named exception tells the agent what to do, but it doesn't turn off a check. A linter finding still needs `//nolint:<linter> // reason` and the engineer's agreement ([COL-7](09-collaboration.md#col-7--refine-then-check)).

Other instruction files are handled like this:

| The project has... | What happens | What to do |
|---|---|---|
| A `CLAUDE.md` with its own text | `task check` fails, because Claude Code would read it instead of `AGENTS.md`, or alongside it if it imports that file | Move its text under the heading in `AGENTS.md`. Then delete it, or leave only the line `@AGENTS.md`. |
| `CLAUDE.md` as a symlink to `AGENTS.md` | The link keeps working | Review `AGENTS.old.md` as above. |
| `AGENTS.md` as a symlink to `CLAUDE.md` | The link is replaced by a real file, and `AGENTS.old.md` holds a copy of the old text | Review `AGENTS.old.md`, and deal with `CLAUDE.md` as in the first row. |
| An `AGENTS.md` in a subfolder | It stays, and `task check` doesn't look at it | Check it against [RULES.md](RULES.md) the same way, and remove any conflicts. |
| An empty `AGENTS.md` | It's replaced | Nothing. |

## How agents get the instructions

`AGENTS.md` is the way in. It's a [shared format](https://agents.md/): a file at the project's root that coding agents read when a session starts. It's committed, so every clone has it. The file tells the agent:

- the rules are mandatory, and a conflict must be raised before doing the work;
- to read [RULES.md](RULES.md), every rule on one page;
- which document to read for the area it's changing;
- that a task isn't finished until `task check` passes.

Some agents need one small step to find it:

| Agent | What to do |
|---|---|
| Reads `AGENTS.md` by itself, such as Codex, Cursor, GitHub Copilot's coding agent or Windsurf | Nothing. |
| Claude Code | Nothing, as long as there's no `CLAUDE.md`. If there is one, Claude Code reads that instead, so `task check` fails until it's deleted or holds only the line `@AGENTS.md`. |
| Reads its own file by default, such as Gemini CLI or Aider | Set its config to read `AGENTS.md`. The [agents.md](https://agents.md/) site shows how. |
| Anything else | Add its instructions file, with one line: "Follow `AGENTS.md`." |

Only Claude Code has been tested. The other rows come from each agent's docs.

**A project's own rules** go under the heading `## This project's own rules`, the last line of the standard's text in `AGENTS.md`. Everything above that heading belongs to the standard. If one of your rules conflicts with the standard, `AGENTS.md` tells the agent to follow the standard, but don't rely on that alone: in our tests it didn't always work. Review your own rules against [RULES.md](RULES.md) when you add them.

**A subfolder can have its own `AGENTS.md`** if a team wants one, for example for one service in a monorepo. `task check` doesn't look at these files, so check each one against [RULES.md](RULES.md) when you add it. The [agents.md](https://agents.md/) site says the nearest file takes priority, so a contradiction there can win in that folder.

**Starting an agent in a subfolder** works, with two differences on Claude Code:
- It reads the root `AGENTS.md`, but may not load `RULES.md` automatically. `AGENTS.md` also tells it in plain words to read the rules, and in our tests it did so every time.
- It doesn't load the root `.claude/settings.json`, so the finish hook doesn't run. If people work this way, copy that file into the subfolder's `.claude/` too. The git hook and CI still apply either way.

## How a project sticks to the rules

An agent can ignore an instruction file, so `AGENTS.md` alone doesn't make anything mandatory. The checks do, at four points:

| When | What happens | Who it covers | How it can be missed |
|---|---|---|---|
| A session starts | The agent reads `AGENTS.md` and the rules. | Any agent that reads `AGENTS.md` | The agent ignores or forgets an instruction. The next three points catch what a check covers. |
| The agent tries to finish | A finish hook runs `task check:fast`. If it fails, the agent gets the errors and has to keep working. | Agents that have a finish hook. For Claude Code: `mkdir -p .claude && cp .standards/templates/claude-settings.json .claude/settings.json`, then commit it. | An agent that can't fix the failure is let go after about 8 to 10 tries. A Claude Code session started in a subfolder skips the hook, unless the settings file is copied there too. |
| Anyone pushes | A git hook runs `task check:fast`, and refuses the push if it fails. | Anyone whose clone has run `task setup` | A clone without `task setup`, or `git push --no-verify`. |
| A change merges | CI runs `task check`, and a required check blocks the merge if it fails. | Everyone | Only by someone who can change the CI job, the Taskfile or the git host's settings. Review changes to those like code. |

`task check` also catches the ways someone could quietly step around the rules:

| If someone... | This fails | And says |
|---|---|---|
| deletes `AGENTS.md`, or changes the standard's part of it | `agents:check` | Move your text below the heading, and run `task agents` |
| leaves an old `AGENTS.old.md` unreviewed | `agents:check` | Review it, and delete it |
| adds a `CLAUDE.md` with its own text | `agents:check` | Move its text into `AGENTS.md`, and leave only `@AGENTS.md` |
| edits `.golangci.yml` to turn a linter off | `lint:config:check` | Run `task lint:config` |
| commits an edit to anything in `.standards/` | `standards:drift` | Undo the edit |
| breaks a rule a tool checks | `lint` or the tests | The rule and the line |

Rules no tool can check yet are checked in review, by citing the rule's ID. [How it's enforced](#how-its-enforced) lists which tools exist today.

### Upgrading

A project moves to a new version with a normal PR, so the rule changes show up in its diff. Here `v0.2.0` stands for the [tag](https://github.com/michaelolof/golang-service-standards/tags) you're moving to:

```sh
git subtree pull --prefix=.standards https://github.com/michaelolof/golang-service-standards.git v0.2.0 --squash
echo v0.2.0 > .standards-version
task setup
git add -A && git commit -m "Upgrade standards to v0.2.0"
```

`task setup` rewrites the standard's part of `AGENTS.md`, and keeps the project's own rules under the heading. `check` depends on `standards:check`, so any new check in the standard starts running without changing the project's Taskfile.

[10 · Adoption](10-adoption.md) has the reasons and what was tested.

## Principles

The rules follow from ten principles. When no rule covers a situation, apply these.

1. **One answer per question.** Where Go allows several reasonable choices, the standard picks one. Being consistent across repos is worth more than being slightly better locally, for people and for agents.
2. **Machines enforce, people judge.** Every rule says how it is checked: formatter, linter, architecture test, analyzer, or review. Review time is spent on design, not on style.
3. **Dependencies point inward, and boundaries translate.** SDK types, driver errors and wire formats stop at adapters, repositories and transports. `domain` and `core` speak only in domain types and errx kinds.
4. **An abstraction must earn its place.** It hides a dependency, enforces an invariant, or has at least two real implementations or consumers today. Otherwise, inline it.
5. **Explicit, not ambient.**
   - Dependencies arrive through `New(Deps)`, or, in HTTP handlers, through gofi's `deps` accessor, which `wire` fills and checks at startup.
   - Time comes from a clock.
   - Tenant and actor travel in `Args`.
   - Config comes as typed sections.
   - There are no globals and no business data hidden in a context.
6. **Fail loudly and early.**
   - Wiring fails at startup.
   - `New` validates its dependencies.
   - Tests fail when their infrastructure is missing.
   - Nothing silently becomes a no-op.
7. **One shape per kind of thing.**
   - A `Service` only where it is justified (HTTP-3), and then one per capability.
   - One use case per method and per file.
   - One handler per file.
   - One repository per aggregate.
   - One job contract.
   - One error mapper per transport.
8. **Handle each error once.** Wrap it once, classify it at the boundary, map it once, and log it once.
9. **Deterministic by construction.** Time, IDs and randomness are injected. Tests are isolated by namespace.
10. **Code stands on its own.** Comments explain *why*. Docs live in doc comments next to the code. Nothing refers to plans or tickets that will disappear.

## One feature, end to end

The pattern is easiest to see in a single use case: cancelling a transfer.

```
cmd/services/api/main.go                     signal ctx → run(ctx) error                      LAY-2
cmd/services/api/wire.go                     build Repo, Service, Deps; fail at startup      LAY-3
internal/http/api/transfers/cancel.go        CancelTransferHandler: bind → deps → Cancel     HTTP-1..3, ERR-7
internal/core/transfer/cancel.go             (ctx, CancelArgs) (CancelResult, error)         ABS-6
internal/core/transfer/store.go              consumer-owned Store interface (≤5 methods)     ABS-1
internal/core/transfer/service.go            New(Deps) (*Service, error)                     ABS-5
internal/domain/transfer.go                  Transfer, TransitionTo(next, at)                DOM-5, DOM-7
internal/domain/transfer_status.go           enumx TransferStatus + transition table         DOM-6, DOM-7
internal/repository/transfers/transitions.go CancelInitiated: one CTE, guards in WHERE       DATA-4, DATA-7
internal/workers/transferjobs/initiate.go    Initiate.Handle: one call, error to the runtime  JOB-1, JOB-2
```

Following one cancel request through those files:

1. `main` creates the signal context and calls `run`.
2. `wire.go` builds every repository, service and handler. Any failure stops the process at startup.
3. The handler binds and validates the request, takes tenant and actor from the authenticated principal, and calls the transfer service's `Cancel` with `CancelArgs`. (Transfers have a service because cancelling coordinates the transfer, its operation and its outbox event. A plain read would call the repository directly.)
4. `Cancel` reads the transfer through its own `Store` interface. It moves the domain entity to its next state with `TransitionTo`, using `now` from the clock.
5. It calls one repository method, `CancelInitiated`. That method cancels the transfer, abandons its operation and writes its event in one statement, with the status guard in the `WHERE` clause (DATA-4).
6. On failure, a kind such as `not_found` or `invalid_transition` travels back unchanged.
7. The handler turns that kind into an HTTP response with one call: `httperr.Map(stamp, err)`.
8. The central error encoder logs it once, with the request's `request_id` and `tenant_id` already attached.

## Documents

| Doc | Covers | Rules |
|---|---|---|
| [01 · Project layout](01-project-layout.md) | Repo root, `cmd` and lifecycle, the `internal/` layers and import direction, package and file naming, package docs, config, Taskfile | LAY-1…14 |
| [02 · Style and naming](02-style-and-naming.md) | Formatting, **the naming table**, function and struct shape, comments, declaration order, modern Go, literals, **code that reads as a story**, what lint can and can't check | STY-1…9 |
| [03 · Domain and abstractions](03-domain-and-abstractions.md) | Typed IDs, money, time, invariants, enums and state machines; interfaces, services, adapters, generics, smells | DOM-1…7, ABS-1…9 |
| [04 · Errors](04-errors.md) | Stamps, which errx functions to use, wrapping, kinds, boundary translation, HTTP and worker mapping, logging once, panics | ERR-1…12 |
| [05 · Transport and runtime](05-transport-and-runtime.md) | gofi handlers (following gofi's GUIDELINES.md), routes, middleware, OpenAPI, moving to gofi v0.10.4; jobs; context; concurrency; logging; telemetry | HTTP-1…7, JOB-1…4, CTX-1…4, CON-1…3, LOG-1…4, OBS-1…3 |
| [06 · Persistence](06-persistence.md) | Repository shape, transactions, queries, concurrency control, idempotency, cursors, caching, migrations, database-specific rules | DATA-1…12 |
| [07 · Testing](07-testing.md) | Test kinds and tags, what to test where, assertions, fakes, tables, parallelism, helpers, per-layer recipes, determinism, meta tests | TEST-1…13 |
| [08 · Security](08-security.md) | Authentication, tenant isolation, secrets, crypto, input and outbound calls, logging of sensitive data, dependencies | SEC-* |
| [security-audit.md](security-audit.md) | The prompt for running a multi-tenant security audit, and the checklist for writing secure code | SEC-26 |
| [10 · Adoption](10-adoption.md) | How a project consumes the standard, and what makes it mandatory for agents and people | ADO-* |
| [09 · Collaboration](09-collaboration.md) | How an engineer and an agent take a task from brief to merge: brief, questions, tests first, plan, report, review, stop rules, autonomy, clear language, task lists, implementation plans | COL-1…16 |
| [lint/.golangci.yml](lint/.golangci.yml) | The shared golangci-lint v2 config, and the standard's own [`requiredtag`](lint/requiredtag) plugin | |
| [decisions.md](decisions.md) | What was decided, and the few things still parked | |
| [RULES.md](RULES.md) | Every rule on one page, generated from the tables in each document. It's what an agent loads first. | |
| [templates/](templates), [hooks/](hooks), [Taskfile.standards.yml](Taskfile.standards.yml) | What a project takes from here to make the rules mandatory ([Install](#install)) | |

There are 151 rules. Each has an ID, so a review comment or an agent can cite one: "violates ERR-3".

## How to use it

- **Writing code:** read this README and the area you're touching. Each area starts with a rules-at-a-glance table, and every rule has a "do" and a canonical example.
- **Reviewing:** cite rule IDs. A correction that keeps recurring is a sign the rule should be enforced by a tool. Add a check, not another comment.
- **Agents:** load [RULES.md](RULES.md) and the area documents for what you're changing, and treat the canonical snippets as templates. Stamps in the snippets are examples only. Never copy or continue them: each new stamp comes from a clock (ERR-1).
- **Adding it to a project:** follow [Install](#install).

## How it's enforced

From cheapest to most expensive:

| Layer | What | Status |
|---|---|---|
| Formatter | gofumpt and gci, through `task fmt` | Ready ([config](lint/.golangci.yml)) |
| Linter | golangci-lint v2 with 40 linters and 2 formatters, including depguard, forbidigo, wsl and the standard's own `requiredtag` plugin. `task lint` builds it | Ready ([config](lint/.golangci.yml), [plugin](lint/requiredtag)) |
| Architecture test | A shared, table-driven `internal/architecture` test that encodes the layer table, naming bans and test hygiene. [LAY-7](01-project-layout.md) has its table. | To build |
| Compiler | Unexported `stamp` and `eventName` types: only a literal stamp and a constant event name compile | errx does this. A project's `httperr` and `logger` packages do it by following ERR-1 and LOG-1 |
| Custom analyzers | `constquery` (constant queries), `repoverbs` (repository verbs) | To build |
| Shared test helpers | `pagination.AssertStablePaging`, `AssertConcurrentUpdateConflicts`, the migrate-twice test, conformance suites | To build |
| Standard's own checks | `AGENTS.md`, `.golangci.yml` and `.standards/` are intact, through `standards:check` | Ready ([Taskfile](Taskfile.standards.yml)) |
| CI | `task check`: the standard's own checks, plus arch, test, build, `generate:check` and vuln as a project has them, in any CI that can run a command | Each project sets it up (ADO-5) |
| Review | What tools can't decide: abstraction fit, naming quality, comment quality | Ongoing |

[02 · Style and naming](02-style-and-naming.md#what-the-lint-config-covers-and-what-it-cant) maps each rule that lint can't express to the check that covers it.

## Status

The rules are written, and nothing is open. Four things are parked ([decisions.md](decisions.md)).

[10 · Adoption](10-adoption.md#whats-built-and-what-isnt) lists what's built and tested, and what isn't built yet.
