---
slug: incident-command
version: 1
applies_to: [cto, devops, tech-leader]
tools_required: [task_create, kanban_move, memory_write]
priority: 10
description: Run a live incident so recovery comes first and the cause is found afterwards
---

## Purpose

Get the company out of a broken state quickly, with one person deciding, and
without losing the evidence needed to stop it happening again.

## When To Use

Something is broken now: data is being lost, a service is down, a credential
leaked, or a deploy made things worse.

## Procedure

1. Declare it. `task_create` one incident task at priority 1 with what is broken
   and since when. Everything about this incident goes on that task, so there is
   one place to read.
2. Take the decision role explicitly and say so. An incident with two people
   deciding takes longer than one with a wrong decider.
3. Stop the bleeding before understanding it. Roll back, disable the feature, or
   cut the traffic. Diagnosis in a broken environment costs more than the
   restore.
4. Preserve evidence before it is overwritten: copy the logs, note the deployed
   commit, capture the failing state. Rolling back destroys the thing you will
   need to explain it.
5. Communicate on a clock, not on progress. A short update at a fixed interval,
   even when the update is "no change", stops three people asking.
6. When it is recovered, say so explicitly and move the task out of the incident
   state. An incident nobody declares over stays open in everyone's head.
7. Only then find the cause. `task_create` a separate task for the fix, and one
   for the thing that would have caught it earlier.
8. `memory_write` of kind `pitfall` with what actually happened and the signal
   that would have shown it sooner.

## Constraints

- Never debug in production while it is still broken for users. Restore first.
- Never let the incident task become the fix task. Recovery and cause have
  different urgencies and different owners.
- Do not assign blame in any record. What failed is useful; who is not.
- Do not skip the write-up because it recovered quickly. A fast recovery from a
  cause nobody wrote down recurs.
- If a credential was exposed, rotation is not optional and not deferrable.

## Output Contract

One incident task carrying the timeline, one line stating when recovery was
confirmed and how, separate tasks for the cause and for the missing detection,
and a `memory_write` of kind `pitfall` naming the earliest signal that existed.

## Examples

Good: "Incident #341 declared 09:12 — all dispatches failing with `exit 127`.
Recovered 09:20 by rolling back to a1b2c3d; the new image dropped the `claude`
binary from PATH. Evidence: run events from 8f3 and 8f4 copied onto the task.
Cause task #342 (pin the tool paths), detection task #343 (smoke-test one
dispatch after deploy). Pitfall recorded: the health check only tested the
database, so a broken agent toolchain read as healthy."

Bad: "Fixed the outage, was a PATH thing." — no timeline, no detection gap, no
recorded pitfall.
