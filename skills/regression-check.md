---
slug: regression-check
version: 1
applies_to: [qa]
tools_required: [repo_diff, task_update, knowledge_search]
priority: 20
description: Check that a change did not break what already worked, in the places most likely to break
---

## Purpose

Find what the change broke outside its own feature, before it reaches the done
column.

## When To Use

Before moving anything out of QA, and always after a change to shared code — a
helper, a middleware, a store query, a config default.

## Procedure

1. `repo_diff summary=true` to list what actually changed. That list, not the
   task description, decides where to look.
2. For each changed shared function, find its other callers. Those are the
   regression surface. A change to one caller's requirements silently changes
   every other caller's behaviour.
3. Run the whole existing test suite and read the output, not just the exit
   code. A test that was already failing before this branch is still a finding —
   report it as pre-existing rather than blaming the branch.
4. Exercise the three paths automated tests usually miss: the empty case, the
   permission-denied case, and the second identical request.
5. `knowledge_search` for any uploaded standard the change might now violate —
   a retention period, a rate limit, an accessibility rule.
6. Record what you checked, including what you decided not to check and why. An
   unstated gap in coverage is read as coverage.

## Constraints

- "The tests pass" is not a regression check. The tests only cover what someone
  already thought of.
- Do not re-verify the feature's own acceptance criteria here; that is a
  separate step and doubling it wastes the run.
- If a shared function changed and you cannot enumerate its callers, say so
  rather than declaring the surface clear.
- Never sign off on a change whose diff you did not read.

## Output Contract

A `task_update` listing: the shared code that changed, the callers checked, the
suite result quoted, the manual paths exercised, and any pre-existing failures
separated from new ones. If anything regressed, a defect task per finding.

## Examples

Good: "Changed `store.ListRuns` signature; four callers — dashboard, listRuns
handler, run_list tool, cost view. Checked all four compile and return the same
shape. Suite: 40 pass. Manually checked the empty-board case: dashboard renders
zero rows rather than erroring. One pre-existing failure in
`TestGitHubSync_Webhook`, present on main, filed separately."

Bad: "Ran the tests, all green, moving to done."
