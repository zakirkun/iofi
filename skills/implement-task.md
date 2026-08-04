---
slug: implement-task
version: 2
applies_to: [lead-programmer, devops]
tools_required: [task_get, task_search, memory_search, knowledge_search, repo_diff, task_update, pr_open]
priority: 10
description: Carry a task from spec to pull request inside the agent's own worktree
---

## Purpose

Take one task from its spec to a reviewable pull request without leaving the
worktree in a state someone else has to clean up.

## When To Use

You have been dispatched with a task assigned to you. Always.

## Procedure

1. `task_get` and read the spec and acceptance criteria in full before opening a
   file. The brief you were dispatched with may be older than the task.
2. Call `memory_search` on the area. Reimplementing a helper that already lives
   in this repository is the most common failure in this company.
3. `knowledge_search` if the task references a specification, standard or
   contract. Building to a guess about a documented requirement is rework.
4. `task_search` the same area. Work already done or already in flight changes
   what you should write, and a conflicting branch is cheaper to find now.
5. Trace the real flow the change touches — every caller, not just the file the
   task names. Fix the cause where all callers route through it.
6. Write the smallest change that satisfies every acceptance criterion.
7. Leave one runnable check behind for non-trivial logic: the smallest test that
   fails if the logic breaks.
8. Run the project's existing tests. A failure you did not cause is still a
   failure you report.
9. `repo_diff` and read your own change before committing. Anything in it the
   task did not ask for is scope you added, and the reviewer will ask about it.
10. Commit on your branch with a message stating what changed and why.
11. `task_update` with what you did and how you verified it, then `pr_open`.
    `pr_status` afterwards if you need to confirm it landed.

## Constraints

- Stay inside your worktree. Another agent owns every other directory.
- Never commit a secret, a token, or a `.env` file.
- Never weaken a test to make it pass.
- If the spec is ambiguous, `ask_role` the tech-leader — `role_list` first if you
  are not certain of the slug. Do not guess and do not silently pick the easier
  reading. If no role can answer because it is a business call, `escalate_founder`
  once and carry on with the parts that do not depend on the answer.
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
