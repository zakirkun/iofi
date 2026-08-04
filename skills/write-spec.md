---
slug: write-spec
version: 1
applies_to: [business-analyst, tech-leader]
tools_required: [knowledge_search, task_create, task_update]
priority: 10
description: Turn intent into a brief an agent can build from without a second conversation
---

## Purpose

Write down what must be true when the work is done, in enough detail that the
builder never has to ask what was meant.

## When To Use

Any request that arrives as intent rather than as a task: a Founder request, a
meeting outcome, a business need.

## Procedure

1. `knowledge_search` first. A specification that contradicts an uploaded
   contract, standard or policy is worse than no specification.
2. Write the user-visible outcome in one sentence. If it takes three, there is
   more than one task here.
3. State the constraints that are real: a deadline, a format someone else
   consumes, a limit from a document, a system that cannot change. Do not invent
   constraints from preference.
4. List the assumptions you had to make, each on its own line, marked as
   assumptions. This is the most valuable part of a specification and the part
   most often skipped — an unstated assumption becomes a defect with no author.
5. Write what is explicitly out of scope. A builder with an unbounded brief will
   either build too much or ask.
6. Write acceptance criteria as observable behaviour. If nobody could check it
   without asking you, it is not a criterion yet.
7. `task_create` with the spec and acceptance, `role_slug` set to whoever should
   own it. If it spans roles, that is a signal it needs splitting first.

## Constraints

- Do not specify implementation. Say what must be true, not which library, table
  or function to use. Naming the solution removes the builder's ability to find
  a better one and hides the requirement behind it.
- Do not write a criterion containing "properly", "correctly", "as expected" or
  "user-friendly". None of those can be checked.
- Every number must have a source: a document, a decision, or an explicit
  assumption. A round number with no source is a guess presented as a
  requirement.
- If the intent itself is contradictory, do not resolve it silently. Name the
  contradiction and ask.

## Output Contract

A task whose spec contains: the outcome, the constraints with their sources, the
assumptions marked as such, the explicit out-of-scope list, and acceptance
criteria stated as observable behaviour.

## Examples

Good criterion: "Uploading a 0-byte file returns 400 and a message naming the
file; no document row is created."

Good assumption: "ASSUMPTION: 'recent' means the last 30 days. Nothing uploaded
defines it and the Founder's request did not say. Change this and the query
changes."

Bad: "The upload should handle errors properly and give good feedback." — no
input, no expected output, nothing checkable.
