---
slug: defect-report
version: 1
applies_to: [qa, support-ticket]
tools_required: [task_create, task_label, kanban_move]
priority: 15
description: File a defect the fixer can reproduce without asking you a single question
---

## Purpose

Turn something that is wrong into a task somebody can pick up cold and fix,
without a conversation.

## When To Use

An acceptance criterion failed, a test broke, or a report describes behaviour
that does not match the spec.

## Procedure

1. Reproduce it. Write down the exact steps, the exact input, and the exact
   output. If you could not reproduce it, that is the finding — file it saying
   so, with what you tried.
2. Establish what should have happened and where that is written: the task's
   acceptance criteria, an uploaded document, or a recorded convention. A defect
   is a gap against a stated expectation, not against your preference.
3. Narrow it before filing. Does it happen with the minimum input? On the base
   branch too? If it reproduces on the base branch, it is a pre-existing defect
   and belongs to a separate task, not to the branch under review.
4. `task_create` with: steps to reproduce, expected, actual, where the
   expectation comes from, and how often it happens. Set `role_slug` to the role
   that should fix it, and `priority` from real impact — 1 is data loss or a
   security hole, not "I would like this soon".
5. `task_label` it so the board can be filtered: `defect`, plus the area.
6. `kanban_move` the original task back to the column that owns the fix, naming
   the defect task in the reason.

## Constraints

- One defect per task. Three findings in one task get one fix and two forgotten.
- Never name a cause you did not verify. "The cache is broken" sends the fixer
  to the wrong file; "the second request returns the first request's body"
  sends them to the right one.
- Never file a preference as a defect. If the behaviour matches the spec and the
  spec is wrong, the task is a spec change, addressed to the business-analyst.
- A criterion you could not verify is a fail, not a pass. Say which and why.
- Do not fix it yourself if fixing is not your role. Report it.

## Output Contract

One `task_create` per defect containing reproduction steps, expected, actual and
the source of the expectation; labels applied; the original task moved back with
the defect ids named in the reason.

## Examples

Good: "Steps: POST /api/knowledge with a 0-byte file. Expected: 400 naming the
empty file (spec: acceptance criterion 3). Actual: 202 and a document stuck in
`pending` forever. Reproduces every time, also on main. Priority 2 — it leaves
rows nothing ever clears."

Bad: "Upload is broken sometimes." — no input, no expectation, no frequency.
