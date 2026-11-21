# 09 · Collaboration: how engineers and agents work on a task

The other documents say what the code looks like. This one says how a task gets from a request to `done` when an engineer works with a coding agent or with a colleague. The same steps apply to both.

It draws on three sources:
- what already works in a service's docs: an `AGENTS.md` that requires plain language, an instructions file that puts sources in priority order, and a guide that has the agent keep explaining until the engineer understands the change;
- *The Clean Coder*, which is about how professionals work: commitments, saying no, acceptance tests, and when to stop;
- the review corrections that started this standard. Each one is a step that was skipped.

**Roles.**
- **The engineer** is the person who owns the work. They own the problem and every decision: what the work must achieve, which trade-off to take, whether a new service or abstraction is allowed. They review the work, decide whether it's `done`, and merge. Only they can mark work `done` or delete an implementation plan. An agent is never the engineer.
- **The agent** does the work inside the standard. It asks when something is ambiguous, says no when a request breaks the standard, and reports the facts. It can mark its work `completed`, never `done`.
- **The reviewer** is a person, optionally helped by a second agent that starts with fresh context.

## Rules at a glance

| ID | Rule |
|---|---|
| COL-1 | Instructions are applied in a fixed priority order. General best practice never overrides the standard. |
| COL-2 | Every task starts with a brief whose "Completed when" lines are its acceptance criteria. |
| COL-3 | The agent reads before it writes, then lists ambiguities and proposed cases. The engineer resolves them. |
| COL-4 | Acceptance tests come first: the promises a task adds or changes, as flows, then the core and integration tests. The implementer is never their only author. |
| COL-5 | Task-list work, and any work that adds a package, type, interface, service or table, starts with a plan the engineer approves. |
| COL-6 | Implement in small steps, each ending green. Change only what the task needs. |
| COL-7 | Refine against the standard with tests green, then run `task check`. |
| COL-8 | `completed` means fully implemented, tested, verified when possible, and meeting the requirement. The agent marks it without asking, and reports completed or not completed, with the evidence. Never "should work". |
| COL-9 | When a workstream's last task is `completed`, the agent writes a completion document straight away, without being asked: a standalone HTML file in `completion/`, written and checked in a browser as COL-22 requires, covering everything it did, why, the impact, code sources, many scenarios, any database change, every transaction used instead of one statement, and every broken promise the flows found. A small change with no plan or task list entry gets one too. It makes the case for the work honestly, and it must not lie. |
| COL-10 | Only the engineer marks a task or a workstream `done`, by an explicit instruction after reviewing it. Review covers four concerns: works, solves the real problem, fits, reads well. |
| COL-11 | Stop rules: two failed attempts, growing scope, a request that breaks the standard, a scope status that doesn't match what the agent finds, or pressure to skip a step. |
| COL-12 | Autonomy is granted per task type and earned through measured results. |
| COL-13 | Every repeated correction becomes a rule, a check, or a canonical example. |
| COL-14 | Everything written for a person uses clear, approachable language, with a practical example and a code source. |
| COL-15 | Each project keeps one living `tasklist.md`: a tracker table grouped into phases that each deliver something useful, linked to a section per task. A task can only depend on tasks above it. A dependency in another workstream that isn't `done` yet is a warning, not a blocker. Each scope is marked `greenfield`, `existing` or `unconfirmed`. The engineer sets it when the task list or a task is written, so the agent asks and never assumes, and nothing is planned or changed while it's `unconfirmed`. |
| COL-16 | Each task being worked on is detailed in an implementation plan, an HTML page in `implementation/`, which the engineer approves before work starts. A plan that adds or changes a write lists each transaction it will use instead of one statement, with its justification and impact. A plan that changes what a caller sees lists each promise it adds or changes, with the flow that will prove it. All the work one plan covers is one workstream. |
| COL-17 | Every implementation plan has a table of all the sub-tasks needed to finish it end to end, in the task list's format, with temporary IDs that exist only in that plan. |
| COL-18 | Only the engineer deletes a proposal, implementation plan or completion document. A finished plan stays in the folder until the engineer explicitly says to delete it. |
| COL-19 | A workstream goes to the engineer whole, implemented end to end. Never ask for `done` on part of it unless the engineer asked for that. If an item must be `done` before another in the same workstream or phase, ask first. |
| COL-20 | A session runs at most one subagent at a time, only when it's justified, and says why before starting it. |
| COL-21 | Number every finding, piece of feedback, decision, concern, question and warning you report, in chat and in documents. A number means one item for the whole conversation or document: never restart at 1, reuse one, or number any other list. The numbers are for the engineer to point at. Never write one without saying what the item is. |
| COL-22 | Proposals, implementation plans and completion documents are standalone HTML pages that read like a clear technical blog post: a walkthrough of the problem, the change and the reasoning, so the engineer can decide from the page alone. Every change is shown as code, with syntax highlighting: today's code with its source, what a caller sees before and after, the diff, the new code and its tests. Visual cues each do a job, colour always has a legend, and a change to a flow between services or to states gets a diagram. Each page is checked in a browser before it's sent. |

---

## COL-1 · Source priority

**Rule.** When instructions conflict, apply them in this order:

1. The engineer's current request.
2. The repo's `AGENTS.md`.
3. This standard.
4. The repo's product and feature docs (PRD, feature specs, task list, proposals, implementation plans, runbooks).
5. Existing code in the repo, starting with the canonical examples.
6. General Go practice.

General best practice never overrides a rule here. If the current request conflicts with the standard, the agent says so before doing it (COL-11).

**Agent instruction files.**
- Each code repo has one `AGENTS.md` at its root, kept short: the source priority, required reading for each area, and the repo's mandatory rules. [10-adoption.md](10-adoption.md) shows how it's set up.
- Library guidance lives in this standard, not in per-repo skill files. The matching sections here replace any skill files for errx, gofi, httperr, enumx and the logger, because copies drift.

**Why.** This order is already used in practice, and it works. Without an order, an agent resolves conflicts by whatever it saw most often in training.

## COL-2 · The brief

**Rule.** The engineer starts every task with a brief. A short one is fine; a missing one is not. If the task is on the task list, the brief can just name it, because the list already gives the goal, the acceptance criteria and the scope (COL-15). Any other brief states the scope and its status if the work could affect users, a deployment or stored data.

```text
Task:           Admins can cancel a transfer that hasn't reached the provider.
Completed when: - INITIATED, no provider reference → CANCELLED, operation abandoned, event queued
                - already CANCELLED → returned unchanged, no second event
                - has a provider reference → invalid_input
                - PENDING or later → invalid_input
                - task check passes
Scope:          API: existing
Priority:       correctness, then API shape. Performance is not a concern here.
Read:           DOM-5, DOM-7, DATA-4, HTTP-3; features/transfers.md
Terms:          transfer, provider reference, operation (domain/doc.go glossary)
Don't touch:    the custody adapter; the transfers table schema
```

**Why.** "Completed when" is the acceptance criteria. Without it the agent decides for itself when the work is finished, and its version is always "it compiles and the tests I wrote pass" (*The Clean Coder* ch 7: acceptance tests are how you "remove ambiguity" and agree on when work is finished).

## COL-3 · Read, then ask

**Rule.** Before writing code, the agent:
1. runs the security check if one is due (SEC-27);
2. reads the docs the brief names, the code it will change, and the nearest canonical example of the same kind of thing (handler, repository method, worker);
3. lists every ambiguity it found, each with a proposed answer;
4. proposes extra cases in the same given/when/then form as the brief.

A scope status that's `unconfirmed` is always on that list, and so is a missing one on work that could affect users, a deployment or stored data, because the agent can't work it out from the repo (COL-15).

The engineer answers before any code is written. An ambiguity is a decision that belongs to the engineer, and guessing it is how wrong code gets written confidently.

**Why.** "An ambiguity … represents an argument amongst the stakeholders" (*The Clean Coder* ch 7). Asking costs one message. A wrong guess costs a review cycle and often a rewrite.

## COL-4 · Acceptance tests first

**Rule.**
- The cases in "Completed when" become tests before the implementation:
  1. the promises the task adds or changes become flows (TEST-18), written from the promise, not from the code;
  2. core tests with fakes for business rules;
  3. integration tests for queries (TEST-1).
- The task updates the area's promise file in `docs/promises` in the same change.
- The engineer writes them, approves the agent's draft, or has a separate spec agent write them. **The agent that implements is never their only author.**
- The implementer shows the tests failing for the right reason before it starts.
- The implementer may not change an approved test to make it pass. If a test looks wrong, it stops and says which test and why (COL-11).

**Why.**
- A test written after the code, by whoever wrote the code, checks what the code does, not what it should do. *The Clean Coder* calls these tests "defense" (ch 5) and requires that "the developer who writes the test is not the same as the developer who implements" (ch 7). An agent editing a failing assertion until it passes is the same failure in a few seconds.
- This rule used to name only core tests and integration tests, so no acceptance test acted as a caller. In one service, about 3,800 tests passed while an approved business couldn't open an account. Its flows, rebuilt from written promises, found three broken promises in their first pass (TEST-18).

**Don't.** A flow written after the code, with its expected message copied from what the code answered. It holds the answer in place, whatever the promise says:

```go
require.Equal(t, "the provider refused the payment", testharness.Refusal(t, resp).Message) // what the code returned
```

**Do.** Start from the promise. The task adds the row to `docs/promises/orders.md`, and the flow written from it fails for the right reason before the implementation exists:

