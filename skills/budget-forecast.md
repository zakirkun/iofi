---
slug: budget-forecast
version: 1
applies_to: [finance]
tools_required: [cost_report, run_list, task_create]
priority: 20
description: Project spend from measured rates, and say what would make the projection wrong
---

## Purpose

Give a forward number the Founder can plan against, with its assumptions visible
enough to be argued with.

## When To Use

Before committing to a workload, when spend is trending, or when asked what
something will cost.

## Procedure

1. Start from measurement. `cost_report` over at least two comparable periods —
   a forecast from one period is a guess with a decimal point.
2. Find the unit that actually drives cost: usually cost per completed task, not
   per run, since retries and failures are part of what a task costs.
3. Compute the rate and state its sample size. "$0.42 per completed task over 31
   tasks" carries its own confidence; "$0.42 per task" does not.
4. Project by multiplying the rate by the planned volume. State the volume
   assumption separately from the rate — one of the two is always the thing that
   turns out wrong.
5. Give a range, not a point. Use the observed best and worst periods as the
   bounds. A single number will be quoted back as a commitment.
6. Name the three things that would break the forecast: a model price change, a
   change in failure rate, a change in task size. Say which way each moves it.
7. If the projection crosses a threshold worth acting on, `task_create` for the
   action now rather than reporting it and waiting.

## Constraints

- Never forecast from a single period, and never from a period containing an
  incident, without saying so.
- Never present a projection without its assumptions in the same paragraph. They
  get separated the moment the number is quoted.
- Do not apply a growth rate nobody has stated as a plan.
- Do not include estimated figures in a projection without carrying the
  estimated label all the way through to the total.
- Do not recommend a spending limit without saying what stops when it is hit.

## Output Contract

A range with its rate, sample size and volume assumption stated separately; the
three sensitivities named with their direction; and a task created for any
action the projection makes necessary.

## Examples

Good: "Rate: $0.42 per completed task, measured over 31 tasks across two 7-day
periods, all actual. At the planned 40 tasks/week the range is $14–$22/week,
bounded by the best and worst observed periods. Sensitive to: failure rate —
currently 12%, each 10 points adds roughly $2/week; task size — the median task
is 3 files, larger ones cost more than proportionally; model pricing, outside our
control. Created task #88 to cap `devops-1` retries, which is most of the
failure spend."

Bad: "Should be around $20 a week going forward."
