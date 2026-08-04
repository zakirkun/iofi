---
slug: escalation-path
version: 1
applies_to: [cto, tech-leader, lead-programmer, qa, devops, business-analyst, business-process, finance, sprint-manager, support-ticket]
tools_required: [role_list, ask_role, escalate_founder]
priority: 5
description: Get an ambiguity answered by the cheapest party who can actually answer it
---

## Purpose

Turn "I am not sure what they meant" into an answer, without either guessing or
stalling the whole run on the slowest person in the company.

## When To Use

The spec admits two readings that lead to different work, a required decision is
outside your role's authority, or you have hit a dependency you cannot resolve.

## Procedure

1. Write the question down as one sentence with a definite answer. If you cannot,
   the problem is not that you lack an answer — it is that you have not isolated
   the question yet.
2. Search first. `knowledge_search` for uploaded material, `memory_search` for a
   prior decision. Most ambiguities were settled before and forgotten.
3. Still open: `role_list` to find the role that owns this, then `ask_role` with
   the specific question. Never guess a slug — a wrong one is rejected outright.
4. If that role's answer is itself uncertain, ask the role above it once. Two
   hops is the limit before it stops being a question and becomes a decision
   nobody owns.
5. Only if no role can answer — a budget, a priority between two things the
   Founder wants, a commitment to a third party — call `escalate_founder` with
   the question, what you already tried, and the options you see.
6. Do not wait. Continue with every part of the task that does not depend on the
   answer, and report what is outstanding in `task_update`.

## Constraints

- Never escalate without having asked a role first. `escalate_founder` rejects
  an empty `tried` field for that reason.
- One escalation per task per run. A second question stacked on an unanswered
  first one is noise.
- Give the Founder options, not an open question. "A or B, and here is what each
  costs" gets answered; "what should we do about pricing" does not.
- If you proceed on an assumption, state the assumption in the task. An unstated
  assumption is a defect that arrives later with no author.

## Output Contract

Either an answer recorded on the task with who gave it, or one
`escalate_founder` naming the question, the roles already asked, and the options
— plus a `task_update` describing what you completed regardless.

## Examples

Good: "Asked tech-leader whether soft delete or hard delete; they said soft, per
the audit requirement. Implemented soft delete. No escalation needed."

Good escalation: "Question: do we honour the 2023 pricing for accounts that
lapsed and returned? Tried: asked business-analyst (says it is a commercial
call, not a spec question) and finance (no policy recorded). Options: (a) honour
it, ~$4k/yr forgone, (b) do not, ~40 accounts affected."

Bad: "The spec is unclear so I picked the simpler option." — no question asked,
no assumption recorded.
