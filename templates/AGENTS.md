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
full-page screenshot and the console, as COL-9 says, then send it to the engineer. A small change with
no plan or task list entry gets one too, with no `done` step. Never send part of a workstream for
review. Never delete an implementation plan or completion document unless the engineer explicitly
tells you to.

Every report and completion document has a "Warnings" line. If a task depends on one in another
workstream that isn't `done` yet, carry on, and say so there.

@.standards/RULES.md

Everything under the next heading adds to the standard and ranks below it. If anything there conflicts with
a rule in the standard, follow the standard and say so in your reply. The only exception is a line starting
"Exception to <rule ID>:" that says why.

## This project's own rules
