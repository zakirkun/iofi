---
slug: sprint-plan
version: 1
applies_to: [sprint-manager]
tools_required: [task_list, task_depend, task_assign]
priority: 10
description: Commit to work the company can actually finish, in an order that does not deadlock
---

## Purpose

Choose what goes into the cycle, put it in a buildable order, and give it to
people who have room for it.

## When To Use

At the start of a cycle, and whenever the committed set stops matching reality.

## Procedure

1. Measure capacity before choosing work. `agent_roster` gives who exists and
   who has room; the last cycle's completed count gives what that has actually
   meant. Plan to the measured number, not to the headcount.
2. `task_list` the backlog and pick by priority, then by whether the task is
   ready: a task with no acceptance criteria is not ready and will come back.
3. Order what you picked. `task_depend` for anything that genuinely must wait —
   a shared migration, an API another task consumes. Ordering that exists only
   in your head produces work built in the wrong sequence.
4. Check the ordering does not serialise the whole cycle. If every task depends
   on one task, that task is the cycle and everyone else idles.
5. `task_assign` to named agents, not to roles, for anything that must finish.
   A role-only task waits for someone to claim it.
6. Leave slack. A cycle planned to full capacity absorbs no defect and no
   incident, and both arrive.
7. Say what you are explicitly not doing this cycle. Unspoken exclusions get
   assumed in.

## Constraints

- Never commit more than the measured throughput. Optimism here is paid for by
  everything slipping rather than by one thing being dropped.
- Never plan a task with no acceptance criteria; send it back to be specified.
- Do not assign work to an agent already at capacity — check `has_capacity` from
  `agent_roster` rather than assuming.
- Do not change a task's acceptance criteria to make it fit the cycle. Split it.
- Do not create a dependency to express a preference about order; only real
  prerequisites belong there, because the board enforces them.

## Output Contract

A committed list with each item assigned to a named agent, dependencies set for
real prerequisites only, capacity stated with the measurement behind it, slack
left explicitly, and a named list of what is deferred.

## Examples

Good: "Capacity: 4 agents with room, last cycle completed 9 tasks. Committing 7,
leaving 2 slots of slack. #101 blocks #102 and #104 — it adds the column both
read — so #101 goes to lead-1 first. #103 and #105 are independent, assigned to
lead-2 and devops-1. Deferred and not started: #110, #111, the export rework."

Bad: "Pulled the top 12 from the backlog for this sprint."
