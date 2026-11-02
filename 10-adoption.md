# 10 · Adoption: how a project takes on this standard

This says how any project consumes the standard, and what makes it mandatory for the agents and people working there. It needs only git, Go and [Task](https://taskfile.dev). It doesn't depend on a particular git host, CI product, operating system or coding agent.

The standard lives in [its own git repo](https://github.com/michaelolof/golang-service-standards), with version tags. A project keeps a pinned copy of it. Three things then make it mandatory:
1. **The agent is told.** `AGENTS.md` loads the rules at the start of every session.
2. **The agent is stopped.** A hook runs the checks before it finishes or pushes.
3. **The merge is stopped.** CI blocks anything that fails, whoever or whatever wrote it.

The first alone isn't enough, because an agent can ignore an instruction file. Only the checks make a rule mandatory.

## Rules at a glance

| ID | Rule | Enforced by |
|---|---|---|
| ADO-1 | The standard is a git repo with version tags. A project adds it at `.standards/` with `git subtree`, and pins the version. | review |
| ADO-2 | Nobody edits `.standards/` inside a project. | CI drift check |
| ADO-3 | Each code repo has one `AGENTS.md` at its root, which says the rules are mandatory and loads the rule index. A subfolder may add its own. | `task check` (`agents:check`) |
| ADO-4 | An agent can't finish its turn, or push, while the checks fail. | agent hook, git hook |
| ADO-5 | `task check` runs every check, and it's a required check on the default branch. | CI |
| ADO-6 | An existing project fixes everything first, and its existing tests stay as they are. | task list |
| ADO-7 | A rule changes through a PR on the standard, approved by its owner. Projects upgrade with a normal PR. | review |

---

## ADO-1 · Add the standard to a project

**Rule.** Add the standard with `git subtree`, and record the version. The [README](README.md#install) has the full steps. The project needs a `go.mod`, at least one Go package and a clean working tree: `git subtree` stops with "working tree has modifications" otherwise, and `task check` has nothing to lint or test in an empty project (tested).

```sh
git subtree add --prefix=.standards https://github.com/michaelolof/golang-service-standards.git v0.1.1 --squash
echo v0.1.1 > .standards-version
cp .standards/templates/Taskfile.yml Taskfile.yml
task setup
git add -A && git commit -m "Pin standards v0.1.1"
```

[templates/Taskfile.yml](templates/Taskfile.yml) has `check` and `check:fast`, because the hooks and CI call them. If the project already has a Taskfile, copy those two tasks, the `vars` and the `includes` into it. Without them, every hook fails with `Task "check:fast" does not exist` (tested).

The result in a project:

```text
my-project/
  AGENTS.md            # the standard's text at the top, the project's own rules below it
  Taskfile.yml         # includes .standards/Taskfile.standards.yml
  .golangci.yml        # generated from the standard; nobody edits it
  .standards/          # the standard at the pinned version; nobody edits it
  .standards-version   # v0.1.1
```

**Why `git subtree` and not a submodule.** The rules become ordinary files in the project, so every clone has them. A cloud agent or a CI job that doesn't load submodules still sees them (tested on a local repo):

| Fresh `git clone` | Files in `.standards/` |
|---|---|
| Added with `git subtree` | all of them |
| Added as a submodule | 0, until someone runs `git submodule update --init` |

**It stays out of the way.** Go skips folders that start with a dot, so `go list ./...` shows only the project's own packages (tested). The rule documents work the same way in a project in any language.

**The lint config is generated, not copied.** Two of its rules need the project's module path, and nobody may edit `.standards/`. So `task lint:config` writes `.golangci.yml` at the project root from the standard's copy, and `task lint:config:check` fails if someone edits it (tested). These two tasks use `sed` and `cmp`, which are on Linux and macOS, and on Windows with Git for Windows.

**The linter builds itself.** The config needs the `requiredtag` plugin, which a plain `golangci-lint` doesn't have. The first `task lint` builds golangci-lint v2.15.0 with the plugin compiled in, into `.standards/lint/bin/`. That takes about 20 seconds and needs network access. You don't install golangci-lint, and nothing is added to your `go.mod`. The folder is ignored by git, so it can't trigger the drift check. An upgrade that changes the linter's version or the plugin rebuilds it on the next run (all tested).

## ADO-2 · Nobody edits the copy

**Rule.** CI compares `.standards/` with the pinned tag, and fails if they differ. `task standards:drift` does it, using only git:

```sh
git fetch -q https://github.com/michaelolof/golang-service-standards.git "refs/tags/$(cat .standards-version)"
test "$(git rev-parse 'FETCH_HEAD^{tree}')" = "$(git rev-parse 'HEAD:.standards')"
```

We edited a rule inside a project, and the check failed. Reverting the edit made it pass again (tested).

**Why.** Copies drift when people can edit them. This keeps the copy read-only, without needing a tool to manage it. To disagree with a rule, change it in the standard (ADO-7), or record an exception under `## This project's own rules` in `AGENTS.md`, as `Exception to TEST-4:` and the reason. An exception tells the agent what to do, but doesn't turn off a check (COL-7).

## ADO-3 · One `AGENTS.md` at the root of each code repo

**Rule.** Every code repo has one `AGENTS.md` at its root. `task setup` writes it from [templates/AGENTS.md](templates/AGENTS.md). Everything down to the heading `## This project's own rules` is the standard's text, and everything below that heading is the project's. The template:

- says the rules are mandatory, and that a conflict must be raised before doing the work;
- lists which document to read for each area;
- says a task isn't finished until `task check` passes;
- repeats the two mistakes agents make most often: inventing stamps, and restructuring existing tests.

[RULES.md](RULES.md) is every rule on one page, generated from the "Rules at a glance" tables: about 200 lines and 3,500 words, so it's short enough to load at the start of every session. An agent reads the full document only for the area it's changing.

**Why at the root of the code repo.** An agent opened in a repo reads that repo's files. An `AGENTS.md` kept only in a separate docs repo is missed when an agent is opened directly in the code repo.

**It stays in place.** `task check` runs `agents:check`, which fails if:
- `AGENTS.md` is missing, or no longer starts with the template, so a project can't lose the instructions by accident or stay on old ones after an upgrade;
- `AGENTS.old.md` still exists;
- a `CLAUDE.md` has any text beyond the line `@AGENTS.md`, unless it's a symlink to `AGENTS.md`;

`task agents` rewrites the standard's text and keeps what's below the heading. It refuses to run if `AGENTS.md` has uncommitted edits above the heading, because it would replace them (all tested).

**An existing `AGENTS.md` is reviewed, not merged.** `task setup` renames it to `AGENTS.old.md`, and `task check` fails until someone has moved what's still true under the heading and deleted the file. The [README](README.md#if-the-project-already-has-an-agentsmd) has the steps.

The reason is that an old line can contradict the standard, and no check can spot that. We put a contradicting line below the template in `AGENTS.md`, and asked real Claude Code 2.1.314 sessions what to do, three times each. This shows how many times the old line won, without and with the template's sentence saying the standard wins:

| The old line said | Model | Without the sentence | With it |
|---|---|---|---|
| Use testify `assert`, against TEST-4 | Haiku | 2 of 3 | 3 of 3 |
| Use testify `assert`, against TEST-4 | Sonnet | 3 of 3 | 0 of 3 |
| Inject a `*slog.Logger`, against LOG-1 | Haiku | 1 of 3 | 0 of 3 |
| Inject a `*slog.Logger`, against LOG-1 | Sonnet | 1 of 3 | Not run |

Three runs each is a warning, not a measurement. It's enough to show the sentence helps but can't be relied on, so the template keeps it and the review is what's enforced. In the same tests, a line starting `Exception to TEST-4:` that gave a reason was followed 3 of 3 times on Haiku, and asking Haiku to list the conflicts found every one, 3 of 3 times, without flagging the line that was fine.

A project's later additions under the heading go through normal review, for the same reason.

**Subfolders.** A team can add an `AGENTS.md` in a subfolder if it wants one, for example for one service in a monorepo. `task check` doesn't look at these files, so they're checked in review. The [agents.md](https://agents.md/) site says the nearest file "takes precedence", so a contradiction there can win in that folder.

**Starting in a subfolder.** We tested this on Claude Code 2.1.314:

| Started in the subfolder | Result |
|---|---|
| No tools allowed, with or without a subfolder `AGENTS.md` | Reads the root `AGENTS.md`, but `RULES.md` isn't loaded |
| Allowed to read files | Went and read the rules, and answered correctly, 3 of 3 times |
| At the root, then reading a file in the subfolder | Has the root rules and the subfolder's file |

The import doesn't load because `.standards/RULES.md` is outside the folder the session started in. Claude Code's [docs](https://code.claude.com/docs/en/memory) say an interactive session asks once for approval, and a non-interactive one only loads it if that was already approved. This is why the template also says in plain words to read `RULES.md`.

**How each agent gets it.** `AGENTS.md` is a [shared format](https://agents.md/) that many coding agents read on their own. The file also says in plain words to read `RULES.md`, so it works for any agent that reads it. The line `@.standards/RULES.md` in the template is an import for agents that support one, and plain text to the rest.

| Agent | What to do |
|---|---|
| Reads `AGENTS.md` on its own (such as Codex, Cursor, GitHub Copilot's coding agent, Windsurf) | Nothing. Not tested. |
| Claude Code, v2.1.277 or later | Nothing. It reads `AGENTS.md` and expands the import. A `CLAUDE.md` replaces `AGENTS.md` unless it imports it, and any other text in it could contradict the standard, so `agents:check` fails unless it holds only `@AGENTS.md` or is a symlink to `AGENTS.md`. |
| Reads its own file by default (such as Gemini CLI, Aider) | Set its config to read `AGENTS.md`, as [agents.md](https://agents.md/) shows. Not tested. |
| Anything else with its own instructions file | Add that file with one line: "Follow `AGENTS.md`." |

We tested the Claude Code row on 2.1.314, in sessions with no tools allowed, asking for one rule from `RULES.md` and one rule from under the heading:

| In the project | What the session could quote |
|---|---|
| `AGENTS.md` only | Both |
| `AGENTS.md`, and a `CLAUDE.md` without the import | Neither |
| `AGENTS.md`, and a `CLAUDE.md` holding only `@AGENTS.md` | Both |
| `AGENTS.md`, and `CLAUDE.md` as a symlink to it | Both |

## ADO-4 · The agent can't finish or push while the checks fail

**Rule.** The checks run before work leaves the agent, in one of two ways. Use the first wherever your agent supports it, and the second everywhere.

`task check:fast` runs the standard's local checks and unit tests, because it runs often. The project has it from the template (LAY-14).

**A git hook, for any agent and any person.** It stops a push while the checks fail. `task setup` turns it on, with `git config core.hooksPath .standards/hooks`. Git doesn't copy that setting between clones, so run `task setup` once on each clone.

We tested this on a project with a lint failure. The push was refused and the remote stayed empty. Once the code was fixed, the push went through. A clone that hasn't run `task setup` has no hook, and `git push --no-verify` skips it. Both are why this can't replace CI.

**A finish hook, for agents that have one.** Some agents can run a command when they try to finish a turn. Point it at [hooks/on-finish.sh](hooks/on-finish.sh). It exits with 0 if the checks pass, or if there's nothing to check: no uncommitted changes and no commits that haven't been pushed. If the checks fail, it sends the errors to stderr and exits with 2 (tested), so the agent can keep working. Looking at unpushed commits matters, because otherwise an agent could get past the hook by committing its failing code (tested).

| Agent | How to connect it |
|---|---|
| Claude Code | `mkdir -p .claude && cp .standards/templates/claude-settings.json .claude/settings.json`, then commit it. If the project already has that file, add the `hooks` block from [the template](templates/claude-settings.json) to it. |
| Any other agent with a finish hook | Point the hook at the script, and check in that agent's docs how it treats exit code 2 and stderr. Not tested. |
| An agent with no such hook | Use the git hook, and rely on CI. |

We tested this on Claude Code 2.1.314, with the template's settings file and a lint failure already in the project. A session asked only to say hello was blocked, read the errors, fixed the code and ran `task check`, and the hook then let it finish. The failure was blocked the same way when it was already committed and the working tree was clean. With nothing to check, a session finished in one turn.

**Starting in a subfolder.** Claude Code doesn't load the root `.claude/settings.json` when a session starts in a subfolder, so the finish hook doesn't run. If people work this way, copy the file into that subfolder's `.claude/` too. Its command finds the repo root itself, so the same file works in both places:

| Session started in | Settings file at | Result |
|---|---|---|
| The root | The root | Blocked, and the agent fixed the code |
| A subfolder | The root only | Not blocked. The push was then refused by the git hook. |
| A subfolder | The root and the subfolder | Blocked, and the agent fixed the code |

**Limit.** A finish hook gives up on an agent that can't fix the failure. Claude Code's docs say it [ends the turn after 8 blocks in a row](https://code.claude.com/docs/en/hooks). In our test, with a failure the agent had no way to fix, the session ended after 10. So a stuck agent doesn't loop forever, and a finish hook can't replace CI either.

## ADO-5 · CI is the final check

**Rule.** `task check` runs everything: `standards:check` (the `AGENTS.md` check, the lint config check, lint and the drift check), then the project's own tests and build. It's a required check on the default branch, so nothing merges while it fails. This is the only layer that works for every agent and every person.

Any CI that can run a command will do. The job installs Go and Task, and runs one command:

```sh
task check
```

The job also needs network access, to build the linter and to reach the standards repo for the drift check.

**What this can't stop.** Marking the job as required is a setting on your git host, so it can't be done or checked from inside the repo. Someone who can change the CI job or the project's Taskfile can take a check out, so review changes to those files like code. If your host can require a named reviewer for certain files, use that for them.

| Who wrote the code | Told by | Stopped by |
|---|---|---|
| An agent with a finish hook | `AGENTS.md` | the hook, then CI |
| Another agent | `AGENTS.md` | the git hook if it's set up, then CI |
| A person | the rules and review | the git hook if it's set up, then CI |

Rules that no check can cover are checked in review (COL-10). A rule that keeps needing the same correction should become a check (COL-13).

## ADO-6 · New and existing projects

**New projects** start with the setup above, so they pass from the first commit.

**Existing projects fix everything first:**
1. Add the standard ([ADO-1](#ado-1--add-the-standard-to-a-project)).
2. Review `AGENTS.old.md`, any `CLAUDE.md`, and any `AGENTS.md` in a subfolder ([ADO-3](#ado-3--one-agentsmd-at-the-root-of-each-code-repo)), so agents stop following instructions that contradict the standard. Do this first.
3. Run `task check`, and turn what fails into task-list items (COL-15).
4. Fix the code. Existing tests stay as they are and must keep passing (TEST-14). Test files aren't linted, so nothing here forces them to change.
5. Make `task check` a required check once it passes.

New feature work waits until step 5.

## ADO-7 · Changing and upgrading

**Rule.**
- A rule changes through a PR on the standard, approved by its owner (COL-13). Each release is a version tag with notes on what changed.
- A project upgrades with a normal PR, so the rule changes show up in its diff. Its CI must pass before it merges (tested, apart from CI):

  ```sh
  git subtree pull --prefix=.standards https://github.com/michaelolof/golang-service-standards.git v0.2.0 --squash
  echo v0.2.0 > .standards-version
  task setup
  git add -A && git commit -m "Upgrade standards to v0.2.0"
  ```

- `task setup` rewrites the standard's part of `AGENTS.md`, keeps the project's rules under the heading, and rewrites `.golangci.yml`. If someone skips it, `task check` fails and says which command to run (tested).
- The project's `check` depends on `standards:check`, so a check added to the standard starts running without any change to the project's Taskfile.

After changing a rule in the standard, run `go run ./tools/genrules` to update `RULES.md`. `go run ./tools/genrules -check` fails if it's out of date.

## What's built and what isn't

| Part | Status |
|---|---|
| Rule documents, `RULES.md` and its generator | Done |
| Lint config and the `requiredtag` plugin | Done, and tested from `.standards/` |
| Templates (`AGENTS.md`, `Taskfile.yml`, `claude-settings.json`), hooks, `standards:check` | Done, and tested |
| The standard's version tags | Done, from `v0.1.0` |
| Shared architecture test (`arch` in the Taskfile) | To build. Until then, the rules it enforces are checked in review. |
| `constquery` and `repoverbs` analyzers | To build. Until then, the rules they enforce are checked in review. |
| Shared test helpers (`testharness`, `apitest`, `pagination.AssertStablePaging`, `testutils.AssertConcurrentUpdateConflicts`, the migrate-twice test, conformance suites) | To build. A project writes its own until then, from the descriptions in [07](07-testing.md) and [06](06-persistence.md). |
