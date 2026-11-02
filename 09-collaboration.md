# 09 · Collaboration: how engineers and agents work on a task

The other documents say what the code looks like. This one says how a task gets from a request to merged code when an engineer works with a coding agent or with a colleague. The same steps apply to both.

It draws on three sources:
- what already works in a service's docs: an `AGENTS.md` that requires plain language, an instructions file that puts sources in priority order, and a guide that has the agent keep explaining until the engineer understands the change;
- *The Clean Coder*, which is about how professionals work: commitments, saying no, acceptance tests, and when to stop;
- the review corrections that started this standard. Each one is a step that was skipped.

**Roles.**
- **The engineer** owns the problem and every decision: what "done" means, which trade-off to take, whether a new service or abstraction is allowed. The engineer reviews, approves and merges.
- **The agent** does the work inside the standard. It asks when something is ambiguous, says no when a request breaks the standard, and reports the facts.
- **The reviewer** is a person, optionally helped by a second agent that starts with fresh context.

## Rules at a glance

| ID | Rule |
|---|---|
| COL-1 | Instructions are applied in a fixed priority order. General best practice never overrides the standard. |
| COL-2 | Every task starts with a brief that says what "done" means. |
| COL-3 | The agent reads before it writes, then lists ambiguities and proposed cases. The engineer resolves them. |
| COL-4 | Acceptance tests come first, and the implementer is never their only author. |
| COL-5 | Task-list work, and any work that adds a package, type, interface, service or table, starts with a plan the engineer approves. |
| COL-6 | Implement in small steps, each ending green. Change only what the task needs. |
| COL-7 | Refine against the standard with tests green, then run `task check`. |
| COL-8 | Report in binary terms: done or not done, with the evidence. Never "should work". |
| COL-9 | Explain until the engineer understands the change. Understanding is part of done. |
| COL-10 | Review covers four concerns: works, solves the real problem, fits, reads well. |
| COL-11 | Stop rules: two failed attempts, growing scope, a request that breaks the standard, or pressure to skip a step. |
| COL-12 | Autonomy is granted per task type and earned through measured results. |
| COL-13 | Every repeated correction becomes a rule, a check, or a canonical example. |
| COL-14 | Everything written for a person uses clear, approachable language, with a practical example and a code source. |
| COL-15 | Each project keeps one living `tasklist.md`: a tracker table grouped into phases that each deliver something useful, linked to a section per task. |
| COL-16 | Each task being worked on is detailed in a short-lived implementation plan in `implementation/`, which ends when its tasks do. |

---

## COL-1 · Source priority

**Rule.** When instructions conflict, apply them in this order:

1. The engineer's current request.
2. The repo's `AGENTS.md`.
3. This standard.
4. The repo's product and feature docs (PRD, feature specs, task list, implementation plans, runbooks).
5. Existing code in the repo, starting with the canonical examples.
6. General Go practice.

General best practice never overrides a rule here. If the current request conflicts with the standard, the agent says so before doing it (COL-11).

**Agent instruction files.**
- Each code repo has one `AGENTS.md` at its root, kept short: the source priority, required reading for each area, and the repo's mandatory rules. [10-adoption.md](10-adoption.md) shows how it's set up.
- Library guidance lives in this standard, not in per-repo skill files. The matching sections here replace any skill files for errx, gofi, httperr, enumx and the logger, because copies drift.

**Why.** This order is already used in practice, and it works. Without an order, an agent resolves conflicts by whatever it saw most often in training.

## COL-2 · The brief

**Rule.** The engineer starts every task with a brief. A short one is fine; a missing one is not. If the task is on the task list, the brief can just name it, because its section already gives the goal and the acceptance criteria (COL-15).

