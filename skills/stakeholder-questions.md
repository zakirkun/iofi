---
slug: stakeholder-questions
version: 1
applies_to: [business-analyst, business-process]
tools_required: [knowledge_search, ask_role, escalate_founder]
priority: 25
description: Ask the few questions that change the work, in a form that gets answered
---

## Purpose

Extract the decisions only the Founder or another role can make, in one round
rather than five.

## When To Use

A request arrives that cannot be specified as it stands, or a specification is
blocked on something nobody has decided.

## Procedure

1. Answer everything you can yourself first. `knowledge_search` for uploaded
   policy, `memory_search` for prior decisions, `ask_role` for anything a role
   owns. What survives all three is the real question list.
2. Keep only the questions whose answers change the work. If both answers lead
   to the same task, it is curiosity, not a blocker — record it as an assumption
   and move on.
3. Rank what remains. Ask at most three at once; a list of ten gets one reply
   answering the easiest.
4. Give each question its options and their consequences. People choose between
   two named things far faster than they compose an answer from nothing.
5. State your recommendation and what you will do if nobody answers. A default
   makes silence productive instead of blocking.
6. Ask in the right place: `ask_role` for anything a role owns,
   `escalate_founder` only for commercial calls, priorities and external
   commitments.
7. Record the answer where the work is, not only in the conversation.

## Constraints

- Never ask a question whose answer is in an uploaded document. It trains people
  to stop reading your questions.
- Never ask an open question when a choice will do. "Which of A or B" gets an
  answer; "what are your thoughts on pricing" gets nothing.
- Never batch a blocking question with a non-blocking one; the reply will cover
  the easy half.
- Do not stall the whole task on an answer. Specify everything that does not
  depend on it, and mark the part that does.

## Output Contract

At most three questions, each with options, consequences, your recommendation
and the default you will apply if unanswered. Everything else resolved and
recorded as an assumption, with the work that does not depend on the answers
already specified.

## Examples

Good: "One blocker. Do lapsed accounts that return keep their 2023 price?
(a) Yes — roughly $4k/yr forgone, no migration needed. (b) No — 40 accounts see
a price rise and need notice. Recommend (b) with 60 days' notice. If nobody
answers by Thursday I will specify (b), since it is reversible and (a) is not."

Bad: "Can you clarify the requirements around pricing for returning customers?"
