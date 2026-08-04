---
slug: ci-pipeline-change
version: 1
applies_to: [devops]
tools_required: [repo_diff, run_list, memory_write]
priority: 15
description: Change the build without leaving the pipeline in a state only you can debug
---

## Purpose

Modify CI so it does what was asked, stays fast enough that people wait for it,
and fails in a way that names the problem.

## When To Use

A task asks for a new check, a changed build step, a caching change, or a fix to
a pipeline that is failing or flaking.

## Procedure

1. Read the existing workflow in full before editing. Most pipeline breakage is
   a step added twice or an environment variable set in two places with
   different values.
2. Establish which failure the change is meant to catch. A check nobody can name
   a failure for is a check that will be disabled the first time it is red.
3. Make the smallest change. Prefer a flag on an existing step over a new step,
   and a new step over a new job — a job pays container startup every run.
4. Make it fail loudly and specifically. A step that exits 1 with no message
   costs the next person the whole run to reproduce locally.
5. Check the cache key actually changes when its inputs change. A stale cache
   that never invalidates produces green builds of old code, which is worse than
   a red build.
6. Never leave a check non-blocking "for now" without an expiry. Record it with
   `memory_write` and an `expires_in_days`, or it stays advisory forever.
7. Verify by pushing and reading the run, not by reasoning about the YAML.

## Constraints

- No secret in the workflow file. Reference the CI secret store by name.
- No step that only works on your machine or in your shell. The runner has a
  different OS, a different PATH and no interactive terminal.
- Do not pin to a floating tag for anything that can break the build; pin the
  major version at minimum.
- If the change makes the pipeline slower, say by how much. A build people stop
  waiting for is a build people stop reading.
- Never disable a failing check to go green. Fix it or report why it cannot be.

## Output Contract

A workflow change, one observed run showing the new behaviour (pass and fail if
the failure is reachable), a `task_update` naming what failure it now catches
and the runtime delta, and a `memory_write` of kind `convention` for any rule the
pipeline now enforces on everybody.

## Examples

Good: "Added `gofmt -l` as a step in the existing lint job rather than a new
job; it adds 2s. Verified: pushed an unformatted file, the step failed naming
the file; formatted it, green. Recorded the convention that formatting is
enforced in CI so nobody adds a second formatter."

Bad: "Added a lint job." — which failure, how long, verified how.