```text
Task:        Admins can cancel a transfer that hasn't reached the provider.
Done when:   - INITIATED, no provider reference → CANCELLED, operation abandoned, event queued
             - already CANCELLED → returned unchanged, no second event
             - has a provider reference → invalid_input
             - PENDING or later → invalid_input
             - task check passes
Priority:    correctness, then API shape. Performance is not a concern here.
Read:        DOM-5, DOM-7, DATA-4, HTTP-3; features/transfers.md
Terms:       transfer, provider reference, operation (domain/doc.go glossary)
Don't touch: the custody adapter; the transfers table schema
```

**Why.** "Done when" is the acceptance criteria. Without it the agent decides what done means, and its version is always "it compiles and the tests I wrote pass" (*The Clean Coder* ch 7: acceptance tests are how you "remove ambiguity" and agree on done).

## COL-3 · Read, then ask

**Rule.** Before writing code, the agent:
1. reads the docs the brief names, the code it will change, and the nearest canonical example of the same kind of thing (handler, repository method, worker);
2. lists every ambiguity it found, each with a proposed answer;
3. proposes extra cases in the same given/when/then form as the brief.

The engineer answers before any code is written. An ambiguity is a decision that belongs to the engineer, and guessing it is how wrong code gets written confidently.

**Why.** "An ambiguity … represents an argument amongst the stakeholders" (*The Clean Coder* ch 7). Asking costs one message. A wrong guess costs a review cycle and often a rewrite.

## COL-4 · Acceptance tests first

**Rule.**
- The cases in "Done when" become tests before the implementation: core tests with fakes for business rules, and integration tests for queries (TEST-1).
- The engineer writes them, approves the agent's draft, or has a separate spec agent write them. **The agent that implements is never their only author.**
- The implementer shows the tests failing for the right reason before it starts.
- The implementer may not change an approved test to make it pass. If a test looks wrong, it stops and says which test and why (COL-11).

**Why.** A test written after the code, by whoever wrote the code, checks what the code does, not what it should do. *The Clean Coder* calls these tests "defense" (ch 5) and requires that "the developer who writes the test is not the same as the developer who implements" (ch 7). An agent editing a failing assertion until it passes is the same failure in a few seconds.

## COL-5 · Plan before structure

**Rule.** If the work is a task on the task list, or adds a package, type, interface, service, table, job kind or endpoint, the agent first writes an implementation plan (COL-16) and the engineer approves it. Where the work adds structure, the plan also covers:
- the files and layers it will touch;
- each new abstraction, with the test it passes (03: hides a dependency, enforces an invariant, or has two real users);
- the order of the steps, and the point where it is cheapest to change course.

A new service always needs explicit approval (HTTP-3). Small changes inside existing structure that aren't on the task list need no plan.

**Why.** Structure is expensive to undo and easy for an agent to add. Most "this abstraction isn't needed" review comments come from here.

## COL-6 · Implement in small green steps

**Rule.**
- Work in slices, each ending with the tests green. Commit at each green step, so there is always a known-good point to return to.
- Make the smallest change that satisfies the brief and keeps the architecture.
- Don't touch unrelated code, and never revert changes you didn't make.
- Don't add libraries, frameworks, error systems or layouts the standard doesn't name.

**Why.** "A failing test holds the context" (*The Clean Coder* ch 4). Small green steps keep every mistake close to its cause, and the Rule of Holes applies: when you're in one, stop digging (ch 9).

## COL-7 · Refine, then check

**Rule.** Once the tests pass:
1. Refactor the change against the standard, keeping the tests green: names (STY-2), function shape (STY-3), the story (STY-9), comments (STY-5).
2. Run `task check`: format, lint, architecture tests, unit tests and the integration tests for the touched packages.
3. Fix what it reports. Never silence a check to get to green (Clean Code G4); an exception needs a `//nolint:<linter> // reason` and the engineer's agreement.

**Why.** "Getting it working and making it clean are two different activities" (Clean Code ch 14). Doing them in that order means cleaning never breaks behaviour.

## COL-8 · Report in binary terms

**Rule.** The report says done or not done, and shows the evidence:

