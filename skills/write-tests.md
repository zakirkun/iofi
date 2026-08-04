---
slug: write-tests
version: 1
applies_to: [lead-programmer, qa, devops]
tools_required: [repo_diff, task_update, memory_write]
priority: 20
description: Leave behind the smallest test that fails if the logic breaks
---

## Purpose

Prove the change works now and catch it when someone breaks it later, without
building a test suite nobody maintains.

## When To Use

Any change with a branch, a loop, a parser, a calculation, or anything touching
money, permissions or data loss. Not for a one-line constant change.

## Procedure

1. Write the test that fails before your fix and passes after. If it passes
   before, it is not testing your change.
2. Name it for the behaviour, not the function: `TestVerifyToken_ExpiresExactlyNow`
   says what broke; `TestVerifyToken2` says nothing.
3. Cover the boundary, not the middle. Zero, one, empty, exactly-at-the-limit,
   one-past-the-limit. The middle of a range is where code already works.
4. Cover the error path at least once. Untested error handling is where most
   production failures actually live.
5. Assert on the outcome, not on how it was reached. A test asserting which
   internal functions were called breaks on every refactor and catches nothing.
6. Run the whole existing suite, not just your new test. A failure you did not
   cause is still a failure you report.
7. `repo_diff` and check the test is actually in it.

## Constraints

- No mocks for things a real one is cheap to use. Mocking the database hides
  exactly the SQL, cast and array-type errors that break in production.
- No fixtures, factories or helper frameworks unless the tests already use them.
- A test that shares mutable state with another test is a test that fails on
  someone else's machine. Key assertions on data you created.
- Never make a test pass by loosening its assertion.
- If the logic genuinely cannot be tested without restructuring it, say so in
  `task_update` rather than shipping a test that asserts nothing.

## Output Contract

At least one test that fails without the change, a green run of the full
existing suite, and `task_update` naming the test and quoting the suite result.
A `memory_write` of kind `convention` if you established a testing pattern
others should follow.

## Examples

Good: "Added `TestSearchChunks_PendingDocIsInvisible`: it inserts a doc left in
`pending`, searches, and asserts zero hits. It fails on the previous code, which
returned passages from documents still being extracted. Full suite: 40 pass."

Bad: "Added TestUpload which calls Upload and checks err == nil." — passes
whether or not the upload did anything.
