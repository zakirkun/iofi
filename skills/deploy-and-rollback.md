---
slug: deploy-and-rollback
version: 1
applies_to: [devops]
tools_required: [repo_log, task_update, memory_write]
priority: 15
description: Ship a change with a rollback that was tested, not assumed
---

## Purpose

Get a change into an environment, and be able to take it back out in minutes
without a discussion about how.

## When To Use

Any deployment, migration, infrastructure change or configuration change to a
running environment.

## Procedure

1. `repo_log` to establish exactly what is going out. "The latest main" is not
   an answer; a commit hash is.
2. Write the rollback before deploying. Name the command or the revert commit,
   and say how long it takes. If the answer is "restore from backup", the change
   is not ready.
3. Check whether the change is reversible at all. A schema migration that drops
   a column, a message format change, and a deleted queue are one-way. Those
   need a two-step deploy: make the new code tolerate both shapes, ship it, then
   change the shape.
4. Name what you will look at to decide it worked, before you deploy. Error
   rate, a specific endpoint's status, a queue depth. Deciding afterwards means
   deciding on whatever looks reassuring.
5. Deploy. Watch the named signal for long enough that a slow failure would have
   appeared — a crash shows in seconds, a leak in minutes.
6. If the signal is wrong, roll back first and diagnose after. Debugging in a
   broken environment costs more than the redeploy.
7. `memory_write` of kind `convention` for anything about this environment the
   next agent must know: an ordering constraint, a warm-up period, a step that
   must not be run twice.

## Constraints

- Never deploy a migration and the code that requires it in a way that cannot be
  reversed independently.
- Never deploy something you cannot roll back at the end of a run, when nobody
  will be watching it.
- Never change two things in one deploy if the signals overlap; you will not
  know which one moved the number.
- Do not skip the rollback rehearsal because the change is small. Small changes
  are the ones deployed without watching.

## Output Contract

A `task_update` containing: the commit deployed, the rollback command and its
measured duration, the signal watched and its value before and after, and
whether the change is reversible in one step or two.

## Examples

Good: "Deployed a1b2c3d. Rollback: `kubectl rollout undo deploy/iofi`, measured
at 40s in staging. Migration 0005 is additive — new tables only — so the old
binary runs against the new schema and rollback is one step. Watched 5xx rate
for 10 minutes: 0.02% before, 0.02% after."

Bad: "Deployed and it looks fine." — no version, no rollback, no signal.