```text
Done.
- task check passes (lint, arch tests, 412 unit tests, 38 integration tests in repository/transfers).
- TestService_Cancel: 4 of 4 cases pass; cases unchanged from the brief.
- Changed: domain/transfer.go (Cancel), repository/transfers/transitions.go (CancelInitiated: one CTE),
  http/api/transfers/cancel.go (CancelTransferHandler).
Not done:
- The SQL guard and the domain rule both check "no provider reference". The duplication is deliberate:
  the SQL guard protects against concurrent requests. Say if you want it written differently.
Deviations: none.
```

Never "should work", "I believe this fixes it" or "tests should pass". If validation couldn't run, say exactly why, and that the task is therefore not done.

**Why.** *The Clean Coder* calls reporting work as done when it isn't "false delivery" (ch 4), and asks for "the language of commitment": say what you will do, mean it, and do it (ch 3). An agent's report is only useful if it can be trusted without being re-checked.

## COL-9 · Explain until it's understood

**Rule.** Code nobody understands is not done, even if it passes. After the report, the agent explains, in the language COL-14 sets out:
1. **the problem:** why it existed, and the options;
2. **the solution:** why this one, the design decisions, the edge cases;
3. **the impact:** what else this change affects.

For non-trivial changes, the engineer restates their understanding and the agent fills the gaps.

**Why.** "It is not good enough that it passes all the tests. You must *know*" (Clean Code G21). The engineer will own this code, debug it at night and explain it to the next person.

## COL-10 · Review

**Rule.**
- A person approves every merge. Nothing merges unreviewed.
- The review covers four concerns, in this order:
  1. **Works:** the acceptance tests cover the brief.
  2. **Solves the real problem:** not just the literal request.
  3. **Fits:** follows the standard, with no new structure that wasn't approved.
  4. **Reads well:** the story test (STY-9), naming and comments.
- Lint already covers formatting and mechanical style, so review time goes on the four concerns and on the rules lint can't check ([02](02-style-and-naming.md#what-the-lint-config-covers-and-what-it-cant)).
- Edits to approved acceptance tests are flagged and discussed, never waved through.
- The writing is reviewed too. If the reviewer has to ask what a report, PR description or comment means, or where it is in the code, it goes back (COL-14).
- A second agent with fresh context may review first. It gets the diff and the brief, not the implementer's reasoning, so its read isn't anchored on it.

## COL-11 · Stop rules

**Rule.** The agent stops and reports, instead of continuing, when:
- **The same failure survives two attempts.** It reports its hypothesis and what it tried, then backs out to the last green commit.
- **Scope grows**, or a requirement conflicts with an invariant. It raises this as soon as it sees it, not at the end.
- **A request breaks the standard.** It says "No, but here's what I can do", and offers the closest compliant option.
- **It is under pressure to skip a step.** Urgency doesn't change the process: "rely on your disciplines" (*The Clean Coder* ch 11).

**Why.** An agent that keeps going produces confident output whatever happens. These are the points where continuing does more harm than stopping.

## COL-12 · Graduated autonomy

**Rule.** How much an agent may do without checking in depends on the type of task, and is earned:

| Level | The agent may | Typical task types |
|---|---|---|
| 0 · Propose | Write a plan or a draft for review; change nothing | security-sensitive code (08), schema migrations, new services |
| 1 · Implement with approval | Implement after the plan is approved; stop at each COL-11 point | new endpoints, new use cases, new job kinds |
| 2 · Implement and open a PR | Go from brief to PR, with review at the end | tests for existing behaviour, refactors with tests green, lint fixes |

A task type moves up a level when its recent PRs merged with few review corrections. It moves down after a correction that a rule should have prevented.

## COL-13 · Keep the standard alive

**Rule.**
- Each correction made twice in review becomes one of these:
  - a lint rule or architecture test, if a machine can check it;
  - a canonical example, if it's about shape;
  - a sentence in the relevant rule, if it needs judgement.
