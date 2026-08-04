---
slug: root-cause-fix
version: 1
applies_to: [lead-programmer, devops, qa]
tools_required: [task_search, repo_diff, memory_search]
priority: 10
description: Fix the cause where every caller routes through it, not the symptom the ticket named
---

## Purpose

Make a defect stop happening everywhere it happens, in one change, instead of
patching the one path somebody noticed.

## When To Use

Any task that reports something behaving wrongly: a bug, a failing test, a
production incident, a QA rejection.

## Procedure

1. Reproduce it before reading any code. A defect you cannot trigger is one you
   cannot prove you fixed.
2. `task_search` the symptom. A defect reported twice is a defect whose first
   fix was in the wrong place, and the earlier task usually says where.
3. Trace the value backwards from where it is wrong to where it becomes wrong.
   Stop at the first function that produces the bad value rather than passing it
   on. That function is the cause; everything after it is a symptom.
4. Find every caller of that function before editing it. If four callers share
   the bug, one guard inside it is a smaller change than four guards outside it
   — and patching only the reported path leaves three still broken.
5. `memory_search` the area for a recorded pitfall. This may be known.
6. Write the fix at the cause. Write the test at the cause too — a test on the
   reported path passes for the wrong reason once the shared function is fixed.
7. `repo_diff` your own change before finishing. Any file in it that is not part
   of the cause is scope you added without being asked.

## Constraints

- Never widen a type, add a nil check at the call site, or swallow an error to
  make a symptom disappear. Those hide the defect from the next reporter.
- Never weaken or delete a test to make it pass.
- If the cause sits in code owned by another role, fix it anyway if it is small
  and say so in the summary; hand it off if it is not.
- If the real cause is a design problem rather than a bug, say that plainly and
  ask the tech-leader rather than working around it in three places.

## Output Contract

A change at the cause, one test that fails without it, a `task_update` naming
the cause and how many call sites were affected, and a `memory_write` of kind
`pitfall` if the cause was non-obvious.

## Examples

Good: "The expiry comparison in `auth.VerifyToken` used `<` rather than `<=`, so
a token expiring exactly on the boundary was accepted. All four callers were
affected, not just the password-reset handler the ticket named. Added
`TestVerifyToken_ExpiresExactlyNow`. Suite passes."

Bad: "Added a check in the reset handler so expired tokens are rejected there."
— the other three callers still accept them.
