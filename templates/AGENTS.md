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

A stamp is the current Unix time in milliseconds, read from a clock for each new stamp. It has no
meaning. Never order it, number it, version it, or work it out from nearby stamps.

Don't restructure existing tests. They're the guardrails during a refactor.

You can mark work `completed` on your own. Only the engineer marks it `done`, by an explicit
instruction. When a workstream's last task is `completed`, write its completion document,
`completion/<topic>.html`, straight away, without being asked. Check it with Playwright MCP, using a
full-page screenshot and the console, as COL-9 says, then send it to the engineer. Every code block in
it has syntax highlighting, and any change to stored data gets its own section, with the schema and the
explanation. A small change with no plan or task list entry gets a completion document too, with no
`done` step. Never send part of a workstream for review. Never delete an implementation plan or
completion document unless the engineer explicitly tells you to.

Every scope on the task list is marked `greenfield`, `existing` or `unconfirmed`, as COL-15 says. The
engineer sets it when the task list or a task is written, because the repo can't show it. Never assume either way: mark it `unconfirmed`
and ask, and don't plan or change anything until they answer. On `greenfield`, don't add migrations of
old data or compatibility code. On `existing`, the plan says how what exists keeps working.

Before starting any work, read `security-checks.md`, beside the task list. If its "Next due" date has
arrived, or there's no file or no row, run the security check first and record it, as SEC-27 says. It covers
primary dependencies, not their dependencies. Every report has a "Security check" line.

Run at most one subagent at a time, and only when it's justified. Say why in one line before starting
it, as COL-20 says.

Every report and completion document has a "Warnings" line. If a task depends on one in another
workstream that isn't `done` yet, carry on, and say so there.

@.standards/RULES.md

Everything under the next heading adds to the standard and ranks below it. If anything there conflicts with
a rule in the standard, follow the standard and say so in your reply. The only exception is a line starting
"Exception to <rule ID>:" that says why.

## This project's own rules