- The code agents copy most (handlers, repositories, tests, `wire.go`) is cleaned first. Agents imitate the code around them, so that code teaches more than any document.
- A fixed set of small practice tasks ("katas") runs against the standard regularly, for example: add a list endpoint, add a guarded transition, add a job. Their review corrections measure whether the standard is working.
- The standard has one named owner, who accepts or rejects changes to it.

**Why.** "Specifying requirements in such detail that a machine can execute them *is* programming" (Clean Code ch 1). A prompt can point at the rules; it can't replace them. Every correction that stays in someone's head will be needed again next week.

## COL-14 · Clear language, practical examples, code sources

**Rule.** Everything written for a person uses clear, approachable language, shows a practical example, and says where in the code it comes from. That covers proposals, plans, questions, reports, explanations, findings, PR descriptions and review comments, whether an agent or an engineer wrote them.

Write for a smart colleague who is new to this code. If they have to read something twice, or ask what it means, it isn't finished.

**Language.**
- Use short sentences and everyday words, with one idea per sentence.
- Say the main point first, then the detail.
- Explain a technical term in a few words the first time you use it.
- Say what happens, not what category it falls into: "the client can't parse the response", not "a serialization concern".
- A rule or issue ID is a pointer, not an explanation. The sentence must still make sense with the ID removed: "a failed type assertion on the input in `Decode` is now a 400 (BUG-28)", never "BUG-28 is fixed".

**Examples.**
- For behaviour, show the input, what happens, and what should happen. For code, show the code today and the code after.
- Use real code from the repo, with its real names. Avoid `foo`, `bar` and `doStuff`.
- If there's no real case, you may invent one, but say that it's invented. An invented example never counts as evidence that a problem is real.

**Code sources.**
- Point to the code with its path and line, from the repo root: `internal/core/httperr/encoder.go:45`. For a library, add the version: gofi v0.10.3 `mux.go:593`.
- Show the lines when the reader needs them to follow the point. Don't make them open a file just to understand the sentence.
- Name the function too if the document will outlive the commit, because line numbers move.
- Say how far you checked, in these terms: **tested** (reproduced by running code), **read the code** (confirmed by reading the source) or **reported** (found by a research pass and not yet independently checked).
- If you have no source, say so: "I haven't found where this happens yet."

**What each piece of writing must say.**

| Writing | Must say |
|---|---|
| Proposal or plan (COL-5) | The problem, today's code and the new code, what it costs, the options not chosen and why, and what you need from the reader |
| Explanation (COL-9) | What happens, step by step and in order, then why it matters, then where it is in the code |
| Finding or bug | What goes wrong, what triggers it, what should happen, whether real code can hit it, where it is, and how far you checked |
| Report (COL-8) | Done or not done, the evidence, and the files changed |
| Question (COL-3) | The question, why the answer changes the work, and your proposed answer |

### Finding

**Don't.** The reader learns the category, not the problem, and can't check it:

> `Relay.Run` has an observability gap: tick errors are swallowed.

**Do.** Say what goes wrong, what should happen, and where:

> **The outbox relay hides its failures, so the logs never show that events have stopped going out.**
>
> On every poll, `Relay.Run` calls `tick`, which reads pending events from the store and publishes them to Kafka. If that read fails, `tick` returns an error and `Run` drops it. Nothing is logged, because this file has no log calls at all.
>
> It should log the error and carry on. The comment says so, but the code doesn't do it. Any failed read while the service is running hits this.
>
> `internal/adapters/kafka/control/kafka.go:298`:
>
> ```go
> if err := r.tick(ctx); err != nil {
> 	// Log error but continue; the relay must not stop on
> 	// transient failures.
> 	continue
> }
> ```
>
> Checked: read the code.

### Proposal

**Don't.** It names a technique, with no problem or cost to weigh:

> Refactor the encoder to use structured marshalling instead of string interpolation, for RFC 8259 compliance.