```markdown
| ORD-5 | A cancelled order is refunded in full, and the app is told `ORDER_REFUNDED`. | person, business | an order that has shipped: 409 "order {id} has shipped, so it can't be cancelled" | `TestOrder_ACancelledOrderIsRefundedInFull` |
```

## COL-5 · Plan before structure

**Rule.** If the work is a task on the task list, or adds a package, type, interface, service, table, job kind or endpoint, the agent first writes an implementation plan (COL-16) and the engineer approves it. Where the work adds structure, the plan also covers:
- the files and layers it will touch;
- each new abstraction, with the test it passes (03: hides a dependency, enforces an invariant, or has two real users);
- the order of the sub-tasks (COL-17), and the point where it is cheapest to change course.

When the plan depends on a choice between designs that the engineer must make first, such as the shape of a new table, the agent can put the options in a proposal before writing the plan (COL-22).

A new service always needs explicit approval (HTTP-3). Small changes inside existing structure that aren't on the task list need no plan, but they still need a completion document (COL-9).

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
2. Run `task check`: format, lint, the standard's structural checks, unit tests and the integration tests for the touched packages.
3. Fix what it reports. Never silence a check to get to green (Clean Code G4); an exception needs a `//nolint:<linter> // reason` and the engineer's agreement.

**Why.** "Getting it working and making it clean are two different activities" (Clean Code ch 14). Doing them in that order means cleaning never breaks behaviour.

## COL-8 · `completed`, and reporting in binary terms

**Rule.** A task is `completed` when all four of these are true:

- **Fully implemented.** Everything the task asked for is there, with nothing left for later.
- **Tested.** Tests cover the change, and they and `task check` pass.
- **Verified, when possible.** Someone ran the change and saw it work, outside the tests. For example, they started the service and called the new endpoint. If that wasn't possible, the report says what couldn't be checked and why.
- **Meets the requirement.** Every acceptance criterion holds. If one is impossible or can't be met, the completion document says so clearly (COL-9).

The agent marks work `completed` on its own, without asking. That's as far as it can take the work, because only the engineer can make it `done` (COL-10). Neither status depends on merging, which stays the engineer's step.

The report says completed or not completed, and shows the evidence:

```text
Completed.
- task check passes (lint, archcheck, 412 unit tests, 38 integration tests in repository/transfers).
- TestService_Cancel: 4 of 4 cases pass; cases unchanged from the brief.
- Ran it: on a local instance, cancelling an INITIATED transfer returned CANCELLED and queued one event.
- Changed: domain/transfer.go (Cancel), repository/transfers/transitions.go (CancelInitiated: one CTE),
  http/api/transfers/cancel.go (CancelTransferHandler).
For your attention:
1. The SQL guard and the domain rule both check "no provider reference". The duplication is deliberate:
   the SQL guard protects against concurrent requests. Say if you want it written differently.
Deviations: none.
Security check: not due until 2026-11-11.
Warnings: none.
```

The report always has a "Warnings" line, even if it says "none". A dependency in another workstream that isn't `done` yet goes there (COL-15). So does a security check that couldn't run, or a finding that's still open. The report also has a "Security check" line (SEC-27).

Each item the engineer might answer on its own, such as each point under "For your attention" and each warning, has a number, and every mention of it says what it is (COL-21).

Never "should work", "I believe this fixes it" or "tests should pass". If the tests or `task check` couldn't run, say exactly why, and that the task is therefore not `completed`.

**Why.** *The Clean Coder* calls reporting work as finished when it isn't "false delivery" (ch 4), and asks for "the language of commitment": say what you will do, mean it, and do it (ch 3). An agent's report is only useful if it can be trusted without being re-checked.

## COL-9 · The completion document

**Rule.** When the last task in a workstream becomes `completed` (COL-19), the agent writes a completion document straight away, without being asked: `completion/<topic>.html`, in a `completion/` folder beside `implementation/`, linked from the plan's header. Then it sends the document to the engineer. The agent's work on the workstream isn't over until it has. The engineer decides from it whether the work is ready for `done`, or needs more work, a correction or a revamp (COL-10).

A small change with no plan or task list entry gets a completion document too, for visibility, as short as the change allows. The report (COL-8) links to it. It has no status and no `done` step, because there's nothing to track it in.

It covers everything that was implemented, not just the highlights. It's written as COL-22 requires: a walkthrough that shows every change as code, in the language COL-14 requires. It says all of this, laid out however communicates it best:

| Section | What it says |
|---|---|
| Header | The tasks it covers (linked), the plan, the status, the scope with its status (COL-15), and where to review the change (a branch or PR). A small change with no plan or task list entry only needs the last of these. |
| Summary | What works now that didn't before, and whether every acceptance criterion is met. Say any bad news here, not just further down. |
| Acceptance criteria | Every criterion, whether it's met, and the evidence. |
| Criteria not met | For each one: what's missing, why it's impossible or can't be met, what was tried, what it means for users, and what the engineer needs to decide. Write "None" if they're all met. Not having got to it isn't a reason: finish it first. |
| Warnings | Anything the engineer should know before deciding. Always check for a dependency in another workstream that isn't `done` yet (COL-15): name the task, and say what would need redoing if its review changes it. Include a security check that couldn't run, or a finding that's still open (SEC-27). Write "None" if there are none. |
| What was done | Everything that changed, with links to the code. |
| Database | Everything that changed in what's stored, with the schema and the explanation. See below. Write "None" if nothing stored changed. |
| Transactions | Every transaction used instead of one statement in the code the workstream touched, and every one it removed. See below. Write "None" if there are none. |
| Promises | Each promise the work added or changed, with the flow that proves it. Every broken promise the flows found: what was fixed, and what waits for a decision, with the decision it waits for (TEST-18). Write "None" if nothing a caller sees changed. |
| What led to it | The problem, what was found in the code, and how that led to this implementation. |
| Justification | Each decision, the options not chosen and why, and anything that differs from the plan. |
| Impact | What changes, and for whom: callers, data, other code, operations. What stays the same. Breaking changes, risks and known limits. |
| Scenarios | Lots of them. See below. |
| How to check it | The commands to run, so the engineer can see it work for themselves. |
| Sources | Links or references to the code, tests and docs behind the claims above. |

Each item the engineer might answer on its own has a number, with one count for the whole document (COL-21). That covers each criterion not met, each warning, each decision under Justification, and each risk under Impact.

**Make the case for the work, and don't lie.** Write it as if you're selling the implementation to the engineer, with full honesty and transparency. Selling means explaining the work well, never making it look better than it is:
- Every claim is true, and says how far it was checked: tested, read the code or reported (COL-14).
- Problems, shortcuts and limits are stated as plainly as the successes. Leaving one out is a lie, even if every sentence in the document is true.
- If something wasn't tested, say so.

**Scenarios.** Write one for each acceptance criterion, then more for failures, edge cases and anything the engineer might ask "what happens if…?" about. Each one tells the story in order: the situation, what happened before the change, what happens now, why it matters, where it is in the code, and how far it was checked.

**The database.** Stored data is the hardest thing to change back, so a change to it gets its own section, not a passing mention elsewhere. The section has:
- **Every change to the schema:** each table, column, index and key added or removed, and each migration. Show the schema itself, copied from the repo, with a diff for anything that changed and the old definition of anything removed.
- **Why:** what each change is for, and why it's shaped that way, such as why a table has that key.
- **Values that changed meaning:** a column can keep its name and type but hold something different, such as a password hash in a new format. The schema doesn't show this, so name each one, with what it held before and what it holds now.
- **Reads and writes:** which actions read and write the changed tables, and any request that now does more database work.
- **Existing data:** what happens to it, what a deploy must do, what breaks if that step is skipped, and whether there's a way back. On a `greenfield` scope (COL-15), just say there's none to account for.
- **Other stores:** the same for anything else that keeps data, such as a cache's keys and how long they last.

**Transactions.** Every transaction used instead of one statement must be visible to the engineer (DATA-4). The section lists:
- every transaction the workstream added, changed or left in place in the code it touched, with the same three fields as the plan: the use case, the justification and the impact (COL-16);
- every transaction it removed by moving to one statement.

If there are none, the section says so.

**The format.** COL-22 sets the format for completion documents, proposals and plans alike: a standalone HTML page, with every change shown as code with syntax highlighting, visual cues that each do a job, and a check in a browser before it's sent. A completion document also never collapses bad news. The summary, the acceptance criteria and the criteria not met are always visible.

Here's a cut-down example. It's invented, based on `TRF-016` (COL-15). A real one has every section and many more scenarios.

