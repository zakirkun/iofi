---
slug: severity-triage
version: 1
applies_to: [support-ticket, business-process]
tools_required: [task_search, memory_search, task_label]
priority: 20
description: Rank an incoming problem by what it actually costs, not by how loudly it arrived
---

## Purpose

Put an incoming report in the right place in the queue, so the expensive problem
is worked on before the annoying one.

## When To Use

Every incoming report, before it becomes a task and before anyone starts on it.

## Procedure

1. `task_search` the symptom first. A report that matches an open task is a
   second occurrence, which raises that task's priority rather than creating a
   new one. A report that matches a closed task is a regression, which is worse
   than a new defect and must say so.
2. `memory_search` for a known cause. If the answer is recorded, this is an
   answer, not a task.
3. Establish three things before assigning a severity: how many are affected,
   whether there is a workaround, and whether anything is lost permanently.
4. Assign severity on that, not on tone:
   - **1** — data lost or exposed, or nobody can work. No workaround.
   - **2** — a core path is broken, with a workaround that costs real effort.
   - **3** — something is wrong but the work can continue.
   - **4** — cosmetic, or a request rather than a fault.
   - **5** — an idea worth keeping, with no claim on this cycle.
5. `task_label` with the severity and the area, so the board can be filtered
   without opening every task.
6. State what would raise the severity. "Currently 3; becomes 1 if it also
   affects the export path" tells the next reader what to watch.

## Constraints

- Urgency expressed by the reporter is data about the reporter, not about the
  defect. Record it, do not price on it.
- Never assign severity 1 to something with a workaround. Doing it twice makes
  the level meaningless and the real one gets ignored.
- Do not guess a root cause to justify a severity. Severity is about impact,
  which is observable; cause is not, at this stage.
- If you cannot determine impact, say so and assign 3 provisionally rather than
  inventing a number.

## Output Contract

A severity with the three facts behind it — reach, workaround, permanence —
labels applied, any duplicate or regression named by task id, and one line
saying what would change the rating.

## Examples

Good: "Severity 2. Affects every account with more than 1000 records (roughly
15% by the last count), workaround is to export in batches, nothing is lost.
Duplicate of #212, which was closed three weeks ago — this is a regression, so
it goes back to the role that closed it. Becomes 1 if the partial export is
silently truncated rather than erroring; unverified."

Bad: "Severity 1, customer is very unhappy."