**Do.** Show the problem and the change, so the reader can decide from this alone:

> **Proposal: build error responses with a Go struct, as another service already does.**
>
> **The problem.** An error message containing a double quote breaks the response, and the client can't parse it. `internal/http/admin/apikeys/create.go:69` sends one:
>
> ```go
> WithMsgf("provider_name %q is not a known provider", v)
> ```
>
> The encoder pastes that message into a string (`internal/core/httperr/encoder.go:45`), so `provider_name=acme` produces this invalid JSON:
>
> ```json
> {"status": "error", "error_code": "BAD_REQUEST_ERROR", "message": "provider_name "acme" is not a known provider", "data": null, "__sid": 1793625671316 }
> ```
>
> **The change.** Use a struct and `json.Marshal`, like `errorBody` in the other service (its `internal/core/httperr/encoder.go:27`). `json.Marshal` escapes the quotes, so the same message becomes `"provider_name \"acme\" is not a known provider"`.
>
> **The cost.** One small struct and four call sites, all in `encoder.go`. The field names clients see stay the same.
>
> **Not chosen.** Removing `%q` from the messages. That fixes today's messages, but not the next one someone writes.
>
> **What I need.** Your approval to change the encoder.
>
> Checked: ran the format string with this message. `json.Valid` is false before the change and true after.

### Explanation

**Don't.** It's accurate, but only someone who already knows the answer can follow it:

> `Dispatch` spawns an unsupervised goroutine with a detached context, so in-flight job state isn't reconciled on process termination.

**Do.** Tell it in the order it happens:

> **A restart can leave an export stuck forever.**
>
> When someone asks for an export, `Dispatch` starts the work in a background goroutine and returns straight away. The goroutine marks the export's row `processing` while it works.
>
> If the process stops, in a crash or a deploy, the goroutine is gone but the row still says `processing`. Nothing goes back to look for rows like that, so the export stays there. `Drain` is meant to wait for running exports at shutdown, but nothing calls it.
>
> `internal/core/export/dispatcher.go:64`:
>
> ```go
> go func() {
> 	defer func() { <-d.pool }()
> 	d.run(context.Background(), job, userEmail)
> }()
> ```
>
> Checked: read the code.

### Example

**Don't.** The reader can't tell if this is real, how to trigger it, or what it looks like:

> A pointer param could be dropped in some cases, which might affect filtering.

**Do.** Give the input, what happens, and what should happen, from real code:

> On gofi v0.7.18, a service's KYC list ignores its filters and returns no error:
>
> ```text
> GET /api/v1/kyc?jurisdiction=NG
>
> What you get:        200, with the filter ignored, so every jurisdiction comes back
> What you should get: 200, with only NG
> ```
>
> The field is `Jurisdiction *string` (`internal/http/api/kyc/list.go:111`). gofi v0.7.18 can't convert the text to `*string`, so it leaves the field nil.
>
> Checked: ran the schema on gofi v0.7.18.

**Why.** A finding nobody understands doesn't get fixed, and a claim with no source has to be taken on trust. An `AGENTS.md` can require plain language in every answer, explanation, plan and doc, and this rule applies that everywhere. It adds the code source, so the reader can check the claim themselves.

The gofi issue list behind this standard shows the cost. Its first versions used bare issue numbers and invented scenarios, and it took several rounds before the list could be acted on.

**How it's checked.** A person checks this, because no linter can. In review (COL-10), anything the reviewer has to ask about goes back. Before sending anything, the writer runs three tests:
1. Could a colleague new to this code say what's wrong, or what's proposed, after one read?
2. Can they open the file and see it for themselves?
3. Remove every rule and issue ID. Does it still make sense?

## COL-15 · The task list

**Rule.** Each project keeps one task list: `tasklist.md`, at the root of its docs repo, or of its code repo if it has no separate one. It holds every task needed to take the project from start to finish, and it stays up to date as the work moves. It usually lasts the project's whole life. If a project has run long and grown large, a task list may cover a significant portion of it instead, and says so at the top. For example: "remaining work after admin auth and tenancy revamp".

