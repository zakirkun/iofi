---
slug: architecture-decision
version: 2
applies_to: [cto, tech-leader]
tools_required: [memory_search, memory_write, memory_update, knowledge_search]
priority: 20
description: Make a technical decision that survives contact with the next agent who reads it
---

## Purpose

Reach a technical decision and record it so the next agent applies it without
asking, and knows what was rejected and why.

## When To Use

A choice affects more than one task, is expensive to reverse, or two roles
disagree about it.

## Procedure

1. `memory_search` for prior decisions in the area. A new decision that
   contradicts an old one without addressing it is a defect, not a decision.
2. `knowledge_search` for an uploaded constraint that decides it outright — a
   compliance requirement, a licence policy, a contracted interface. Deciding
   against one of those is not a tradeoff, it is a mistake.
3. State the question as a single sentence with a yes/no or a named choice.
4. List the options actually available here, not in general. Two is usually
   enough; more than four means the question is too broad.
5. For each option name the cost you would accept — not only the benefit.
6. Choose. Say what you traded away.
7. `memory_write` kind `decision` with the question, the choice, the rejected
   option, and the reason.
8. If this decision supersedes an earlier one, `memory_update` that memory with
   the reason rather than leaving two records that disagree. An agent that finds
   both will follow whichever ranks higher, which is not a decision anyone made.
9. Create tasks for what the decision now requires.

## Constraints

- No decision without a named rejected alternative.
- No decision that only lists benefits.
- Do not decide something a single task could settle by trying it.
- Reversibility beats theoretical fit: prefer the choice that is cheap to undo
  when the evidence is thin.

## Output Contract

One `memory_write` of kind `decision` shaped as:
`Question / Chosen / Rejected / Because / Revisit when`.
Plus the tasks the decision creates.

## Examples

```
Question: Do agent workspaces get a container each, or a git worktree?
Chosen: git worktree per agent.
Rejected: container per agent.
Because: isolation we need today is index-level, not process-level; a container
image carrying four coding CLIs plus their auth is a week of setup for a threat
we do not have yet.
Revisit when: we run untrusted repositories, or an agent needs a resource limit.
```
