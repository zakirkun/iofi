---
slug: observability
version: 1
applies_to: [devops, tech-leader]
tools_required: [run_list, run_tail, memory_write]
priority: 25
description: Make a failure answer the question "what happened" without a second reproduction
---

## Purpose

Ensure that when something breaks, the recorded evidence is enough to find the
cause — instead of needing to reproduce it to learn anything.

## When To Use

After any incident, when adding a component that can fail silently, or when a
task asks why something failed and the answer is not in the record.

## Procedure

1. Start from a real failure. `run_list state=failed` for the recent ones, then
   `run_tail` the worst. What you could not determine from those events is
   exactly what is missing.
2. For each gap, decide which of three things fixes it: a log line with the
   identifier that ties events together, a counter that shows how often it
   happens, or an error message that names the input rather than the symptom.
3. Log the identifier, not the prose. `run=abc123 agent=qa-1 task=42` is
   greppable; "Something went wrong processing the task" is not.
4. Log at the boundary, where the process meets something it does not control:
   the database, another service, the filesystem, a spawned CLI. Inner functions
   returning errors upward do not each need a line.
5. Include the value that decided the branch. An error saying a limit was
   exceeded must say the limit and the actual number.
6. Check the failure is visible where someone looks. An error recorded only in a
   log file nobody reads is not observable; a state on the row is.
7. `memory_write` of kind `convention` for the field names, so two components do
   not call the same identifier `run_id` and `runID`.

## Constraints

- Never log a secret, a token or personal data. Log whether a value was present
  and its length.
- Do not log inside a loop over rows. One line per operation, with a count.
- An error that is caught and logged but not returned is an error that silently
  did nothing. Decide which it is.
- Do not add a metric with no threshold. A number nobody has an opinion about is
  a number nobody looks at.
- Resist adding a tracing system to answer a question one log line answers.

## Output Contract

A change that makes the specific failure you started from diagnosable from the
record alone, a `task_update` quoting the before and after evidence for that
failure, and a `memory_write` of kind `convention` for any field naming
introduced.

## Examples

Good: "Run 8f3 failed with `error: failed`. The adapter discarded the CLI's
stderr. Now the last 20 lines of stderr are recorded as a run event and the exit
code is stored, so `run_tail` shows `exit=127 command not found: opencode`. Same
failure now diagnosable without rerunning it."

Bad: "Added debug logging to the runner." — which failure, and is it answerable
now.