The file has two parts, in this order:
1. **The tracker:** one summary table of every task, grouped by phase.
2. **The task sections:** one section per task, linked from the tracker, with the details.

### Phases

A phase delivers something useful. When it ends, someone can do something they couldn't do before. It takes one feature from idea to implementation, across everything that feature needs (API, Admin, Web and so on). Only then does the next phase start.

**Don't.** Don't make a phase out of a layer or a shared component, because none of those lets anyone do anything on its own. One project's first draft did this and had to be corrected, which cancelled four tasks:

> The previous draft placed runtime, provider routing, the broad `acme` adapter, and cross-feature webhook reconciliation ahead of the first accepted Product API feature.

**Do.** Name each phase after the feature it delivers. These are real phases from two projects:

```text
Project A  Phase 2: KYC, end to end
           Phase 3: Vendor wallet provisioning, end to end
           Phase 4: Outflows, end to end

Project B  Phase 3: One-shot event, end to end
           Phase 9: Cold restore, end to end
```

Shared work, such as a queue, an outbox or a provider adapter, goes in the first phase that needs it. A new project's first phase can be the smallest service that starts and answers a request.

To test a phase, ask: "What can someone do when this is done that they couldn't do before?" If the answer is "nothing yet", it isn't a phase.

### The tracker

The tracker comes first in the file, with these columns:

| Column | What goes in it |
|---|---|
| Task ID | A unique ID, linked to the task's section: a prefix and a number, such as `TRF-016`. It never changes and is never reused. It doesn't contain the phase number, because tasks move between phases. |
| Title | What the task delivers, in a few plain words. |
| Phase | The phase number and name, such as `3 · Wallets`. Rows stay in phase order. |
| Scope | Where the work lands: `API`, `Admin`, `Web`, and anything else the project has, such as `Worker` or `SDK`. Usually one scope per task, so each can be planned and finished on its own. |
| Priority | `P0`: the project can't launch without it. `P1`: first work after launch. `P2`: useful, with no date. |
| Dependencies | The IDs of the tasks that must be `done` first, or `None`. |
| Status | One of the six statuses below. |

For example:

```markdown
| Task ID | Title | Phase | Scope | Priority | Dependencies | Status |
|---|---|---|---|---|---|---|
| [TRF-012](#trf-012) | Create and manage wallets through the Product API | 3 · Wallets | API | P0 | TRF-011 | `done` |
| [TRF-013](#trf-013) | Manage wallets through the Admin API | 3 · Wallets | Admin | P0 | TRF-012 | `done` |
| [TRF-014](#trf-014) | Manage wallets on the admin dashboard | 3 · Wallets | Web | P0 | TRF-013 | `done` |
| [TRF-015](#trf-015) | Test the wallet feature end to end | 3 · Wallets | API, Admin, Web | P0 | TRF-014 | `done` |
| [TRF-016](#trf-016) | Add a new asset and one-coin `acme` wallets | 3 · Wallets | API | P0 | TRF-015 | `inprogress` |
| [TRF-017](#trf-017) | Send funds through the Product API | 4 · Outflows | API | P0 | TRF-015 | `todo` |
```

### Statuses

There are six, and no others. Anything else that needs saying goes in the task's section.

| Status | Meaning | Also write in the task's section |
|---|---|---|
| `todo` | Not started. | |
| `inprogress` | Someone is working on it now. Work that's built but not yet tested stays here (COL-8). | A link to the implementation plan |
| `done` | Every acceptance criterion is met, with evidence, and the work is merged. | |
| `blocked` | It can't continue until something outside the task happens. | What's blocking it, and who can clear it |
| `deferred` | Valid work, put off on purpose. | What would restart it |
| `cancelled` | No longer needed. The row stays, so its ID isn't reused. | Why, and who decided |

### The task section

