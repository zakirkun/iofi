---
slug: secret-handling
version: 1
applies_to: [devops, lead-programmer, tech-leader]
tools_required: [repo_status, memory_write, escalate_founder]
priority: 10
description: Keep credentials out of the repository, the board, and the shared memory
---

## Purpose

Make sure nothing you produce contains a credential, and that anything that
needs one can still find it.

## When To Use

Any task touching authentication, deployment, a third-party API, an environment
file, or CI configuration. Also the moment you notice a secret anywhere it
should not be.

## Procedure

1. Before committing, `repo_status` and look at every file you are adding. A
   `.env`, a `*.pem`, a `credentials.json` or a config file with a filled-in
   token must not be among them.
2. Reference secrets by location, never by value. "The deploy key lives in the
   CI secret `DEPLOY_KEY`" is the correct form everywhere: in code comments, in
   task updates, in memories and in pull request bodies.
3. Read configuration from the environment. A default value in code is fine only
   when it is safe in public — a localhost URL is, a token is not.
4. Add the file pattern to `.gitignore` in the same commit that introduces the
   thing it protects, not afterwards.
5. If you find a secret already committed, stop the task. Do not just delete the
   line — the value is in history and must be treated as leaked. Report it with
   `escalate_founder` immediately, naming what kind of credential and where, but
   never the value itself.
6. `memory_write` of kind `convention` for where a class of secret belongs, so
   the next agent does not invent a second location.

## Constraints

- Never put a credential in a task, a memory, a meeting message, a commit
  message, a log line or a pull request body. Every one of those is stored and
  readable.
- Never print a secret to debug a problem. Print whether it is set and how long
  it is.
- Never commit a real value as a "temporary" placeholder or a test fixture.
- Rotating a leaked credential is the Founder's call to make and yours to
  recommend — say so explicitly rather than assuming someone noticed.

## Output Contract

A branch containing no credential, `.gitignore` entries for the files that hold
them, a `task_update` stating where each new secret is expected to live, and —
if one was found committed — an `escalate_founder` naming the credential type
and location without its value.

## Examples

Good: "The webhook needs a signing secret. It is read from
`IOFI_GITHUB_WEBHOOK_SECRET`; `.env.example` documents it with an empty value
and `.env` is gitignored. No value appears in the branch."

Good escalation: "A live Stripe restricted key is committed in
`deploy/config.yaml` at line 14, present since commit 3f2a1c. It must be revoked
and rotated — it is in history, so removing the line is not sufficient."

Bad: "Removed the API key from config.yaml." — still in history, still valid,
nobody told to rotate it.
