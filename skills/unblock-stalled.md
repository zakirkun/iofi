---
slug: unblock-stalled
version: 1
applies_to: [sprint-manager, tech-leader]
tools_required: [task_list, task_depend, handoff]
priority: 20
description: Turn a task that has stopped moving into one that is moving or honestly blocked
---

## Purpose

Get stopped work moving again, or make the reason it cannot move explicit enough
that someone can act on it.

## When To Use

A task has sat in a working column with no events, a dependency chain is not
progressing, or the same task appears in two consecutive hygiene passes.

## Procedure

1. `task_list stalled_hours=24` to find them, then read each task's events. The
   last event usually names the reason, and a task with no events at all was
   never started — a different problem from one that stopped.
2. Classify the cause before acting, because each has a different fix:
   - **Waiting on another task** — check whether the dependency is still real.
     Dependencies outlive their reason; `task_depend` removes the stale link.
   - **Waiting on a person** — someone was asked and did not answer. Ask again
     once through the right role, then escalate.
   - **Waiting on an agent that is not free** — capacity, not blockage. Reassign.
   - **Too big to finish** — it will stall again after any intervention. Split it.
   - **Nobody knows how** — it needs a decision, not more time.
3. Act on the classification. Do not simply move the task to `blocked`; that
   records the symptom and stops anyone from looking again.
4. If it is genuinely blocked, name the blocker as a task id or a person and a
   question. "Blocked" with no named blocker is where tasks go to be forgotten.
5. `handoff` with a note when the right fix is a different role, saying what was
   done and what remains. A handoff without that makes the receiver start over.
6. Set a check-back. If nothing has changed by the next pass, escalate rather
   than repeating this one.

## Constraints

- Never leave a stalled task in the state you found it. If you could not fix it,
  record what you tried and who must act.
- Never remove a dependency to make the board move without confirming the work
  genuinely no longer needs it.
- Do not reassign a task with an active run; `run_list` first, cancel or wait.
- Do not split a task that is stalled for a reason splitting will not fix. Two
  blocked tasks are worse than one.
- The second time the same task stalls, the fix is not another nudge.

## Output Contract

Per stalled task: the classified cause, the action taken, and either an owner
with a next step or a named blocker with a question. Anything stalled twice
escalated rather than nudged again.

## Examples

Good: "#93 stalled 40h in `in_progress`, last event was `ask_role` to tech-leader
5 days ago with no reply. Cause: waiting on a person. The question is whether
soft delete is required for audit; I asked the CTO, who ruled soft delete.
Recorded on the task and handed back to lead-1 with the ruling. #97 stalled 30h
waiting on #70, which closed Monday — stale dependency removed, moved to ready."

Bad: "Moved four stalled tasks to blocked."
