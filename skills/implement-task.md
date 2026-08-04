---
slug: implement-task
version: 1
applies_to: [lead-programmer, devops]
tools_required: [memory_search, task_update, pr_open]
priority: 10
description: Carry a task from spec to pull request inside the agent's own worktree
---

## Purpose

Take one task from its spec to a reviewable pull request without leaving the
worktree in a state someone else has to clean up.

## When To Use

You have been dispatched with a task assigned to you. Always.

## Procedure

1. Read the spec and the acceptance criteria in full before opening a file.
2. Call `memory_search` on the area. Reimplementing a helper that already lives
   in this repository is the most common failure in this company.
3. Trace the real flow the change touches — every caller, not just the file the
   task names. Fix the cause where all callers route through it.
4. Write the smallest change that satisfies every acceptance criterion.
5. Leave one runnable check behind for non-trivial logic: the smallest test that
   fails if the logic breaks.
6. Run the project's existing tests. A failure you did not cause is still a
   failure you report.
7. Commit on your branch with a message stating what changed and why.
8. `task_update` with what you did and how you verified it, then `pr_open`.

## Constraints

- Stay inside your worktree. Another agent owns every other directory.
- Never commit a secret, a token, or a `.env` file.
- Never weaken a test to make it pass.
- If the spec is ambiguous, `ask_role` the tech-leader. Do not guess and do not
  silently pick the easier reading.
- If you could not finish, say so in `task_update` and name the blocker. An
  unreported failure costs more than a reported one.

## Output Contract

A branch with commits, one `task_update` containing a summary and the
verification you ran, one `pr_open`, and a `memory_write` of kind `pitfall` for
anything that cost you time.

## Examples

Good `task_update` summary:
"Added expiry check in `auth.VerifyToken`; the bug was in the shared verifier,
not the reset handler, so all four callers were affected. Added
`TestVerifyToken_Expired`. Full suite passes (142 tests)."

Bad: "Fixed the bug. Should work now."
