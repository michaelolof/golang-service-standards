#!/bin/sh
# For a coding agent's "finish" hook. Point the hook at this file.
# Nothing uncommitted and nothing unpushed: exit 0. Checks pass: exit 0.
# Checks fail: the errors go to stderr and it exits with 2, which tells the agent to keep working.
[ -z "$(git status --porcelain)" ] && [ -z "$(git log --branches --not --remotes -1 --format=%h)" ] && exit 0
out=$(task check:fast 2>&1) && exit 0
echo "$out" >&2
exit 2
