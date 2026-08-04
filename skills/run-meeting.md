---
slug: run-meeting
version: 1
applies_to: [cto, tech-leader, sprint-manager, business-analyst, business-process]
tools_required: [meeting_open, meeting_read, meeting_close]
priority: 20
description: Open a discussion that reaches a decision and is closed with it recorded
---

## Purpose

Use a meeting only where one is the cheapest way to settle something, and end it
with a decision that outlives the transcript.

## When To Use

A question needs more than one role's input and `ask_role` has failed or would
need three separate answers reconciled. Not for anything one role owns.

## Procedure

1. `meeting_list` first. An open meeting on the topic is where this belongs; a
   second one splits the discussion and neither reaches a decision.
2. Decide what must be settled and write it as questions, not as a topic. "Which
   of soft delete or hard delete, given the audit requirement" ends; "data
   deletion approach" does not.
3. Invite only the roles whose position changes the outcome. Every extra
   participant is a round of tokens and a chance to widen the scope.
4. `meeting_open` with the agenda as those questions. The facilitator runs it
   from there; you do not have to drive each round.
5. `meeting_read` as it progresses. Watch for the two failure modes: agreement
   reached in round one and then re-litigated, and a question nobody answered
   because everyone answered a different one.
6. `meeting_post` only to add a position with a reason, or to bring the
   discussion back to an unanswered agenda question.
7. `meeting_close` with the decision: what was chosen, why, and what was rejected
   and why. Closing records it as a memory automatically, which is the whole
   point of holding the meeting.
8. Create the tasks the decision implies. A decision with no task is a decision
   that will be made again.

## Constraints

- Never open a meeting to inform people. That is a memory or a task comment.
- Never leave a meeting open. An unclosed meeting has no decision, so the
  discussion is lost and the next agent repeats it.
- Never close with a summary instead of a decision. "We discussed the tradeoffs"
  is not an outcome anyone can act on.
- Record the rejected option and its reason. Without it, the decision gets
  reopened by whoever thinks of the rejected option next.
- Do not exceed the agenda. A question that arrives mid-meeting becomes its own
  task, not a fifth round.

## Output Contract

One `meeting_close` stating the decision, its reason, and the rejected
alternative with why, plus the tasks the decision implies, created and assigned.

## Examples

Good close: "Decision: soft delete with a 24-month purge. Reason: the Data
Processing Agreement requires records to be recoverable for audit within 24
months, which hard delete cannot satisfy. Rejected: hard delete with an audit
log — the log would hold the same personal data with none of the deletion
guarantees, so it moves the problem rather than solving it. Tasks #120 (schema),
#121 (purge job) created."

Bad: "Good discussion, everyone agreed soft delete is probably better."