Each task has one section, with these fields. Add more if the task needs them.

| Field | What it says |
|---|---|
| Problem statement | What's wrong or missing today, and who it affects. |
| Goal | What will be true when the task is done. |
| Requirements | What the work must include, and any limits on how it's done. |
| Acceptance criteria | Statements someone can check. They're the "Done when" of the brief (COL-2), and they become the tests (COL-4). |
| Breaking changes | What callers must change, or "None". |
| Implementation plan | A link to the plan, added when work starts (COL-16). |

The section doesn't repeat the phase, scope, priority, dependencies or status. Those live only in the tracker, so the two can't disagree.

Here's the section for `TRF-016`:

```markdown
<a id="trf-016"></a>
### TRF-016 · Add a new asset and one-coin `acme` wallets

**Problem statement.** Customers can't hold a new asset, because the service doesn't list it.
`acme` is also moving to one account per coin, but the service still creates one `USD` account
that receives both USDC and USDT.

**Goal.** Customers can hold the new asset on `acme`, and every `acme` wallet holds exactly one coin,
chosen when the wallet is created.

**Requirements.**
- Map the new asset to `acme`'s code, `NEW`.
- Declare asset support per network, because the new asset only works on Ethereum.

**Acceptance criteria.**
- [ ] The new asset is accepted on `ETH` and refused on any other network.
- [ ] Creating an `acme` wallet with two assets returns a 400 with a readable message.
- [ ] Enabling a different coin on an `acme` wallet returns a 409.
- [ ] Deleting an `acme` wallet's asset returns a 409, and the asset is still there.
- [ ] `acme` accounts are created in the wallet's coin, and nothing sends `USD`.

**Breaking changes.** `supported_assets` stays an array, but on `acme` wallets it must now
hold exactly one asset. There are no live users to migrate.

**Implementation plan.** [implementation/new-asset-and-acme.md](implementation/new-asset-and-acme.md)
```

The `<a id>` line keeps the tracker's link working if the title changes.

### Keeping it up to date

- Change the status when the work changes, not at the end of a phase.
- New work goes on the list when it's found, as `todo`, in the phase that needs it.
- The engineer decides what's on the list: adding, cancelling or deferring a task, and changing a priority or an acceptance criterion. The agent proposes these and may move a task's status as the work goes.
- Don't keep counts by hand. Anyone who needs a count can get it from the table.

**Why.** One list means nobody has to piece the project together from chat and memory. Phases by feature mean that stopping early still leaves something that works. Keeping each fact in one place stops the list contradicting itself, which real task lists do (tested: a script compared each tracker with its sections):

- The tracker says `done`, and the task's section says `in-progress`.
- The tracker says `done`, and the task's section says `todo`.
- The hand-kept summary says 14 tasks are `done` and 7 are `code-complete`. The table has 26 `done` and no `code-complete`.
- The ID says phase 3, but the task is in phase 2. The phase number in the ID is wrong for 33 of one tracker's 40 rows.

Without one format, each project's list drifts its own way. Across three codebases, the file had two names (`tasklist.md`, `PROJECT_TASKS.md`), the plans folder had two spellings (`implementation/`, `implementations/`), each list added statuses beyond the six (such as `revalidate` and `code-complete`), and only one tracker linked to its sections.

**How it's checked.** A person checks it in review, using the phase test above. A script could check the mechanical parts: every row has a section, every dependency exists, and every status is one of the six. That script isn't built yet.

## COL-16 · The implementation plan

**Rule.** Each task being worked on is detailed in an implementation plan: a Markdown file in the `implementation/` folder, beside `tasklist.md`. It says what the work is, how it will be done and why, with clear examples of what the issue is and what needs to change, in the language COL-14 requires. A plan can cover one task or several, as the engineer instructs. The engineer approves it before implementation starts (COL-5).

Plans are kept up to date too, but they're short-lived. A plan lasts as long as its tasks do.

