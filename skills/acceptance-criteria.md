---
slug: acceptance-criteria
version: 1
applies_to: [business-analyst, tech-leader, qa]
tools_required: [task_get, task_update, knowledge_search]
priority: 15
description: Write criteria QA can verify alone, and reject the ones nobody can check
---

## Purpose

Produce the list that decides whether a task is done, in a form that does not
require the author to be present.

## When To Use

Writing a task, or reviewing one before it is dispatched. Also when QA cannot
verify something and the criteria are the reason.

## Procedure

1. `task_get` and read the spec. Criteria that do not follow from a stated
   outcome are scope arriving through the side door.
2. Write each criterion as: given this input or state, this observable thing
   happens. One behaviour per line.
3. Cover the three cases every criteria list omits: the empty input, the
   duplicate request, and the unauthorised caller.
4. Give every number a source. `knowledge_search` for the document that sets it;
   if none exists, mark it as an assumption in the spec, not as a fact in the
   criteria.
5. Apply the alone test to each line: could QA check this with only the task and
   the running system? If they would need to ask what was meant, rewrite it.
6. State what is explicitly not required, where a reader would reasonably assume
   it. This prevents both over-building and a QA rejection for something never
   asked for.
7. `task_update` with the criteria replacing the old ones, and say what changed.

## Constraints

- Never write "works correctly", "handles errors gracefully", "performs well",
  or "is user-friendly". Replace each with the observation that would prove it.
- Never write a criterion about the implementation — "uses a transaction" is not
  observable and locks the builder out of a better approach. If the property
  matters, state the property: "two concurrent claims leave exactly one winner".
- A criterion that requires new tooling to check is not ready; either simplify
  it or make building the check part of the task.
- Do not merge two behaviours into one line to shorten the list. Each becomes
  half-verified.

## Output Contract

A numbered list on the task, every line independently checkable, every number
sourced, including the empty case, the repeat case and the permission case, plus
an explicit not-required list where relevant.

## Examples

Good:
1. POST with a 0-byte file returns 400; the body names the file; no row created.
2. Uploading identical bytes into the same scope twice leaves one document, and
   its title is the one from the second upload.
3. A role-scoped document does not appear in `knowledge_search` for another role.
4. Not required: OCR of scanned pages. Out of scope for this task.

Bad:
1. Uploads work reliably.
2. Errors are handled.
