---
slug: tech-radar
version: 1
applies_to: [cto]
tools_required: [knowledge_search, memory_write, meeting_open]
priority: 25
description: Decide whether to adopt a technology, and record the decision so it is not reopened monthly
---

## Purpose

Rule on whether the company adopts, trials, holds or drops a technology, with
the reasoning attached so the question stays settled.

## When To Use

A role proposes a new dependency, framework, service or language; or an existing
one is causing enough trouble to reconsider.

## Procedure

1. `memory_search` for a prior ruling. Most proposals are the second or third
   time; if a ruling exists and nothing has changed, the answer is the ruling.
2. Name what problem it solves, in the company's own terms. A technology with no
   named problem is a preference, and the answer is no.
3. Check what already solves it: something already in the codebase, the standard
   library, a platform feature, or a dependency already installed. Adoption is
   only on the table once those are ruled out with a reason.
4. `knowledge_search` for a constraint that decides it — a licence policy, a
   hosting restriction, a compliance requirement. This can end the question
   before any technical discussion.
5. Price the real cost: what it adds to the build, what it does when it breaks,
   who can debug it at 3am, and what removing it later would take. The last one
   is the one that is never asked and always matters.
6. Rule in one of four ways and say which: **adopt** (default choice for this
   problem), **trial** (one bounded task, with the criterion that decides it),
   **hold** (not now, and what would change that), **drop** (remove what exists,
   with the migration path).
7. If the decision changes architecture rather than just tooling, `meeting_open`
   with the affected roles before ruling.
8. `memory_write` of kind `decision`, global scope, naming the ruling, the
   rejected alternative, and what would justify revisiting.

## Constraints

- A dependency added for one function is not adoption, it is a cost with no
  buyer. Say so plainly.
- Never rule on popularity, novelty or a benchmark that does not reflect this
  workload.
- Never approve a trial without the criterion that ends it. An open-ended trial
  is adoption without a decision.
- Do not rule where you have no authority to enforce it. A ruling nobody
  implements is worse than no ruling, because it looks settled.
- Say what the ruling costs the person who wanted the other answer. A decision
  presented as free is not believed.

## Output Contract

A ruling of adopt, trial, hold or drop; the problem it addresses; what already
solved it and why that was insufficient; the removal cost; and a `memory_write`
of kind `decision` carrying all of it plus what would justify reopening.

## Examples

Good: "Ruling: hold on adding a workflow engine. Problem stated was ordering
between tasks, which `depends_on` and `task_depend` already handle at the board
level. The engine adds a service to operate and a second place work can be
defined, and removing it later means rewriting every definition. Revisit if we
need retries with backoff across process restarts, which the board genuinely
cannot express. Rejected: adopting now to 'get ahead of it' — that buys the
operational cost immediately and the benefit never."

Bad: "Approved, it's the standard choice these days."
