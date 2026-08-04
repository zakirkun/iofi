---
slug: cost-audit
version: 1
applies_to: [finance]
tools_required: [cost_report, run_list, memory_write]
priority: 10
description: Report what the company actually spent, with every figure labelled actual or estimate
---

## Purpose

Say what was spent, on what, over which period, without presenting a derived
number as a measured one.

## When To Use

Any question about spend, any periodic review, and before any decision that
depends on what something costs.

## Procedure

1. `cost_report` for the window. Read its note: figures from tools that report
   real usage are actual; the rest are priced from the configured rate table and
   are estimates. That distinction must survive into everything you write.
2. `run_list` to see where the money went in shape, not just in total: which
   agents, which states. A large number is usually a few large runs, and naming
   them is more useful than the average.
3. Separate failed spend from productive spend. `run_list state=failed` — tokens
   burned on runs that produced nothing is the number most worth acting on, and
   it is invisible in a total.
4. Give every figure its period and its basis. "$14.20" means nothing;
   "$14.20 actual, 7 days to 4 Aug, Claude Code runs only" means something.
5. Compare against the previous equivalent period, not against a target nobody
   set. A trend is actionable; a number alone is not.
6. Name the largest single contributor and what it bought. If it bought nothing
   — a failing agent retrying — that is the finding.
7. `memory_write` of kind `fact` with the period totals, so the next audit has a
   baseline instead of starting over.

## Constraints

- Never present an estimate as an actual. Label every figure, every time, even
  when it makes the report longer.
- Never quote a total without its window. A monthly figure read as weekly starts
  a wrong decision.
- Do not extrapolate from fewer than two comparable periods.
- Do not recommend a cut without saying what stops working. A cost with no named
  consequence reads as free to remove.
- Do not compute a per-unit cost until you can name the unit and count it.

## Output Contract

A report giving: the window, total spend split actual against estimate, spend on
failed runs stated separately, the largest contributor and what it produced, the
change against the previous equivalent period, and a `memory_write` recording
the totals as a baseline.

## Examples

Good: "7 days to 4 Aug. Total $18.40: $14.20 actual (Claude Code), $4.20
estimated (Codex, priced from the rate table). $6.10 of that — a third — went to
failed runs, 11 of 14 failures from agent `devops-1` retrying the same task
after a missing binary. Previous 7 days: $11.90, so the increase is failure, not
throughput. Baseline recorded."

Bad: "We spent about $18 last week, up a bit." — no basis, no split, no cause.
