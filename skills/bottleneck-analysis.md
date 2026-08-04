---
slug: bottleneck-analysis
version: 1
applies_to: [business-process, sprint-manager]
tools_required: [task_list, run_list, memory_write]
priority: 20
description: Find the one step that sets the pace, and propose the smallest change that moves it
---

## Purpose

Identify where throughput is actually limited, and change that, rather than
improving a step that was never the constraint.

## When To Use

Delivery is slower than expected, work piles up somewhere, or someone proposes a
process change without evidence that it targets the limit.

## Procedure

1. Look for the queue, not the busy step. Work waiting in front of a step is the
   signal; a step that is busy but never has a queue is not the constraint.
   `task_list` per column and `board_get` for the WIP limits show both.
2. Separate waiting from working. A task in `in_progress` for 20 hours with one
   event at hour 19 waited 19 hours; it did not take 20.
3. Check the automated half too. `run_list state=failed` — a step that succeeds
   only half the time has double its apparent cost, and reruns are invisible in
   column dwell time.
4. Confirm it is the constraint before proposing anything: if this step got
   twice as fast, would total time halve? If not, it is not the bottleneck.
5. Propose the smallest change that relieves it, in this order: remove the step,
   remove the wait in front of it, raise its capacity, split it. Adding capacity
   is the most expensive option and usually the first one suggested.
6. Say what will become the new constraint afterwards. Every relieved bottleneck
   moves to the next step, and a proposal that does not name the successor will
   be read as a fix rather than a step.
7. `memory_write` the measurement so the next analysis compares against it.

## Constraints

- Never name a bottleneck without the number that identifies it. An opinion
  about which step feels slow is not evidence.
- Do not propose more parallelism at a step that is starved of input; it will
  idle harder.
- Do not count a single outlier as a pattern. Three occurrences or a median.
- A human or external wait is a floor, not a bottleneck to optimise. Say so
  rather than proposing to speed it up.
- Do not propose a change whose effect you cannot measure afterwards.

## Output Contract

The constraint named with its measurement and sample size, the smallest proposed
change, the predicted effect on total time, the step that becomes the next
constraint, and the number to re-check afterwards.

## Examples

Good: "Constraint is the wait before QA, not QA itself: median 27h queued
against 4h verifying, across 24 tasks. One QA agent, capacity 1, and 6 tasks
arrived in a day. Smallest change: raise QA capacity to 2 — an agent row, no
process change. Predicted: median total drops from 31h to about 8h. The next
constraint becomes review, currently at 6h median. Re-check the queued figure
after 20 more tasks."

Bad: "QA is slow, we should add more QA."
