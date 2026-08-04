---
slug: code-review
version: 1
applies_to: [tech-leader, qa]
tools_required: [repo_diff, repo_log, task_comment]
priority: 15
description: Review a branch for defects and scope, and say what must change rather than what you felt
---

## Purpose

Decide whether a change is safe to merge, and give the author a list of things
to do rather than an impression of the work.

## When To Use

A task has reached the review column, or a pull request needs a decision before
it goes to QA.

## Procedure

1. `repo_log` first. Commits that do several unrelated things predict a diff
   that does too.
2. `repo_diff summary=true` to see the shape. A change touching files the task
   never mentioned is the first thing to ask about.
3. `repo_diff` in full. Read the change against the task's acceptance criteria,
   not against how you would have written it.
4. For each finding, decide which it is:
   - **Blocking** — it is wrong, unsafe, or fails an acceptance criterion.
   - **Should fix** — it will cause a defect or confusion later.
   - **Preference** — you would have done it differently. Say so once or not at
     all; it is not a reason to hold a branch.
5. Check the things authors most often skip: error paths, the case where the
   input is empty, concurrent callers, and whether the new test would actually
   fail if the logic were reverted.
6. Post the findings with `task_comment`, each as `file:line — what is wrong —
   what to do instead`. Move the task back only if something blocking exists.

## Constraints

- Never approve a change whose test you did not read. A test that asserts
  nothing is worse than no test, because it stops anyone else writing one.
- Do not review style a formatter already enforces.
- Do not ask for an abstraction the task does not need. One implementation
  behind an interface is a cost with no purchaser.
- No praise, no hedging, no "maybe consider". If it must change, say it must.
- If the diff is too large to review honestly, say that instead of skimming it,
  and ask for it to be split.

## Output Contract

One `task_comment` listing every finding as `severity: file:line — problem —
fix`, and either a move to the next column or a move back with the blocking
items named. An empty list is a valid review: say "no blocking findings" and
what you checked.

## Examples

Good: "blocking: internal/api/handlers.go:212 — the 500 path returns before
closing the transaction, leaking a connection per failure. Defer the rollback.
should-fix: internal/store/docs.go:88 — the ILIKE has no bound on query length;
a 1MB query string scans every row."

Bad: "Looks good overall, maybe consider extracting a helper?" — no decision, no
finding, no action.
