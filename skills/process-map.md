---
slug: process-map
version: 1
applies_to: [business-process]
tools_required: [task_search, run_list, memory_write]
priority: 15
description: Describe how work actually flows, from the record rather than from how it is supposed to work
---

## Purpose

Produce an accurate picture of how a piece of work moves through the company, so
a change can be aimed at the step that matters.

## When To Use

Before proposing any process change, and whenever a delay is reported without a
named cause.

## Procedure

1. Map from evidence, not from description. `task_search` the class of work and
   read the task events: they record what actually happened and when, which is
   usually not what anyone would describe.
2. Write each step as four things: the trigger, the owning role, the input it
   needs, and the output it produces. A step missing any of the four is where
   work stalls.
3. Record the handoffs separately from the steps. Time is lost between steps far
   more often than inside them.
4. Measure, do not estimate. Task event timestamps give the real dwell time per
   column; `run_list` gives what the automated steps cost. A guessed duration
   produces a change aimed at the wrong step.
5. Mark every step that waits on something outside the company or on a human
   decision. Those set the floor on total time and cannot be optimised away.
6. Show the map to one role that lives in it before proposing anything. The step
   they say is missing is usually the expensive one.
7. `memory_write` the map with kind `convention` once it is confirmed, so the
   next analysis starts from it.

## Constraints

- Do not map the ideal process. The ideal one has no bottleneck, which is why it
  is useless for finding one.
- Do not merge two steps because one role owns both. If work waits between them,
  they are two steps.
- No step described as "review" or "approval" without saying what is checked and
  what causes a rejection. Those are where unmeasured time accumulates.
- Do not propose the change in the map. Describing and fixing in one document
  gets the description read as advocacy.

## Output Contract

A step list with trigger, owner, input and output for each; the handoffs marked;
measured dwell time per step with its source; and external or human waits flagged
as irreducible.

## Examples

Good: "Step 4 — QA verification. Trigger: task enters `qa`. Owner: qa. Input: a
branch and acceptance criteria. Output: pass, or a defect task per failure.
Measured dwell: median 31h across 24 tasks, of which 27h is before the first
event — the wait is for a free QA agent, not the verification itself."

Bad: "QA reviews the work and approves it. This takes a while."
