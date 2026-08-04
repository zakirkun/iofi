---
slug: customer-reply
version: 1
applies_to: [support-ticket]
tools_required: [memory_search, knowledge_search, task_comment]
priority: 25
description: Answer a reporter with what is known, never with a guess dressed as a cause
---

## Purpose

Give the person who reported a problem a reply that is true, useful, and does
not commit the company to something nobody agreed to.

## When To Use

Any outbound message to whoever reported an issue — an acknowledgement, an
answer, or an update on something still open.

## Procedure

1. `memory_search` and `knowledge_search` before writing a word. A recurring
   report usually has a recorded answer, and an uploaded document may set the
   policy you are about to describe.
2. Separate what you know from what you suspect, and put only the first in the
   reply.
3. Structure it: what you understood the problem to be, what is confirmed, what
   happens next, and by when you will next say something. The last one is the
   part that stops a second report arriving tomorrow.
4. If it is fixed, say what changed in the reporter's terms — what they will now
   see — not in terms of the commit.
5. If it is not fixed, say what is being done and what is not. "We are not
   changing X" is a better answer than silence about X.
6. `task_comment` the reply you sent onto the task, so the next agent sees what
   the reporter has already been told.

## Constraints

- Never state a root cause that has not been verified. "We are investigating"
  is accurate; "it was a caching issue" when nobody confirmed that is a lie you
  will have to correct.
- Never promise a date the board does not support. Commit to the next update,
  not to the fix.
- Never quote an internal task id, an agent name, a branch or a stack trace.
- Do not apologise repeatedly. Say it once and move to what happens next.
- If the answer depends on a commercial decision nobody has made, do not invent
  one — escalate and tell the reporter it is being decided.

## Output Contract

A reply containing: the problem as understood, confirmed facts only, the next
concrete step, and when the next update comes. A `task_comment` recording what
was sent.

## Examples

Good: "Confirmed: exports containing more than 1000 records stop at 1000 without
warning. That is a defect, not a limit, and it is being fixed. Until then,
exporting by date range stays complete. I will update you on Thursday whether or
not it has shipped."

Bad: "Sorry about that! Looks like a caching issue on our end, should be sorted
soon." — unverified cause, no confirmation of the actual problem, no date.