````html
<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Completion: new asset and one-coin acme wallets</title>
<link rel="icon" href="data:,">
<style>
  body { font: 16px/1.6 system-ui, sans-serif; max-width: 60rem; margin: 2rem auto; padding: 0 1rem; color: #1f2328; }
  table { border-collapse: collapse; width: 100%; }
  th, td { border: 1px solid #d0d7de; padding: .4rem .6rem; text-align: left; vertical-align: top; }
  code, pre { font-family: ui-monospace, monospace; font-size: .9em; }
  pre { background: #f6f8fa; padding: .75rem; margin: 0; white-space: pre-wrap; overflow-wrap: anywhere; tab-size: 4; }
  .met { color: #116329; font-weight: 600; }
  .unmet { color: #a40e26; font-weight: 600; }
  .warning { border-left: 4px solid #a40e26; background: #fff5f5; padding: .5rem 1rem; }
  .compare { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); gap: 1rem; }
  .add { background: #dafbe1; color: #116329; }
  .src { color: #59636e; font-size: .85em; margin-bottom: 0; }
  .kw { color: #cf222e; }
  .fn { color: #8250df; }
  .str { color: #116329; }
  .key { color: #0550ae; }
</style>
</head>
<body>
<h1>Completion: new asset and one-coin <code>acme</code> wallets</h1>
<p><strong>Task:</strong> <a href="../tasklist.md#trf-016">TRF-016</a>.
   <strong>Plan:</strong> <a href="../implementation/new-asset-and-acme.html">new-asset-and-acme.html</a>.
   <strong>Status:</strong> <code>completed</code>. <strong>Scope:</strong> <code>API: greenfield</code>. <strong>Review:</strong> branch <code>new-asset-and-acme</code>.</p>

<h2>Summary</h2>
<p>Customers can now hold the new asset on <code>acme</code>, and each <code>acme</code> wallet holds exactly one coin.
   Four of the five acceptance criteria are fully met. The fifth passes in tests, but I couldn't check it against the
   real <code>acme</code>, because it has no test environment.</p>
<p><strong>What I need from you:</strong></p>
<ol>
  <li>Whether the fifth criterion, checked only against a fake <code>acme</code>, is enough as it is, or whether to
      create one real wallet first. See "Criteria not met".</li>
</ol>
<p><strong>How to read the colours:</strong> <span class="met">Met</span> and <span class="unmet">Not fully met</span>
   for the criteria, and <span class="add">+ added</span> for new lines of code.</p>

<h2>Acceptance criteria</h2>
<table>
  <tr><th>Criterion</th><th>Result</th><th>Evidence</th></tr>
  <tr><td>Creating an <code>acme</code> wallet with two assets returns a 400 with a readable message.</td>
      <td class="met">Met</td><td><code>TestCreateWalletHandler_TwoAssetsReturns400</code>; ran it on a local instance</td></tr>
  <tr><td><code>acme</code> accounts are created in the wallet's coin, and nothing sends <code>USD</code>.</td>
      <td class="unmet">Not fully met</td><td>Passes against a fake <code>acme</code> only: see below</td></tr>
</table>

<h2>Criteria not met</h2>
<div class="warning">
  <p><strong><code>acme</code> accounts in the wallet's coin weren't checked against the real provider.</strong>
     <code>acme</code> has no test environment, so the only evidence is a fake that I built from its docs. It records
     every request, and none of them sends <code>USD</code>. If the docs are wrong, the first real wallet fails to
     create and the customer gets an error. You need to decide whether that's acceptable, or whether to create one
     real wallet to confirm it first.</p>
</div>

<h2>What was done</h2>
<h3>Create refuses a second coin once it knows the provider is <code>acme</code></h3>
<p>The check sits right after the provider is chosen, so it only applies to providers that hold one coin per wallet:</p>
<p class="src"><code>internal/core/wallet/create.go:52</code></p>
<pre><code> provider, err := s.router.<span class="fn">Select</span>(req.Network)
 <span class="kw">if</span> err != <span class="kw">nil</span> {
 	<span class="kw">return</span> domain.Wallet{}, err
 }
<span class="add">+if err := ValidateWalletAssetCount(req.Assets, provider.Capabilities()); err != nil {
+	return domain.Wallet{}, err
+}</span>
 <span class="kw">return</span> s.repo.<span class="fn">Create</span>(ctx, <span class="fn">newWallet</span>(req, provider))</code></pre>

<h2>Database</h2>
<p>None. Nothing stored changed: the new rules are checks in the code, and <code>supported_assets</code> keeps its column and type.</p>

<h2>Scenarios</h2>
<h3>A customer asks for an <code>acme</code> wallet with two coins</h3>
<div class="compare">
  <div><h4>Before</h4><pre><span class="kw">POST</span> <span class="fn">/api/v1/wallets</span>
{<span class="key">"supported_assets"</span>: [<span class="str">"USDC"</span>, <span class="str">"USDT"</span>], <span class="key">"network"</span>: <span class="str">"BASE"</span>}

<span class="str">201</span>: the wallet is created with both coins,
which acme can no longer hold</pre></div>
  <div><h4>Now</h4><pre><span class="kw">POST</span> <span class="fn">/api/v1/wallets</span>
{<span class="key">"supported_assets"</span>: [<span class="str">"USDC"</span>, <span class="str">"USDT"</span>], <span class="key">"network"</span>: <span class="str">"BASE"</span>}

<span class="kw">400</span>: <span class="str">"supported_assets must contain exactly one asset
for this provider; got 2. Create one wallet per asset"</span></pre></div>
</div>
<p>The check runs after a provider is selected, so other providers still accept several coins.</p>
<p>Where: <code>internal/core/wallet/routing.go</code> (<code>ValidateWalletAssetCount</code>).
   Checked: tested (<code>TestCreateWalletHandler_TwoAssetsReturns400</code>), and ran it on a local instance.</p>
</body>
</html>
````

**Why.** The engineer is being asked to accept work they didn't write. A report says that the checks passed. It doesn't say what was built, why, or what it affects, and without that the engineer has to work it all out from the diff. An agent also writes confidently whether or not it's right, so the document is only useful if every claim can be trusted and checked.

**How it's checked.** The engineer reads it, and these go back:
- a known problem left out or played down;
- a claim with no source, or that doesn't say how far it was checked;
- something implemented that the document doesn't mention;
- anything the engineer has to ask the meaning of;
- a page that hides bad news in a collapsed section;
- a page that fails COL-22's checks, such as a change with no code or a code block with no syntax highlighting;
- a change to stored data that the Database section doesn't cover;
- a transaction in the touched code that the Transactions section doesn't list;
- too few scenarios to decide from.

## COL-10 · Review, and moving work to `done`

**Rule.** Only the engineer can mark a task or a whole workstream `done`, and only by an explicit instruction after reviewing it. No agent, second reviewer or passing check can do it.

1. The agent finishes the whole workstream (COL-19) and marks its tasks `completed`.
2. Straight away, without being asked, the agent writes the completion document (COL-9) and sends it to the engineer.
3. The engineer reviews the document and the change, and the agent answers any questions.
4. The engineer decides what happens next:

| The engineer decides | What happens |
|---|---|
| It's ready | They say so explicitly, and the agent moves the tasks and the plan to `done`. |
| It needs more work or a correction | The affected tasks go back to `inprogress`. The agent fixes them, marks them `completed` again, and updates the completion document, saying what changed since the last review. |
| It needs a revamp | The affected tasks go back to `inprogress`. The plan is rewritten and approved again (COL-5) before the work restarts. |

Only a clear instruction moves work to `done`:

| The engineer says or does | Move it to `done`? |
|---|---|
| "Mark TRF-016 done." | Yes, that task. |
| "Mark this workstream done." | Yes, every task in it. |
| "Looks good." or "Nice work." | No. If you think they mean `done`, ask. |
| Approves or merges the PR, without saying so | No. |

**What the review covers.**
- Four concerns, in this order:
  1. **Works:** the acceptance tests cover the brief.
  2. **Solves the real problem:** not just the literal request.
  3. **Fits:** follows the standard, with no new structure that wasn't approved.
  4. **Reads well:** the story test (STY-9), naming and comments.
- Lint already covers formatting and mechanical style, so review time goes on the four concerns and on the rules lint can't check ([02](02-style-and-naming.md#what-the-lint-config-covers-and-what-it-cant)).
- Edits to approved acceptance tests are flagged and discussed, never waved through.
- The writing is reviewed too. If the reviewer has to ask what a report, PR description or comment means, or where it is in the code, it goes back (COL-14).
- A second agent with fresh context may review first (COL-20). It gets the diff and the brief, not the implementer's reasoning, so its read isn't anchored on it.

**Merging.** A person approves every merge, and nothing merges unreviewed. Merging and `done` are separate decisions.

**Why.** Finishing work and accepting it are different jobs. Whoever wrote the work can check it against the criteria, but only the person who owns it can say it solves their problem.

## COL-11 · Stop rules

**Rule.** The agent stops and reports, instead of continuing, when:
- **The same failure survives two attempts.** It reports its hypothesis and what it tried, then backs out to the last green commit.
- **Scope grows**, or a requirement conflicts with an invariant. It raises this as soon as it sees it, not at the end.
- **A request breaks the standard.** It says "No, but here's what I can do", and offers the closest compliant option.
- **A scope status doesn't match what it finds**, such as production data behind a scope marked `greenfield` (COL-15). It asks before going on.
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

At every level, `completed` is as far as an agent can take work. Only the engineer makes it `done` (COL-10).

## COL-13 · Keep the standard alive

**Rule.**
- Each correction made twice in review becomes one of these:
  - a lint rule, or a check in the standard's analyzers or `tools/archcheck`, if a machine can check it;
  - a canonical example, if it's about shape;
  - a sentence in the relevant rule, if it needs judgement.
- The code agents copy most (handlers, repositories, tests, `wire.go`) is cleaned first. Agents imitate the code around them, so that code teaches more than any document.
- A fixed set of small practice tasks ("katas") runs against the standard regularly, for example: add a list endpoint, add a guarded transition, add a job. Their review corrections measure whether the standard is working.
- The standard has one named owner, who accepts or rejects changes to it.

**Why.** "Specifying requirements in such detail that a machine can execute them *is* programming" (Clean Code ch 1). A prompt can point at the rules; it can't replace them. Every correction that stays in someone's head will be needed again next week.

## COL-14 · Clear language, practical examples, code sources

**Rule.** Everything written for a person uses clear, approachable language, shows a practical example, and says where in the code it comes from. That covers proposals, plans, questions, reports, completion documents, explanations, findings, PR descriptions and review comments, whether an agent or an engineer wrote them. Proposals, implementation plans and completion documents also follow COL-22: they're HTML walkthroughs that show every change as code.

Write for a smart colleague who is new to this code. If they have to read something twice, or ask what it means, it isn't finished.

**Language.**
- Use short sentences and everyday words, with one idea per sentence.
- Say the main point first, then the detail.
- Explain a technical term in a few words the first time you use it.
- Say what happens, not what category it falls into: "the client can't parse the response", not "a serialization concern".
- A rule or issue ID is a pointer, not an explanation. The sentence must still make sense with the ID removed: "a failed type assertion on the input in `Decode` is now a 400 (BUG-28)", never "BUG-28 is fixed". The same goes for the numbers on reported items (COL-21).

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
| Proposal or plan (COL-5, COL-22) | The problem, today's code and the new code, what it costs, the options not chosen and why, and what you need from the reader |
| Explanation | What happens, step by step and in order, then why it matters, then where it is in the code |
| Finding or bug | What goes wrong, what triggers it, what should happen, whether real code can hit it, where it is, and how far you checked |
| Report (COL-8) | Completed or not completed, the evidence, and the files changed |
| Completion document (COL-9, COL-22) | Everything that was implemented, why, the impact, and many scenarios, with every claim true and checkable |
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
3. Remove every rule ID, issue ID and item number. Does it still make sense?

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

To test a phase, ask: "What can someone do when this phase is finished that they couldn't do before?" If the answer is "nothing yet", it isn't a phase.

### The tracker

The tracker comes first in the file, with these columns:

| Column | What goes in it |
|---|---|
| Task ID | A unique ID, linked to the task's section: a prefix and a number, such as `TRF-016`. It never changes and is never reused. It doesn't contain the phase number, because tasks move between phases. |
| Title | What the task delivers, in a few plain words. |
| Phase | The phase number and name, such as `3 · Wallets`. Rows stay in phase order. |
| Scope | Where the work lands, and that scope's status: `API: greenfield`, `Admin: existing`, `Web: unconfirmed`. A scope can be anything the project has, such as `Worker` or `SDK`. Usually one scope per task, so each can be planned and finished on its own. |
| Priority | `P0`: the project can't launch without it. `P1`: first work after launch. `P2`: useful, with no date. |
| Dependencies | The IDs of the tasks above it that must be finished first, or `None`. |
| Status | One of the seven statuses below. |

For example:

```markdown
| Task ID | Title | Phase | Scope | Priority | Dependencies | Status |
|---|---|---|---|---|---|---|
| [TRF-012](#trf-012) | Create and manage wallets through the Product API | 3 · Wallets | API: greenfield | P0 | TRF-011 | `done` |
| [TRF-013](#trf-013) | Manage wallets through the Admin API | 3 · Wallets | Admin: greenfield | P0 | TRF-012 | `done` |
| [TRF-014](#trf-014) | Manage wallets on the admin dashboard | 3 · Wallets | Web: existing | P0 | TRF-013 | `done` |
| [TRF-015](#trf-015) | Test the wallet feature end to end | 3 · Wallets | API: greenfield, Admin: greenfield, Web: existing | P0 | TRF-014 | `done` |
| [TRF-016](#trf-016) | Add a new asset and one-coin `acme` wallets | 3 · Wallets | API: greenfield | P0 | TRF-015 | `inprogress` |
| [TRF-017](#trf-017) | Send funds through the Product API | 4 · Outflows | API: unconfirmed | P0 | TRF-015 | `todo` |
```

**Dependencies only point up the list.** A newer task, further down the list, can depend on older tasks above it. An older task never depends on a newer one. If a task turns out to need one below it, the order is wrong, so move the needed task up.

A dependency must be `completed` before work starts on the task that needs it. If it's in another workstream (COL-19) and isn't `done` yet, that's a warning, not a blocker. The agent carries on, and flags it under "Warnings" in its report (COL-8) and completion document (COL-9), saying what would need redoing if the review changes that work.

### Statuses

There are seven, and no others. Anything else that needs saying goes in the task's section.

| Status | Meaning | Also write in the task's section |
|---|---|---|
| `todo` | Not started. | |
| `inprogress` | Someone is working on it now. Work that's built but not yet tested stays here (COL-8). | A link to the implementation plan |
| `completed` | Fully implemented, tested, verified when possible, and meets the requirement (COL-8). The agent marks this on its own, without asking. | |
| `done` | The engineer reviewed the work and explicitly said it's done (COL-10). Nobody else can set this. | |
| `blocked` | It can't continue until something outside the task happens. | What's blocking it, and who can clear it |
| `deferred` | Valid work, put off on purpose. | What would restart it |
| `cancelled` | No longer needed. The row stays, so its ID isn't reused. | Why, and who decided |

### Scope status

Each scope in a task's row has a status of its own, because one task can touch a new API and a dashboard people already use. It says what the work has to protect, while the task's status says how far the work has got. There are three, and no others.

| Scope status | Meaning | What the work does |
|---|---|---|
| `greenfield` | Nothing here has to be preserved: no real users, no deployment that must keep running, and no data or infrastructure that must survive. A test environment that can be wiped and rebuilt still counts. | Change things directly. Don't add what only existing users would need: data migrations, backfills, compatibility code, old and new versions side by side, deprecation periods or gradual rollouts. Renaming a field or enum value is a plain edit. A schema change is still a migration file (DATA-11). |
| `existing` | Something real depends on it: users, API callers, a running deployment, infrastructure or stored data. If only part of it exists, it's still `existing`, and the task's section says which part. | The task's section says what exists, and the plan says how it keeps working (COL-16). |
| `unconfirmed` | The engineer hasn't said which of the two it is yet. | Ask. Nothing gets planned or changed until the engineer answers. The task list itself can be written and reviewed with rows like this. |

**The engineer sets it, when the task list or a task is written.** Nothing fixes a scope or its status ahead of time for a whole project or repo. The agent can't set it, because the repo can't show it. A project can have migrations and a production deploy file with no real users, and a service that looks new can already have callers. A doc that says "no live users" may be out of date. All of that is evidence for a proposal, not a confirmation.
- When an agent writes a task list or adds a task, every scope the engineer hasn't given a status goes in as `unconfirmed`, even if the agent has a good guess. It never writes `greenfield` or `existing` on an assumption.
- It raises them straight away, all in one message, with what it found and a proposed answer (COL-3). One sentence from the engineer can confirm many rows, such as "All of this is greenfield".
- A scope with no status, such as in a task list written before this rule, counts as `unconfirmed`.

Here's an invented example of the question:

```text
I've marked API and Web as `unconfirmed` in the task list, because the repo can't show whether anything
real depends on them. The answer changes the work: if something does, each task needs a plan to keep it
working. If nothing does, we change things directly.

- API: I propose `existing`. deploy/production.yaml deploys it, but that doesn't show real callers or data.
- Web: I propose `greenfield`. I found no deployment config for it.

Does anything real depend on each one? If so, what has to keep working?
```

**It can change.** The first real user, deployment or stored data makes every later task in that scope `existing`. The engineer says when that happens, and the agent updates the rows that haven't started. If the agent finds something that doesn't match a status, such as production data behind a `greenfield` scope, it stops and asks (COL-11).

### The task section

Each task has one section, with these fields. Add more if the task needs them.

| Field | What it says |
|---|---|
| Problem statement | What's wrong or missing today, and who it affects. |
| Goal | What will be true when the task is finished. |
| Requirements | What the work must include, and any limits on how it's done. |
| Acceptance criteria | Statements someone can check. They're the "Completed when" of the brief (COL-2), and they become the tests (COL-4). |
| Breaking changes | What callers must change, or "None". |
| What exists | Only for a scope marked `existing`: who and what depends on it now, and what must keep working or move. For example: "The admin team uses the dashboard daily. Its login and wallet list must keep working during this change." (invented) |
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
hold exactly one asset. Nothing calls the API yet, so no caller has to change.

**Implementation plan.** [implementation/new-asset-and-acme.html](implementation/new-asset-and-acme.html)
```

The `<a id>` line keeps the tracker's link working if the title changes.

### Keeping it up to date

- Change the status when the work changes, not at the end of a phase.
- New work goes on the list when it's found, as `todo`, in the phase that needs it.
- The engineer decides what's on the list: adding, cancelling or deferring a task, and changing a priority, an acceptance criterion or a scope status. The agent proposes these and may move a task's status as the work goes, up to `completed`. Only the engineer's instruction moves it to `done` (COL-10).
- Don't keep counts by hand. Anyone who needs a count can get it from the table.

A scope status exists because a wrong guess costs real work either way. An agent that assumes existing users adds migrations and compatibility code that nobody needs. An agent that assumes there are none can break a real caller or lose real data.

**Why.** One list means nobody has to piece the project together from chat and memory. Phases by feature mean that stopping early still leaves something that works. Keeping each fact in one place stops the list contradicting itself, which real task lists do (tested: a script compared each tracker with its sections):

- The tracker says `done`, and the task's section says `in-progress`.
- The tracker says `done`, and the task's section says `todo`.
- The hand-kept summary says 14 tasks are `done` and 7 are `code-complete`. The table has 26 `done` and no `code-complete`.
- The ID says phase 3, but the task is in phase 2. The phase number in the ID is wrong for 33 of one tracker's 40 rows.

Without one format, each project's list drifts its own way. Across three codebases, the file had two names (`tasklist.md`, `PROJECT_TASKS.md`), the plans folder had two spellings (`implementation/`, `implementations/`), each list added its own statuses (such as `revalidate` and `code-complete`), and only one tracker linked to its sections.

**How it's checked.** A person checks it in review, using the phase test above. A script could check the mechanical parts: every row has a section, every dependency exists and is above its task, every status is one of the seven, and every scope has one of the three scope statuses. That script isn't built yet. Only the engineer can tell whether a scope status is true.

## COL-16 · The implementation plan

**Rule.** Each task being worked on is detailed in an implementation plan: an HTML page in the `implementation/` folder, beside `tasklist.md`, written as COL-22 requires. It says what the work is, how it will be done and why, with clear examples of what the issue is and what needs to change, shown as code. A plan can cover one task or several, as the engineer instructs, and all the work it covers is one workstream (COL-19). The engineer approves it before implementation starts (COL-5). If the engineer first chose between designs in a proposal, the plan links to it and carries the decision (COL-22).

Plans are kept up to date until the engineer marks their work `done`. After that, a plan is a record of finished work, and it stays until the engineer deletes it (COL-18).

A plan written in Markdown before COL-22 stays as it is, unless the engineer asks for it to be converted. Every new plan is HTML.

| | Task list | Implementation plan |
|---|---|---|
| Answers | What needs doing, and in what order? | How and why will this task get done? |
| Covers | The whole project, or a large part of it | One or more tasks |
| Lasts | As long as the project | Until the engineer deletes it |
| Where | `tasklist.md` | `implementation/<topic>.html` |

### What a plan contains

| Section | What it says |
|---|---|
| Header | The tasks it covers (linked), its status, the repos it touches, its proposal if there was one, and a link to the completion document once there is one (COL-9). |
| Summary | What's being done and why, in a few sentences, ending with what you need from the engineer (COL-22). |
| Sub-task table | Every sub-task needed to finish the plan end to end (COL-17). |
| The issue today | How the code behaves now, with the code copied from the repo, its source, and an example of what goes wrong. |
| What changes and what doesn't | The behaviour that changes, shown as code (COL-22), and what callers can rely on staying the same. |
| Keeping what exists working | Only for a scope marked `existing` (COL-15): how data moves, what callers must change, the order of the deploy, and the way back. |
| Decisions | Each choice: the options, the one taken and why. Open ones are the engineer's to answer (COL-3). |
| Transactions | Only for a plan that adds or changes a write: each transaction it will use instead of one statement, with the use case, the justification and the impact. See below. |
| Promises | Only for a plan that changes what a caller sees: each promise it adds or changes, from the API description, the feature document or the design, with the flow that will prove it (TEST-18). |
| Sub-task sections | One section per row of the table, in the same order (COL-17). |
| Risks | What could go wrong, and what we'd do about it. |
| Out of scope | What this plan won't do, and where that work goes instead. |
| Completion criteria | Each sub-task's "Completed when" line, tied back to the task's acceptance criteria. Link to the criteria rather than copying them. |

### Transactions

Every transaction used instead of one statement must be visible to the engineer before work starts (DATA-4). A plan that adds or changes a write lists each one. Each entry gives:
- **the use case**;
- **the justification:** which database limit, or which boundary writes, and how each one passes DATA-4's three tests: shared, owned and atomic;
- **the impact:**
  - how many round trips each call makes;
  - how busy the path is;
  - the risk of collisions and retries;
  - what one statement would save.

A transaction with no entry means the plan isn't ready for approval. If the plan uses none, the section says so.

Here's an entry. It's invented.

**`ordering.Place`**
- **Justification:** boundary writes beside `orders.Place`, which writes the order, its items and its event in one statement.
  - `audit.Record`: one audit writer serves every use case (shared), and it owns the actor fields and what's redacted (owned). An order placed without its record is an unaudited change (atomic).
  - `jobs.Enqueuer`: the queue lives in the same database, and its library owns the job table (shared, owned). An order without its fulfilment job is lost work (atomic).
- **Impact:**
  - 5 round trips per call: begin, the statement, the audit insert, the job insert and the commit.
  - The busiest write path, at about 40 calls a second at peak.
  - The order and its items are new rows, so other writes rarely collide with them. Every use case writes to the job table, so a serialization failure there retries the whole closure.
  - One statement would save 4 round trips, but it would copy the audit writer's rules and the queue's columns into the order query.

### Keeping it up to date

- If the approach changes, change the plan first. If that changes what the engineer approved, stop and ask (COL-11).
- The task's section links to the plan, and the plan links back to its tasks.

### When the work is `done`

A plan describes the work, not the system. Once the engineer marks the work `done`, the code and the feature docs are the truth.

1. Move anything that must last into the permanent docs: behaviour into the feature docs, and vendor facts into the vendor doc.
2. Set the status in the plan's header to `done`. A plan marked `done` describes past work, so don't follow it as instructions.
3. Leave the plan in the folder, and leave the link in the task's section (COL-18).

A plan that covers several tasks is finished when the engineer has marked the last of them `done`.

**Why.** Keeping the detail in the plan keeps the task list short enough to scan. The plan also shows the engineer the approach before any code exists, which is the cheapest point to change course. Marking a finished plan matters because a plan left in the folder can look like current instructions, and agents follow what they find.

**How it's checked.** A person checks it in review. These go back:
- a task marked `inprogress` with no linked plan;
- a plan for a scope that's still `unconfirmed`;
- a plan whose tasks are all `done`, but whose header doesn't say so;
- a plan that adds or changes a write, with a transaction it doesn't list, or an entry without its justification or impact;
- a plan that changes what a caller sees, without the promises it adds or changes;
- a plan that fails COL-14's three tests, or COL-22's checks.

## COL-17 · The plan's sub-task table

**Rule.** Every implementation plan has one table of all the sub-tasks needed to finish it end to end: the code, the tests, any migration, the docs, and the final check that it all works. The table comes straight after the plan's summary. It's an HTML table in the same format as the task list's tracker (COL-15): the same columns, the same seven statuses, rows in phase order, and each row linked to a section further down the plan.

Here's the table from the plan for `TRF-016`:

```html
<table>
  <tr><th>Sub-task ID</th><th>Title</th><th>Phase</th><th>Scope</th><th>Priority</th><th>Dependencies</th><th>Status</th></tr>
  <tr><td><a href="#st-1">ST-1</a></td><td>Map the new asset to <code>acme</code>'s code</td><td>1 · New asset</td>
      <td>API: greenfield</td><td>P0</td><td>None</td><td><code>completed</code></td></tr>
  <tr><td><a href="#st-2">ST-2</a></td><td>Accept the new asset on <code>ETH</code> only</td><td>1 · New asset</td>
      <td>API: greenfield</td><td>P0</td><td>ST-1</td><td><code>completed</code></td></tr>
  <tr><td><a href="#st-3">ST-3</a></td><td>Enforce one asset per <code>acme</code> wallet</td><td>2 · One-coin wallets</td>
      <td>API: greenfield</td><td>P0</td><td>None</td><td><code>inprogress</code></td></tr>
  <tr><td><a href="#st-4">ST-4</a></td><td>Create <code>acme</code> accounts in the wallet's coin</td><td>2 · One-coin wallets</td>
      <td>API: greenfield</td><td>P0</td><td>ST-3</td><td><code>todo</code></td></tr>
  <tr><td><a href="#st-5">ST-5</a></td><td>Check every acceptance criterion on a running service</td><td>3 · End to end</td>
      <td>API: greenfield</td><td>P0</td><td>ST-2, ST-4</td><td><code>todo</code></td></tr>
</table>
```

Title and Scope mean what they mean on the task list, and a sub-task's scope status always matches its task's. The others work like this in a plan:

| Column | In a plan |
|---|---|
| Sub-task ID | A short, temporary ID such as `ST-3`, linked to its section. It stays the same while the plan is in use, and another plan can use the same ID. |
| Phase | A stage of this plan, such as `2 · One-coin wallets`. If the plan covers several tasks, use one phase per task, named after the task: `1 · TRF-016`. |
| Priority | The priority of the task it belongs to. |
| Dependencies | The IDs of the sub-tasks above it that must be `completed` first, or `None`. As on the task list, a row never depends on one below it. |
| Status | One of the seven statuses. `completed` means COL-8's four points, measured against the sub-task's own "Completed when" line. A sub-task only moves to `done` along with its task, on the engineer's instruction (COL-10). For `blocked`, `deferred` and `cancelled`, say why in the sub-task's section. |

**The IDs are temporary.** They only mean something inside the plan. Don't use them in `tasklist.md`, code, tests, commit messages or permanent docs, because they'd point at nothing once the plan is deleted. Say what the work is instead (COL-14). If a sub-task turns out to be a task of its own, propose adding it to the task list (COL-15).

To test the table, ask: "If every row were `completed`, would every acceptance criterion of the plan's tasks be met?" If not, something is missing from it.

### The sub-task section

Each row has one section, in the same order as the table. It says why the work is needed, what changes, which tests, and when it's completed, and it's small enough to end with tests passing (COL-6). It walks through the change and shows it as code, as COL-22 says. Like a task's section, it doesn't repeat what's in the table.

Here's the section for `ST-3`, cut down where it says `…`. The styles are the ones in COL-22's example.

````html
<section id="st-3">
<h3>ST-3 · Enforce one asset per <code>acme</code> wallet</h3>

<p><strong>Why.</strong> <code>acme</code> now opens one account per coin, and each wallet has one provider account.
   So an <code>acme</code> wallet can only hold one coin.</p>

<h4>Today, nothing stops a request for two coins</h4>
<p>Create picks the provider, then saves every asset in the request. Nothing counts them first:</p>
<p class="src"><code>internal/core/wallet/create.go:52</code></p>
<pre><code>provider, err := s.router.<span class="fn">Select</span>(req.Network)
<span class="kw">if</span> err != <span class="kw">nil</span> {
	<span class="kw">return</span> domain.Wallet{}, err
}
<span class="kw">return</span> s.repo.<span class="fn">Create</span>(ctx, <span class="fn">newWallet</span>(req, provider))</code></pre>

<h4>A request for two coins is refused, and the caller is told what to do instead</h4>
<div class="compare">
<div><p>Today</p>
<pre><code><span class="kw">POST</span> /api/v1/wallets
{<span class="key">"supported_assets"</span>: [<span class="str">"USDC"</span>, <span class="str">"USDT"</span>], <span class="key">"network"</span>: <span class="str">"ETH"</span>}

→ 201 Created, with a coin acme can't hold</code></pre></div>
<div><p>After</p>
<pre><code><span class="kw">POST</span> /api/v1/wallets
{<span class="key">"supported_assets"</span>: [<span class="str">"USDC"</span>, <span class="str">"USDT"</span>], <span class="key">"network"</span>: <span class="str">"ETH"</span>}

<span class="chg">→ 400 Bad Request
"supported_assets must contain exactly one asset for this
provider; got 2. Create one wallet per asset"</span></code></pre></div>
</div>

<h4>The check runs after the provider is chosen, so other providers still accept several coins</h4>
<p>A new <code>OneAssetPerWallet</code> flag on <code>Capabilities</code> says which providers hold one coin per
   wallet, and <code>acme</code> sets it in <code>client.go</code>. Services check the flag, not the provider's name.
   Create checks it here:</p>
<p class="src"><code>internal/core/wallet/create.go:52</code></p>
<pre><code> provider, err := s.router.<span class="fn">Select</span>(req.Network)
 <span class="kw">if</span> err != <span class="kw">nil</span> {
 	<span class="kw">return</span> domain.Wallet{}, err
 }
<span class="add">+if err := ValidateWalletAssetCount(req.Assets, provider.Capabilities()); err != nil {
+	return domain.Wallet{}, err
+}</span>
 <span class="kw">return</span> s.repo.<span class="fn">Create</span>(ctx, <span class="fn">newWallet</span>(req, provider))</code></pre>

<p>The new function, in full:</p>
<p class="src"><code>internal/core/wallet/routing.go</code> (new)</p>
<pre><code><span class="kw">func</span> <span class="fn">ValidateWalletAssetCount</span>(assets []domain.KnownAsset, caps domain.Capabilities) <span class="kw">error</span> {
	<span class="kw">if</span> caps.OneAssetPerWallet &amp;&amp; <span class="fn">len</span>(assets) != <span class="num">1</span> {
		<span class="kw">return</span> errx.<span class="fn">NewKindf</span>(__STAMP__, errkinds.ErrKindInvalidInput,
			<span class="str">"supported_assets must contain exactly one asset for this provider; got %d. Create one wallet per asset"</span>, <span class="fn">len</span>(assets))
	}
	<span class="kw">return</span> <span class="kw">nil</span>
}</code></pre>
<p>Checked: written, not compiled yet.</p>

<h4>Enabling another coin, or removing the only one, is refused with a 409</h4>
<p>…the same steps for enable and delete: today's code, the caller before and after, the diff…</p>

<h4>Tests</h4>
<p><code>TestCreateWalletHandler_MultiAssetCreatesAllAssetRows</code> now expects a 400. These are new:</p>
<p class="src"><code>internal/core/wallet/routing_test.go</code> (new)</p>
<pre><code><span class="kw">func</span> <span class="fn">TestValidateWalletAssetCount</span>(t *testing.T) {
	oneCoin := domain.Capabilities{OneAssetPerWallet: <span class="kw">true</span>}
	two := []domain.KnownAsset{domain.AssetUSDC, domain.AssetUSDT}
	tests := []<span class="kw">struct</span> {
		name    <span class="kw">string</span>
		assets  []domain.KnownAsset
		caps    domain.Capabilities
		wantErr <span class="kw">bool</span>
	}{
		{<span class="str">"acme, one coin"</span>, two[:<span class="num">1</span>], oneCoin, <span class="kw">false</span>},
		{<span class="str">"acme, two coins"</span>, two, oneCoin, <span class="kw">true</span>},
		{<span class="str">"another provider, two coins"</span>, two, domain.Capabilities{}, <span class="kw">false</span>},
	}
	<span class="cm">// …</span>
}</code></pre>
<p>Two handler tests are added too: enabling a different coin returns a 409, and deleting the coin returns a 409
   with the asset row still there.</p>

<p><strong>Completed when.</strong> The new tests pass, and <code>task test</code> passes.</p>
</section>
````

### Keeping it up to date

- Change a sub-task's status when the work changes, and record what changed from the plan in its section.
- New work goes in the table as `todo` when it's found. If it changes what the engineer approved, stop and ask (COL-11).
- Only the engineer defers or cancels a sub-task, as with tasks (COL-15).
- A task can't be `completed` while any of its sub-tasks is `todo`, `inprogress` or `blocked`.

**Why.** A list of steps in prose doesn't show how far the work has got, what's waiting on what, or whether anything was left out. A table shows all three at a glance, and using the task list's format means there's only one format to learn. The IDs are temporary because sub-tasks only matter while the plan is in use. Keeping them off the task list keeps that list short enough to scan.

**How it's checked.** A person checks it in review. No script checks it yet. These go back:
- a plan with no table, or with different columns or statuses;
- a row with no section, or a section with no row;
- a table that fails the test above;
- a task marked `completed` with a sub-task still `todo`, `inprogress` or `blocked`;
- a sub-task ID in code, a commit message or a permanent doc.

## COL-18 · Only the engineer deletes a plan

**Rule.** Never delete an implementation plan unless the engineer has explicitly told you to delete it. This holds even when all its tasks are `done`, and however old it is. The same goes for a proposal or a completion document. The engineer can delete one whenever it's out of date or no longer needed, and nothing depends on it, so a link left behind does no harm. Moving it out of the folder or emptying it counts as deleting it. Don't add a script, hook or CI job that deletes plans either.

| The engineer says | Delete the plan? |
|---|---|
| "Delete the plan for TRF-016." | Yes, that plan only. |
| "Delete every finished plan." | Yes, the plans marked `done`. |
| "Mark TRF-016 done." | No. Marking work `done` doesn't delete its plan. |
| "Clean up the repo." | No. If you think they mean the plans, ask. |

When the engineer does say to delete a plan, delete the file and remove the links to it.

**Why.** A plan holds things the code can't show, such as the options not chosen and why, and the engineer decides when those are no longer needed. Deleting can also lose a plan for good. Git history only keeps a plan that was committed, and a squash merge drops a plan that was added and deleted on the same branch.

**How it's checked.** A person checks it in review: a change that deletes a plan goes back, unless the engineer asked for it. No script can tell whether they asked.

## COL-19 · A workstream goes to the engineer whole

**Rule.** A workstream is all the work one implementation plan covers: one task or several. A small change with no plan or task list entry (COL-5) isn't a workstream. It gets a completion document for visibility (COL-9), and it has no `done` step. Everything in it is implemented end to end and `completed` before the agent sends it to the engineer. Don't implement it in bits and ask for each bit to be marked `done`.

There are two exceptions:
- **The engineer explicitly asks** to review a part first.
- **The agent thinks an item must be reviewed and `done` before another** in the same workstream or phase. That should be rare. It stops and asks before going on, saying which item, why it can't wait, and what could go wrong if it does.

This only limits asking for `done`. Questions and problems don't wait for the end: an ambiguity (COL-3) or a stop rule (COL-11) is still raised as soon as it comes up.

For a plan that covers `TRF-016` and `TRF-017`:

| Where the work is | What the agent does |
|---|---|
| `TRF-016` is `completed`, and `TRF-017` hasn't started | Keeps going. It doesn't send `TRF-016` for review. |
| `TRF-016` fails the same way after two attempts | Stops and reports it (COL-11). That isn't asking for `done`. |
| Both are `completed` | Writes the completion document for both straight away, without being asked, and sends it (COL-9). |
| The engineer says "Let me review `TRF-016` first." | Sends `TRF-016`, with a completion document for it. |

**Why.** Part of a workstream can't show that the whole thing works end to end, so the engineer would be judging it without the evidence that matters most. Each extra review also costs their time, and work built on a part they later change has to be redone.

**How it's checked.** The engineer checks it: a request to mark part of a workstream `done`, with no instruction to do so, goes back.

## COL-20 · At most one subagent, and only when justified

**Rule.** A working session may run one subagent at a time besides itself, and only when it's justified. This covers anything that starts another agent, such as a fork, a background agent or a workflow, so a tool that starts several at once breaks the limit. Another can start once the first has finished and its result has been read. Only the engineer's explicit instruction raises the limit, and only for that request.

A subagent is justified only when one of these is true. The agent says which, in one line, before it starts one:
- **It needs a fresh read:** a review or acceptance tests that mustn't be shaped by the implementer's reasoning (COL-4, COL-10).
- **It's a side job the session shouldn't wait on,** such as a long security check (SEC-27), while the session gets on with other work.
- **It means reading across many files,** and only the conclusion is needed.

| The situation | Use a subagent? |
|---|---|
| Reading three files you know the names of | No. Read them. |
| Running `task check` | No. Run it. |
| Finding every caller of a function | No. Search for it. |
| Reviewing a diff with fresh context before sending it | Yes, one. |
| Splitting a feature between four agents to finish faster | No. Speed alone isn't a reason. |
| An audit with five groups to check | One group at a time, in the session or in one subagent. |
| The engineer says "Review this with three agents." | Yes, for that request. |

**Why.** A subagent knows only what its prompt says, and the session sees only the summary it sends back, so detail is lost in both directions. Its mistakes come back sounding as confident as its findings. Each one costs tokens, several at once can edit the same files, and nobody can follow several streams of work at the same time.

**How it's checked.** A person checks it in review, from the session and the report. No script checks it yet.

## COL-21 · Number every item, and never use the number alone

**Rule.** When you report findings, feedback, decisions, concerns, questions, proposals, risks or warnings, give each one a number. This applies wherever you write for a person (COL-14): replies in a chat or session, reports, proposals, plans, completion documents, audits, review comments and PR descriptions. An item is anything the engineer might want to answer, accept, reject or ask about on its own.

**How to number.**
- Use plain numbers: 1, 2, 3.
- A conversation keeps one count from start to end, and so does each document. Never start again at 1 partway through. If a reply lists three findings and then two questions, they're 1 to 5, not 1 to 3 and 1 to 2.
- A number belongs to one item for as long as the conversation or document lasts. When you report the items again, they keep their numbers. An item that's resolved, withdrawn or found to be wrong keeps its number too, and the number is never given to another item.
- When an updated document goes back to the engineer (COL-10), every item still in it keeps its number, and new items carry on from the highest number used.
- To mention a document's item in a chat, use the document's number and name the document: "4 in the completion document (…)".
- Something that already has an ID of its own, such as a task (COL-15) or a sub-task in a plan (COL-17), keeps that ID and gets no number.
- Number only the items. Any other list in the same reply or document, such as next steps or a suggested order of work, uses bullets, so a number never means two things.

**The numbers are for the engineer, not the writer.** They let the engineer point at an item without retyping it: "fix 4", "agree with 2, not 5", "what do you mean in 7?". They're never a shorthand for the writer.
- Every time you mention an item, in the first reply or the tenth, say in plain words what it is, next to its number.
- That includes a range or a list of items, and one item pointing at another. Write "the fixes for 1 to 3 (the crash on a short last page, the crash on a negative size and the offset overflow) are small", never "fixes 1 to 3 are small". Write "this is also why 6 (database errors reach the client) happens", never "this is how 6 happens".
- The sentence must still make sense with the number removed, as COL-14 says for every ID.
- When the engineer uses bare numbers, answer with each number and what it is.

**Don't.** The engineer has to scroll back to work out what each number means:

> Fixed 3 and 7. 5 is still open, and 9 needs your decision.

**Do.** Each number comes with what it is:

> Fixed 3 (the outbox relay drops errors from `tick` without logging them) and 7 (a double quote in an error message breaks the JSON response). Still open: 5 (a restart leaves an export stuck in `processing`). Needs your decision: 9 (whether the KYC list should reject a filter it can't parse, instead of ignoring it as it does today).

A reply that reports two kinds of item keeps one count:

```text
Findings:
1. The outbox relay drops every error from tick, so a failed read never shows in the logs.
   internal/adapters/kafka/control/kafka.go:298. Checked: read the code.
2. A double quote in an error message breaks the JSON response, so the client can't parse it.
   internal/core/httperr/encoder.go:45. Checked: tested.

Questions:
3. Should the relay stop after repeated failures, or keep retrying? The answer decides whether a
   long Kafka outage pages someone. I propose: keep retrying, and log each failure.
```

| The situation | What to do |
|---|---|
| A reply has three findings, then two questions | Number them 1 to 5. |
| The next reply covers the same findings and one new one | The old ones keep 1 to 3. The new one is 6. |
| Finding 2 turns out to be wrong | Say so, with what it was: "2 (…) was wrong, because …". Never give 2 to another item. |
| The engineer says "fix 4" | Fix it, and report "Fixed 4 (what it was)", never "Fixed 4". |
| The engineer says "agree with 1 and 3" | Say what you'll do about 1 (what it is) and 3 (what it is). |
| A reply ends with what to do next | Use bullets, and say what each item is: "Answer 15 (whether the endpoint is `existing` or `greenfield`) first", never "Answer 15 first". |
| One item depends on another | Say what the other one is: "Depends on 17 (the maximum page size)", never "Depends on 17". |
| The report's "Warnings" or "Security check" line points at items | Say what each is: "Warnings: 19 (the security check hasn't run) and 20 (`task check` can't run here)", never "Warnings: 19 and 20". |
| A list of tasks from the task list | Use their task IDs. Don't number them again. |

**Why.** Reviewing work means answering items one at a time, and a number lets the engineer do that in a word. Two items with the same number make "fix 3" ambiguous, so each number has to mean one thing for the whole conversation. But a number is a pointer, not a description. A reply made of bare numbers sends the engineer back through the conversation to decode it, which costs more than the numbers saved. If they decode one wrongly, they approve or reject the wrong thing.

**How it's checked.** Before sending anything, the writer finds every item number in it, including in summary lines and next steps, and checks that each one says what the item is. A person checks it too, because no linter can. In review (COL-10), a report or document goes back if it has an unnumbered item, a number used for two items, or a number with nothing next to it saying what it is. In a chat, the engineer points it out and the agent rewrites the reply.

## COL-22 · Proposals, plans and completion documents are walkthroughs

**Rule.** The engineer makes decisions from three documents: a proposal (which design to build), an implementation plan (whether to start) and a completion document (whether the work is `done`). Each one teaches the reader the problem, the change and the reasoning, so they can decide from the document alone, without a round of questions to work out what it means. All three:
- read like a clear technical blog post: a walkthrough in plain words, not a changelog of edits or a research paper;
- show every change as code: the code today, what a caller sees before and after, the change as a diff, the new code and its tests;
- use visual cues that each do a job: before and after, colour with a legend, sample rows, walkthroughs and diagrams;
- are standalone HTML pages, checked in a browser before they're sent.

COL-14's language rules apply to every sentence in them, and COL-21's numbering applies to every item.

| Document | Where | Written when | The engineer decides |
|---|---|---|---|
| Proposal | `implementation/<topic>-proposal.html` | The engineer must choose between designs before a plan can be approved, such as the shape of a new table (COL-5) | Which option to build |
| Implementation plan (COL-16) | `implementation/<topic>.html` | Before work on a task starts | Whether to start the work as planned |
| Completion document (COL-9) | `completion/<topic>.html` | As soon as a workstream is `completed` | Whether it's `done`, or needs more work, a correction or a revamp |

**A proposal** says what COL-14 asks of one: the problem, today's code and the new code, what each option costs, the options not chosen and why, and what you need from the reader. Its header links to its tasks, and to the plan once there is one. It also gives the proposal's status: waiting for a decision, or the option the engineer chose. Once they choose, the plan carries the decision, and the proposal stays as the record of why (COL-18). A proposal small enough for one reply, like the one in COL-14, can stay in the chat.

### Write a walkthrough, not a changelog

The reader should finish the document understanding the work, not just knowing which lines will change.
- **Tell each change in order:** what happens today, why that's a problem, what changes, what it looks like afterwards, and what it costs.
- **Put the decision first.** The summary comes straight after the header, and it ends with what you need from the engineer: an approval, a choice or an answer, each one numbered (COL-21). The summary and any bad news are never collapsed.
- **Make each heading say its point**, so the headings alone read as a summary: "A request for two coins is refused", not "Validation changes".
- **Explain before you show.** Before each code block, table or diagram, say in a sentence what to look at. After it, say what it showed, unless that's obvious.
- **Show the reasoning.** Each decision gives the options, the one taken, why, and what it costs. An option that's dropped stays in the document, greyed out, with why it was dropped and what replaced it. It keeps its number (COL-21).
- **Say how far each claim was checked:** tested, read the code, or reported (COL-14).
- **Let it be as long as it needs to be.** The length comes from examples and reasoning, never from saying the same thing twice.

**Don't.** A list of edits. It's quick to read, but the engineer can't approve it without asking why each edit is needed and what a caller will see:

> **ST-3 · One asset per wallet**
> - Add `OneAssetPerWallet` to `Capabilities`.
> - Validate the asset count in create.
> - Return 409 on enable and delete.
> - Update the tests.

**Do.** Walk through it, so the engineer can decide from this alone. Here's how the same section starts:

> **An `acme` wallet can only hold one coin, but nothing stops a request for two.**
>
> `acme` now opens one account per coin, and each wallet has one provider account. So a wallet with two coins has a coin that `acme` can't hold. Today, this request succeeds:
>
> ```http
> POST /api/v1/wallets
> {"supported_assets": ["USDC", "USDT"], "network": "ETH"}
>
> → 201 Created
> ```
>
> After this change, it's refused, and the message tells the caller what to do instead:
>
> ```http
> → 400 Bad Request
> "supported_assets must contain exactly one asset for this provider; got 2. Create one wallet per asset"
> ```
>
> The check runs after the provider is chosen, so other providers still accept several coins.

[COL-17](#the-sub-task-section) shows the whole section as it appears in a plan.

### Show every change as code

The prose says why. The code shows exactly what will change, and it's what the engineer checks the work against. For each change, show these in order, leaving out any that don't apply:

1. **The code today**, copied from the repo, with its source above it: `internal/core/wallet/create.go:52`.
2. **What a caller sees**, before and after: the request, then the response or error today and after the change.
3. **The change as a diff**, with removed lines in red and added lines in green.
4. **The new code in full:** each new function, type or field, not a sketch of it.
5. **The tests**, as code, so the engineer can judge whether they prove the acceptance criteria.
6. **The SQL**, when stored data changes: the schema, copied from the migration or from the database's own output such as `SHOW CREATE TABLE`, with the changes marked, and the statements that read and write it. A completion document's Database section covers the rest (COL-9).

For all of it:
- Today's code is copied, never paraphrased or tidied up.
- Proposed code is real Go in the project's style, following this standard, not pseudo-code. Where a stamp belongs, write `__STAMP__` (ERR-1). Say how far it was checked: compiled and tested on a scratch branch, or written but not compiled yet.
- Mark the lines that matter, so the reader doesn't have to hunt for them.
- Cut a long block down to the lines needed to follow the change, mark each cut with `…`, and give the file's path.
- Every block has syntax highlighting for its language, including Go, SQL, JSON, HTTP, shell and command output. The colours stay inside the page: use any highlighter while writing it and keep the coloured HTML, or put the highlighter's script in the page.
- Code sources are written as text, `path:line` (COL-14), because a browser can't open a local file at a line.

### Visual cues

HTML can show what Markdown can't. Each cue below has a job. Use the ones this work needs, and none for decoration.

| Cue | What it's for |
|---|---|
| Before and after | Every change in behaviour, code, schema or response, side by side or as a diff. |
| Colour | Red and struck through: removed. Green: added. Yellow: what a caller or user will notice. Green and red also mark met and not met. A legend near the top says what each colour means. A word or a sign always goes with the colour, so the page still reads when it's printed, or for someone who can't tell the colours apart. |
| Sample rows | Each table whose contents change: a few rows before and after, so the reader sees the data, not only the columns. |
| Walkthrough | A flow a caller or user goes through, told as an invented customer's story, step by step. Each step shows the request, what the service checks, what it sends a provider, the response, and the rows it writes. |
| Refusals | Each request the change turns away, with its status and its exact message. |
| Diagram | Required when the work changes how a request moves between services or components, or adds or changes states and the moves between them. Elsewhere, use one where it's clearer than text. Draw it inside the page, such as in inline SVG, and label every box and arrow in words. |
| Numbered items | Each finding, decision, risk and question, numbered as COL-21 says. A withdrawn item stays, greyed out, with why. |

### The page

- It stands alone, like a Markdown file: one `.html` file with its styles and any scripts inside it. It needs no build step, shares no files with other documents, and loads nothing from the internet.
- If the work reads better in more than one file, use more than one. Each still stands alone.
- Its header links to the documents it belongs with, by relative path: its tasks, and its proposal, plan or completion document.
- Nothing depends on it, so the engineer can delete it when it's no longer needed (COL-18).

Here's how a plan's page starts. It's invented, based on `TRF-016` (COL-15). Start from these styles or your own, as long as yours do the same jobs.

````html
<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Plan: new asset and one-coin acme wallets</title>
<link rel="icon" href="data:,">
<style>
  body { font: 16px/1.6 system-ui, sans-serif; max-width: 60rem; margin: 2rem auto; padding: 0 1rem; color: #1f2328; background: #fff; }
  h2 { margin-top: 2.5rem; padding-bottom: .3rem; border-bottom: 1px solid #d0d7de; }
  table { border-collapse: collapse; width: 100%; }
  th, td { border: 1px solid #d0d7de; padding: .4rem .6rem; text-align: left; vertical-align: top; }
  code, pre { font-family: ui-monospace, monospace; font-size: .9em; }
  pre { background: #f6f8fa; border: 1px solid #d0d7de; padding: .75rem; overflow-x: auto; tab-size: 4; }
  .src { color: #59636e; font-size: .85em; margin-bottom: 0; }
  .compare { display: grid; grid-template-columns: minmax(0, 1fr) minmax(0, 1fr); gap: 1rem; }
  /* What changes. A word or a +/- sign always goes with the colour. */
  .add { background: #dafbe1; color: #116329; }
  .del { background: #ffebe9; color: #82071e; text-decoration: line-through; }
  .chg { background: #fff8c5; }
  .withdrawn { color: #59636e; }
  /* Syntax highlighting */
  .kw { color: #cf222e; } .fn { color: #8250df; } .str { color: #0a3069; }
  .key, .num { color: #0550ae; } .cm { color: #59636e; font-style: italic; }
</style>
</head>
<body>
<h1>Plan: new asset and one-coin <code>acme</code> wallets</h1>
<p><strong>Task:</strong> <a href="../tasklist.md#trf-016">TRF-016</a>.
   <strong>Status:</strong> waiting for your approval. <strong>Repos:</strong> <code>wallet-service</code>.
   <strong>Completion document:</strong> none yet.</p>

<h2>Summary</h2>
<p><code>acme</code> now opens one account per coin, but the service still asks it for one <code>USD</code> account
   that takes every coin. This plan makes each <code>acme</code> wallet hold exactly one coin, chosen when the
   wallet is created. It also adds the new asset, on Ethereum only.</p>
<p><strong>What I need from you:</strong></p>
<ol>
  <li>Approval to start, or what to change first.</li>
  <li>Whether removing an <code>acme</code> wallet's only coin should be refused, as I propose, or should close the wallet.</li>
</ol>

<p><strong>How to read the changes:</strong> <span class="del">− removed</span>, <span class="add">+ added</span>,
   and <span class="chg">what a caller will notice</span>.</p>

<!-- The sub-task table comes next (COL-17), then one section for each sub-task. -->
</body>
</html>
````

### Check it in a browser

Before sending a proposal, a plan or a completion document, open it with [Playwright MCP](https://github.com/microsoft/playwright-mcp), which this standard requires. Playwright MCP blocks `file:` URLs by default (tested on v0.0.82), so serve the documents first. Serve the folder that holds `implementation/` and `completion/`, so the links between documents work. This or any other local server will do:

```sh
python3 -m http.server 8321 --bind 127.0.0.1
```

Open `http://127.0.0.1:8321/implementation/<topic>.html` or `http://127.0.0.1:8321/completion/<topic>.html`, then do all of these. Playwright's page snapshot isn't enough, because it can't show the layout.
- Take a full-page screenshot and look at it. Don't give it a file name, so it goes to Playwright's output folder and not into the project.
- On a long page, also look at it one screen at a time, with a screenshot of each screen as you scroll, or of each section. A full-page screenshot of a long page shrinks too far to read, and past about 16,000 pixels Chromium starts repeating the top of the page in it (tested on a plan 24,700 pixels tall).
- Read the browser console for errors.
- Try anything interactive.
- Open each link to another document.

Fix what's wrong and check again, then stop the server. If Playwright MCP isn't available, say so in the report, because the document hasn't been checked.

**Why.** These documents are where the engineer decides: which design, whether to start, and whether the work is `done`. A list of edits is quick to write, but it leaves the engineer to rebuild the reasoning from the code, or to ask. Each question is a round trip while the work waits, and the answer stays in a chat where the next reader won't find it. The standard's owner found that plans written as lists of edits needed rounds of questions before they could be approved. Code examples matter most, because prose about code can be wrong in ways nobody notices, and the code is what the engineer can check.

One project's schema proposal shows what works. It follows two invented customers through every request, response and stored row. It shows each table as the database reports it, with removed columns struck through in red and new ones in green, and it gives sample rows for each kind of transfer. Its three revisions are recorded in it, and the options it dropped stay visible, greyed out, with why.

**How it's checked.** A person checks it in review, because no linter can tell whether a page explains. Before sending one, the writer runs COL-14's three tests, and these three:
1. Could the engineer make their decision from this page alone, without asking anything?
2. Does every change have its code: today's code, what a caller sees before and after, the diff, the new code and the tests?
3. Did the browser check pass, with the screenshots looked at and no console errors?

These go back:
- a list of edits, with no walkthrough of the problem and the reasoning;
- a change with no code, proposed code that's pseudo-code, or proposed code that doesn't say how far it was checked;
- today's code paraphrased instead of copied, or with no source;
- a code block with no syntax highlighting;
- colour with no legend, or colour as the only signal;
- a change to how a request moves between services, or to states, with no diagram;
- a collapsed summary, or a decision the engineer has to dig for;
- a page that's broken, loads anything from the internet, or wasn't checked in a browser;
- anything the engineer has to ask the meaning of.
