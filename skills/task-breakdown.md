---
slug: task-breakdown
version: 2
applies_to: [tech-leader, sprint-manager, business-analyst, cto]
tools_required: [task_split, task_depend, task_search, memory_search, knowledge_search]
priority: 10
description: Split a request into tasks small enough that one agent finishes one in a single run
---

## Purpose

Turn a request into tasks an agent can finish in one focused run, ordered so
nobody waits on work that has not started.

## When To Use

The request touches more than one role, or would take an agent more than a
single session, or its acceptance criteria cannot be checked in one pass.

## Procedure

1. `task_search` the request first. Work already on the board — open or closed —
   changes the split, and a second copy of an existing task wastes a whole run.
2. `memory_search` and `knowledge_search` for the area. A recorded convention or
   an uploaded specification changes how the work divides.
3. Write the outcome in one sentence. If you cannot, the request is not
   understood yet — ask before splitting.
4. List the observable changes that outcome requires. Each becomes a candidate
   task.
5. Merge any candidate that cannot be verified on its own into its neighbour.
   A task whose only proof is another task's completion is not a task.
6. Assign each a `role_slug`. A task no role owns is a task nobody starts.
7. Give each acceptance criteria phrased as behaviour, never as implementation.
8. Order them with `task_depend`, after the split returns the new ids. `task_split`
   creates tasks but does not order them: without this step the board will hand
   out the second task before the first one exists, and nothing will report why
   it failed. Link only real prerequisites — the board refuses to let an agent
   claim a task whose dependencies are open, so a link added for preference
   stalls real work.

## Constraints

- No task without acceptance criteria.
- No task that requires two roles to be in progress at once.
- Do not split below a run's worth of work: five tasks of ten minutes each cost
  more in setup than one task of an hour.
- Do not invent scope the request did not ask for.

## Output Contract

One `task_split` call — each subtask carrying `title`, `spec`, `role_slug` and
`acceptance` — followed by `task_depend` for every real prerequisite, and a
`task_update` on the parent stating how you split it, what order the pieces run
in, and what you deliberately left out.

## Examples

Request: "users can reset their password".

- `business-analyst` — Define the reset flow states and the failure messages.
  Acceptance: every state has a named message and a defined next state.
- `lead-programmer` — Implement token issue and verification.
  Acceptance: a token is single-use, expires in 30 minutes, and a reused token
  returns 400 with no account information.
- `qa` — Verify expiry, reuse, and unknown-address behaviour.
  Acceptance: all three cases have a recorded result.

Not a split: "write the backend" and "write the frontend" — neither can be
verified alone.
