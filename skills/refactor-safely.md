---
slug: refactor-safely
version: 1
applies_to: [lead-programmer, tech-leader]
tools_required: [repo_diff, task_update, memory_search]
priority: 30
description: Change the shape of code without changing what it does, in a diff a reviewer can check
---

## Purpose

Restructure code so the next change is cheaper, while keeping the diff small
enough that a reviewer can confirm nothing else moved.

## When To Use

A task explicitly asks for it, or the change you were assigned is impossible to
make cleanly without it. Never as an unrequested addition to another task.

## Procedure

1. Confirm the behaviour is covered by tests before you touch it. If it is not,
   write those tests first and commit them separately — that commit is the proof
   the refactor changed nothing.
2. `memory_search` for a convention governing the area. Restructuring against a
   recorded convention is a defect, not an improvement.
3. Do one kind of change per commit: rename, or move, or extract, or reorder.
   A commit that renames and reorders is unreviewable, because every line looks
   changed.
4. Keep behaviour identical. If you find a bug mid-refactor, stop, note it, and
   fix it in its own commit — a behaviour change hidden inside a rename is how
   defects reach production unreviewed.
5. Run the suite after each commit, not at the end.
6. `repo_diff summary=true`. If the file count surprises you, the refactor grew
   past its task.

## Constraints

- Do not introduce an interface with one implementation, a factory for one
  product, or a config value that never changes. Those are costs with no buyer.
- Do not reformat untouched lines. A whitespace change across a file buries the
  three lines that matter.
- Deleting code is a better outcome than restructuring it. Check whether the
  thing is used at all before improving it.
- Stop when the assigned task is possible. "While I was in there" is how a
  one-file task becomes an unreviewable branch.

## Output Contract

A branch of small single-purpose commits, the test suite green at each one, and
a `task_update` stating what shape changed, what stayed identical, and how you
know — usually "the same tests pass, untouched".

## Examples

Good: "Three commits: (1) added `TestComposePrompt_IncludesMemories` covering
existing behaviour, (2) extracted `recallKnowledge` out of `composePrompt` with
no logic change, (3) moved the two helpers below their caller. Suite green after
each. No behaviour change."

Bad: "Refactored the runner and fixed a nil check while I was there." — the
reviewer now cannot tell which line was the fix.