| | Task list | Implementation plan |
|---|---|---|
| Answers | What needs doing, and in what order? | How and why will this task get done? |
| Covers | The whole project, or a large part of it | One or more tasks |
| Lasts | As long as the project | Until its tasks are done |
| Where | `tasklist.md` | `implementation/<topic>.md` |

### What a plan contains

| Section | What it says |
|---|---|
| Header | The tasks it covers (linked), its status, and the repos it touches. |
| Summary | What's being done and why, in a few sentences. |
| The issue today | How the code behaves now, with code sources and an example of what goes wrong. |
| What changes and what doesn't | The behaviour that changes, and what callers can rely on staying the same. |
| Decisions | Each choice: the options, the one taken and why. Open ones are the engineer's to answer (COL-3). |
| Steps | The work in order, each step small enough to end with tests passing (COL-6). Each step says why, what changes, which tests, and when it's done. |
| Risks | What could go wrong, and what we'd do about it. |
| Out of scope | What this plan won't do, and where that work goes instead. |
| Definition of done | Each step's "done when" line, tied back to the task's acceptance criteria. Link to the criteria rather than copying them. |

Here's one step from a plan:

````markdown
### Step 3: Enforce one asset per `acme` wallet

**Why.** A wallet has one address and one provider account. An `acme` account now holds
one coin, so an `acme` wallet can only hold one coin.

**The issue today.** Nothing stops a request that asks for two coins:

    POST /api/v1/wallets
    {"supported_assets": ["USDC", "USDT"], "network": "BASE", ...}

    Today: the wallet is created with both coins, which acme can no longer hold.
    After: 400, "supported_assets must contain exactly one asset for this provider; got 2.
           Create one wallet per asset"

**What changes.**
- Add `OneAssetPerWallet bool` to `Capabilities`. `acme` sets it to `true` in `client.go`,
  so services check the flag, not the provider's name.
- Create: check the asset count after a provider is selected.

  ```go
  // internal/core/wallet/routing.go
  func ValidateWalletAssetCount(assets []domain.KnownAsset, providerCaps domain.Capabilities) error {
  	if providerCaps.OneAssetPerWallet && len(assets) != 1 {
  		return errx.NewKindf(1793625670461, errkinds.ErrKindInvalidInput,
  			"supported_assets must contain exactly one asset for this provider; got %d. Create one wallet per asset", len(assets))
  	}
  	return nil
  }
  ```

- Enable: allow only an asset the wallet already has. Otherwise, return a 409.
- Delete: refuse with a 409, because an `acme` wallet keeps its coin for life.

**Tests.**
- Update `TestCreateWalletHandler_MultiAssetCreatesAllAssetRows`, which now expects a 400.
- Add: two-asset create returns a 400 with a readable message.
- Add: enabling a different coin returns a 409.
- Add: deleting the asset returns a 409, and the asset row is still there.

**Done when.** The new tests pass, and `task test` passes.
````

### Keeping it up to date

- Mark each step as it finishes, and record what changed from the plan.
- If the approach changes, change the plan first. If that changes what the engineer approved, stop and ask (COL-11).
- The task's section links to the plan, and the plan links back to its tasks.

### When the task is done

A plan describes the work, not the system. Once the work is merged, the code and the feature docs are the truth.

1. Move anything that must last into the permanent docs: behaviour into the feature docs, and vendor facts into the vendor doc.
2. Delete the plan. Git history keeps it.
3. Remove the link from the task's section.

A plan that covers several tasks ends when the last of them is done.

**Why.** Keeping the detail in the plan keeps the task list short enough to scan. The plan also shows the engineer the approach before any code exists, which is the cheapest point to change course. Deleting it afterwards matters because a finished plan left in the folder looks like current instructions, and agents follow what they find.

**How it's checked.** A person checks it in review. These go back:
- a task marked `inprogress` with no linked plan;
- a task marked `done` whose plan is still in the folder;
- a plan that fails COL-14's three tests.
