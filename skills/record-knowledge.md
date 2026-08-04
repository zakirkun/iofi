---
slug: record-knowledge
version: 1
applies_to: [cto, tech-leader, lead-programmer, qa, devops, business-analyst, business-process, finance, sprint-manager, support-ticket]
tools_required: [memory_search, memory_write]
priority: 15
description: Write what you learned so the next agent does not pay for it again
---

## Purpose

Leave behind the one thing that would have saved you an hour, in a form the next
agent will actually find.

## When To Use

At the end of every run that cost you time you could not have predicted, and
whenever you settle something others will hit.

## Procedure

1. `memory_search` for the same thing first. If it already exists and is right,
   write nothing. If it exists and is wrong, that is a correction, not a new
   memory — see Constraints.
2. Choose the kind honestly:
   - `decision` — a choice with a rejected alternative and a reason.
   - `convention` — a rule others must follow from now on.
   - `pitfall` — something that cost you time and will cost the next agent too.
   - `fact` — a stable truth about the system that is not obvious from reading it.
3. Title it as the thing itself, not as your experience of it. "Auth tokens
   expire at 15 minutes, not 60" beats "Problem with tokens".
4. Write the body so it stands alone. The reader will not have your task, your
   branch, or your conversation.
5. Choose the scope. `global` unless it is genuinely only true for your role or
   only for this board — an over-scoped memory is invisible to whoever needed it.
6. Set `confidence` honestly. Something you verified is 0.9; something you
   inferred once is 0.4. Low confidence still ranks and still helps.
7. Set `expires_in_days` for anything temporary — a workaround for a broken
   dependency, a rate limit during a migration.

## Constraints

- One memory, one thing. A memory covering three topics is found by none of the
  three queries that needed it.
- Never record a secret, a token, a password or a customer's data. Reference
  where it lives.
- Do not record what the repository already says. Code structure, file layout
  and function names are read faster from the source than from memory.
- If a memory is wrong, correct the record rather than adding a contradiction.
  Two memories that disagree make every future reader guess.

## Output Contract

Zero or more `memory_write` calls, each self-contained, correctly scoped, with a
kind that matches what it is and a confidence that matches how sure you are.

## Examples

Good pitfall: "The generated client retries 5xx but not 429. A rate-limited
call surfaces as a success with an empty body, which reads as 'no results'.
Check `resp.StatusCode` explicitly before trusting an empty list."

Good convention: "New HTTP handlers return errors through `fail(w, code, err)`,
never `http.Error`. The former emits the JSON shape the frontend parses; the
latter produces text that renders as an empty error banner."

Bad: "Had trouble with the API today, be careful." — names nothing, helps nobody.
