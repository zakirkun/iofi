---
slug: verify-acceptance
version: 2
applies_to: [qa]
tools_required: [task_get, task_create, kanban_move, repo_diff, knowledge_search, memory_write]
priority: 10
description: Check a task's acceptance criteria against the real branch and gate the done column
---

## Purpose

Decide whether a task actually meets its acceptance criteria, on the real
branch, and keep anything unverified out of the done column.

## When To Use

A task has reached the QA column, or a pull request is open against a task you
own verification for.

## Procedure

1. `task_get` and read the acceptance criteria as a numbered list.
2. `repo_diff` to see what actually changed. Verifying against the task
   description alone misses the two things that matter most: a criterion the
   change never touched, and a change nothing in the task asked for.
3. For each criterion, decide how you will observe it before you observe it.
   Write that down.
4. `knowledge_search` for any uploaded standard the criteria reference or should
   have. A change meeting its criteria while violating an uploaded policy is a
   fail, and the criteria are then also a defect.
5. Run the project's tests. Then check each criterion by the method you chose.
6. Record, per criterion: pass, fail, or could-not-verify — with what you saw.
7. File each failure as its own task with reproduction steps and `role_slug`
   set to the role that should fix it.
8. Move the task: `done` only if every criterion passed; otherwise back to
   `in_progress`, or `blocked` when it cannot proceed at all. If the blocker is
   a decision nobody has made, `escalate_founder` rather than leaving it to sit.

## Constraints

- A criterion you could not verify is not a pass.
- Do not fix the code. You verify; another role fixes.
- Do not change acceptance criteria to match what was built.
- Do not accept "works on my machine" — say which command you ran.

## Output Contract

One `task_update` containing the numbered criteria with a verdict and evidence
for each, one task per defect found, one `kanban_move`, and a `memory_write` of
kind `pitfall` when the same defect pattern appears a second time.

## Examples

```
1. Token expires in 30 minutes — PASS. Set clock +31m, VerifyToken returned
   ErrExpired.
2. Reused token returns 400 without account info — FAIL. Returns 400 but the
   body includes the email address. Filed as task #58.
3. Unknown address behaves identically — COULD NOT VERIFY. No staging account
   available. Blocking on devops.
```
