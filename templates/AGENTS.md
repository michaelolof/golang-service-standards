# Agent instructions

This project follows the standard in `.standards/`. Its rules are mandatory. If a request
conflicts with one, say so before doing the work.

Before changing code, read `.standards/RULES.md`, then the section for your area:

| You're changing                         | Read                                       |
|-----------------------------------------|--------------------------------------------|
| project layout, config, the Taskfile    | `.standards/01-project-layout.md`          |
| any Go code: naming, shape, readability | `.standards/02-style-and-naming.md`        |
| IDs, money, time, enums, interfaces     | `.standards/03-domain-and-abstractions.md` |
| errors or stamps                        | `.standards/04-errors.md`                  |
| a handler, job, context or log line     | `.standards/05-transport-and-runtime.md`   |
| a query, repository or migration        | `.standards/06-persistence.md`             |
| a test                                  | `.standards/07-testing.md`                 |
| a route, token, webhook or integration  | `.standards/08-security.md`                |

Before starting a task, read `.standards/09-collaboration.md`. It says how a task goes from brief
to `done`, how to write task lists and plans, and how to explain things in clear language.

For a security audit, follow `.standards/security-audit.md`.

A task isn't `completed` until `task check` passes. Never edit `.standards/`. On a new clone, run
`task setup` once.

Never type a stamp. Write `__STAMP__` where one belongs, then run `task stamps`, which reads the clock
for each one. A stamp has no meaning. Never order it, number it, version it, or work it out from
nearby stamps. To change a stamp, replace it with `__STAMP__` and run `task stamps` again.

New tests follow `.standards/07-testing.md`, in every project. A flow proves a promise from
`docs/promises/` (TEST-18). Harness steps do one thing, and flows compose them (TEST-19). Each kind of
test has its own task: `task test` for unit and handler tests, `task test:integration`, `task test:flows`,
and `task test:all` for all three on a fresh test stack. Don't restructure existing tests: they're the
guardrails during a refactor. Only the engineer lifts that pause, and the project then records it below,
as TEST-14 says.

You can mark work `completed` on your own. Only the engineer marks it `done`, by an explicit
instruction. When a workstream's last task is `completed`, write its completion document,
`completion/<topic>.html`, straight away, without being asked, then send it to the engineer. Any
change to stored data gets its own section, with the schema and the explanation. A small change with
no plan or task list entry gets a completion document too, with no `done` step. Never send part of a
workstream for review. Never delete a proposal, implementation plan or completion document unless the
engineer explicitly tells you to.

Proposals, implementation plans and completion documents are standalone HTML pages, as COL-22 says:
`implementation/<topic>-proposal.html`, `implementation/<topic>.html` and `completion/<topic>.html`.
Write each one as a walkthrough that a colleague new to the code can follow, like a clear technical
blog post, never a list of edits: what happens today, why it's a problem, what changes, what it costs,
and what you need from the engineer. Show every change as code, with syntax highlighting: today's code
copied with its `path:line`, what a caller sees before and after, the diff, the new code in full, and
its tests. Give colour a legend, and draw a diagram whenever a flow between services or a set of
states changes. Before sending one, check it with Playwright MCP, using screenshots and the console:
a full-page screenshot, and on a long page, one screen at a time.

Every scope on the task list is marked `greenfield`, `existing` or `unconfirmed`, as COL-15 says. The
engineer sets it when the task list or a task is written, because the repo can't show it. Never assume either way: mark it `unconfirmed`
and ask, and don't plan or change anything until they answer. On `greenfield`, don't add migrations of
old data or compatibility code. On `existing`, the plan says how what exists keeps working.

Before starting any work, read `security-checks.md`, beside the task list. If its "Next due" date has
arrived, or there's no file or no row, run the security check first and record it, as SEC-27 says. It covers
primary dependencies, not their dependencies. Every report has a "Security check" line.

Run at most one subagent at a time, and only when it's justified. Say why in one line before starting
it, as COL-20 says.

Number every finding, piece of feedback, decision, concern, question and warning you report, in chat
and in documents, as COL-21 says. Keep one count for the whole conversation or document: never start
again at 1, never give a number to a second item, and use bullets for any other list, such as next
steps. The numbers are for the engineer to point at, not a shorthand for you. Every time you mention
one, even in a range, a list or a "depends on", say what each item is: "Fixed 3 (the relay drops
errors from `tick`)", never "Fixed 3". Before you send anything, find every item number in it,
including in next steps and the "Warnings" line, and check that each one says what the item is.

Every report and completion document has a "Warnings" line. If a task depends on one in another
workstream that isn't `done` yet, carry on, and say so there.

@.standards/RULES.md

Everything under the next heading adds to the standard and ranks below it. If anything there conflicts with
a rule in the standard, follow the standard and say so in your reply. The only exception is a line starting
"Exception to <rule ID>:" that says why.

## This project's own rules
