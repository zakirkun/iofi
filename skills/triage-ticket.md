---
slug: triage-ticket
version: 2
applies_to: [support-ticket, business-process, finance]
tools_required: [memory_search, knowledge_search, task_search, task_create, memory_write]
priority: 20
description: Turn an incoming report into either an answer or a task, never into a guess
---

## Purpose

Convert an incoming report into one of two things: an answer backed by what is
known, or a task with enough detail that nobody has to come back to the
reporter.

## When To Use

Anything arrives from outside the company: a user report, a GitHub issue, a
question from the Founder.

## Procedure

1. Restate the report in one sentence. If you cannot, the report is incomplete —
   say exactly which fact is missing.
2. `task_search` the symptom. A match against an open task makes this a second
   occurrence rather than a new task; a match against a closed one makes it a
   regression, which is worse and must be labelled as such.
3. `memory_search` for the symptom and `knowledge_search` for the documented
   behaviour. A recurring report usually has a recorded cause, and "this is
   working as specified" is only sayable once you have read the specification.
4. Try to reproduce it. Record the result either way, including "could not
   reproduce, here is what I tried".
5. Classify severity by what the user cannot do, not by how the report sounds.
6. If memory or an uploaded document answers it, answer and stop. Otherwise
   `task_create` with the reproduction steps, the severity, and `role_slug` set
   to the owning role — `role_list` first if you are unsure of the exact slug.
7. `memory_write` kind `fact` when the same report has now arrived twice.

## Constraints

- Never state a root cause you have not confirmed. Say what is known.
- Never close a report as "works for me" without saying what you ran.
- Severity is about impact, not about volume of complaints.
- Do not assign to a role by guessing — if unsure, `ask_role`.

## Output Contract

Either a written answer citing the memory it came from, or one `task_create`
containing: symptom, reproduction steps, expected vs actual, severity, and
owning role.

## Examples

Answered from memory:
"Known: exports over 50k rows time out at the gateway, not in the exporter.
Recorded 2026-03-14. Workaround is date-range splitting. Fix tracked in #212."

Converted to a task:
"Symptom: password reset email never arrives for @outlook.com addresses.
Steps: request reset for an outlook.com account, wait 10 minutes.
Expected: email delivered. Actual: nothing, and no bounce in the log.
Severity: high — affected users cannot sign in at all. Owner: devops."
