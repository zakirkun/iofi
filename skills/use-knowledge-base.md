---
slug: use-knowledge-base
version: 1
applies_to: [cto, tech-leader, lead-programmer, qa, devops, business-analyst, business-process, finance, sprint-manager, support-ticket]
tools_required: [knowledge_search, knowledge_list]
priority: 5
description: Check the Founder's uploaded documents before deciding anything they might already answer
---

## Purpose

Find out whether the answer is already written down in a document the Founder
uploaded, before spending a run deriving it or a meeting debating it.

## When To Use

Before any decision, estimate, specification or implementation choice that a
contract, standard, brand guideline, API specification or runbook could govern.
Also whenever a task mentions a document by name.

## Procedure

1. `knowledge_search` with the specific question, not the topic. "retention
   period for customer records" finds the clause; "data policy" finds the
   table of contents.
2. Read what comes back including the document title and section shown with
   each passage. That is how you cite it later.
3. If a passage looks like the right area but is cut off, call `knowledge_get`
   with its `doc_id` to read the surrounding sections.
4. If nothing matches, call `knowledge_list` before concluding the company has
   no material on it. A relevant document may be titled in a way your query
   missed, and its presence changes your next step.
5. Search again with wording taken from the document itself. Uploaded material
   uses its own vocabulary, and the second search is usually the one that hits.
6. If the material genuinely does not cover it, say so explicitly in whatever
   you produce, and continue with the rest of the procedure you were following.

## Constraints

- An uploaded document outranks a memory. If `memory_search` and
  `knowledge_search` disagree, follow the document and record the conflict with
  `memory_write` so the stale memory gets corrected.
- Never paraphrase a requirement you did not read. Quote the passage and name
  its document and section.
- Do not treat an empty result as permission to invent a rule. It means nobody
  has written one down, which is a fact worth stating, not a gap to fill
  silently.
- A document listed as `pending` is still being extracted; a document listed as
  `failed` was never readable. Neither means the company has no position — say
  which you saw.

## Output Contract

Every requirement you assert from uploaded material carries its source, in the
form `document title › section`. Where the material was silent on something you
had to decide anyway, one line naming the assumption you made instead.

## Examples

Good: "The retention clause is 24 months, not 12 — Data Processing Agreement ›
Retention. The task spec says 12; I have moved it to blocked and asked the
business-analyst which is authoritative."

Good: "knowledge_list shows no security standard uploaded. I applied OWASP ASVS
L1 as the assumption and noted it in the task."

Bad: "Per our data policy, 12 months." — no document named, and the number does
not match the clause.
