---
slug: board-hygiene
version: 1
applies_to: [sprint-manager]
tools_required: [task_list, board_get, task_assign]
priority: 15
description: Keep the board saying what is true, so decisions made from it are not made from fiction
---

## Purpose

Make the board an accurate record of what is happening, because every plan,
estimate and escalation in the company is derived from it.

## When To Use

Every cycle through the board, and before any planning or reporting that reads
from it.

## Procedure

1. `board_get` first. A column at its WIP limit is why work elsewhere is not
   moving, and it is invisible from a task list.
2. `task_list stalled_hours=24`. Every result needs one of three outcomes: an
   event explaining the wait, a reassignment, or a move to blocked with a named
   blocker. Leaving it is not an outcome.
3. Check every task in `in_progress` has an assignee. An unassigned task in an
   active column is work nobody is doing that everyone believes is being done.
4. Check every task in `blocked` names its blocker and that the blocker is still
   real. A task blocked on something already finished is the most common lie a
   board tells.
5. Check `done` against reality: closed tasks with no verification event are
   claims, not outcomes.
6. Reassign where the assignee is at capacity or paused, using `agent_roster` to
   find someone with room. Say why in the reason.
7. Report what you changed and what remains wrong. A hygiene pass that reports
   only "cleaned up" cannot be checked.

## Constraints

- You may reprioritise, reassign and move tasks. You may not change acceptance
  criteria — that is the author's, and editing it silently changes what "done"
  means after the work started.
- Do not close a task to tidy the board. An abandoned task is closed with a
  reason recorded, or it stays open.
- Do not move a task out of blocked without confirming the blocker cleared.
- Do not silently reassign away from an agent mid-run; check `run_list` for an
  active run first.
- A WIP limit is a decision, not an obstacle. Do not raise it to fit more work
  in; find why the column is not draining.

## Output Contract

A report listing: columns at their limit, stalled tasks and what was done with
each, unassigned active tasks now assigned, blocked tasks whose blocker was
stale, and anything left wrong that you could not fix and who must.

## Examples

Good: "`review` is at its WIP limit of 5, which is why `in_progress` is not
draining — the cause is one reviewer paused since Tuesday. Four tasks stalled
over 24h: #91 reassigned from `qa-1` (at capacity) to `qa-2`; #93 moved to
blocked, waiting on #88; #95 had no assignee while sitting in `in_progress`,
assigned to lead-2; #97 blocked on #70, which closed on Monday — unblocked and
moved to ready. Still wrong: #99 is in done with no verification event; QA must
confirm or reopen."

Bad: "Tidied the board, looks better now."
