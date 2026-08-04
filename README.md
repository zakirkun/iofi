# iofi

<a href="https://hololive.hololivepro.com/en/talents/airani-iofifteen/">
  <img
    src="https://hololive.hololivepro.com/wp-content/uploads/2020/07/Airani-Iofifteen_pr-img_01.webp"
    alt="Airani Iofifteen, hololive Indonesia"
    align="right"
    width="230"
  />
</a>

**Run a software company staffed by AI agents. You are the Founder.**

You describe what you want in plain words. iofi decides which role owns it,
writes the acceptance criteria, hands it to an agent bound to that role, gives
that agent a private git worktree and a set of tools to run the company with —
and records what it learned so the next agent starts from it.

Named after **Airani Iofifteen** — "iofi" — of hololive Indonesia: an artist who
coordinates a room full of very different personalities and somehow ships.
Fitting for a program whose whole job is running a company of agents that do not
think alike.

<sub>Airani Iofifteen and her artwork are © COVER Corp. Image linked from the
[official talent page](https://hololive.hololivepro.com/en/talents/airani-iofifteen/).
This project is an unaffiliated fan-named tool, not endorsed by COVER Corp.
· [YouTube](https://www.youtube.com/channel/UCAoy6rzhSf4ydcYjJw3WoVg)
· [X](https://twitter.com/airaniiofifteen)</sub>

<br clear="right"/>

---

## What this actually is

Coding agents are good at *doing a task*. None of them run a company. They do
not hold a backlog, do not know which role owns a problem, and forget everything
the moment the process exits. Put two of them on one repository and they
duplicate work, contradict last week's decision, and fight over the git index.

iofi is the layer above them.

| | |
|---|---|
| **Roles** | CTO, Tech Leader, Lead Programmer, QA, DevOps, Business Analyst, Business Process, Finance, Sprint Manager, Support Ticket. Each has a system prompt, a model preference, and a policy for which tools it may use. |
| **Agents** | A named worker: one role + one coding CLI + a concurrency limit. Stateless between runs. |
| **Coding tools** | Claude Code, Codex, OpenCode, CommandCode — attached as **database rows**, not Go code. |
| **Shared memory** | Decisions, conventions and pitfalls, searchable by meaning, injected into every brief. |
| **Kanban** | Agents run the board themselves: claim, update, move, split, hand off. |
| **Meetings** | A facilitated, round-capped discussion between roles that ends in a decision and produces tasks. |
| **GitHub** | Two-way sync of issues and pull requests, with a loop guard. |

One Go binary with the web console embedded inside it. One PostgreSQL. One
LiteLLM proxy for the coordinator's reasoning.

### A concrete example

You type: *"Users should be able to reset their password."*

1. The coordinator routes it to **Business Analyst**, writes acceptance
   criteria, and decides the work spans three roles — so it splits into three
   tasks with dependencies between them.
2. The **Lead Programmer** task waits, because its prerequisite is open.
3. When the analyst finishes, the programmer agent is dispatched into a fresh
   worktree on `iofi/<task-id>`. Before writing code it reads the memory the
   company already holds about your auth layer.
4. It commits, calls `task_update` with what it verified, opens a pull request,
   and writes a `pitfall` memory about the thing that cost it an hour.
5. **QA** picks the task up from the review column and checks each criterion
   against the branch.
6. You watched all of it stream live, and the whole run cost is on the Command
   view.

---

## Contents

- [How it works](#how-it-works)
- [Requirements](#requirements)
- [Quick start](#quick-start)
- [Configuration](#configuration)
- [Your first hour](#your-first-hour)
- [Concepts](#concepts)
- [What agents can do](#what-agents-can-do)
- [Adding a coding tool](#adding-a-coding-tool)
- [Writing skills](#writing-skills)
- [GitHub integration](#github-integration)
- [Security](#security)
- [Operations](#operations)
- [Development](#development)
- [Troubleshooting](#troubleshooting)
- [Known limits](#known-limits)
- [Documentation](#documentation)

---

## How it works

```
                  Browser — React + Vite + Tailwind
                  served from the Go binary's embed.FS
                            │  REST + SSE
                            ▼
        ┌───────────────────────────────────────────────┐
        │  Go / Chi — control plane                     │
        │                                               │
        │  api/      HTTP handlers, SSE hubs, auth      │
        │  brain/    LiteLLM: triage, routing, meetings │
        │  mcpsrv/   MCP server exposed to agents       │
        │  runner/   worktrees + agent process spawn    │
        │  meeting/  discussion state machine           │
        │  github/   webhook in, REST out               │
        │  skills/   markdown loader and validator      │
        │  store/    pgx queries, embedded migrations   │
        └───┬───────────────────────┬───────────────────┘
            │                       │
   PostgreSQL 17 + pgvector    LiteLLM proxy (OpenAI-compatible)
            ▲
            │  MCP over HTTP, one bearer token per run
            │
   ┌────────┴─────────┬──────────────┬──────────────┐
   │                  │              │              │
Claude Code        Codex         OpenCode      CommandCode
   └──────── each in its own git worktree ───────────┘
```

**Coding agents are stateless workers.** They are spawned per task, handed a
composed brief and an MCP endpoint, and reaped when done. All state lives in
PostgreSQL, so restarting iofi loses nothing but the in-flight processes — and
those are reclaimed at boot.

**MCP points both ways.** iofi is an MCP *server* that agents call to run the
company (that is the main channel), and an MCP *client* for tools that expose a
whole agent turn as a tool of their own, like `codex mcp-server`.

**LiteLLM is the coordinator's brain, not the agents'.** Triage, skill
selection, meeting facilitation and embeddings go through it. The coding CLIs
keep their own subscription login unless you flip an agent to `proxy` mode.

Full topology and the reasoning behind each decision:
[docs/ARCHITECTURE.md](docs/ARCHITECTURE.md).

---

## Requirements

| | Version | Why |
|---|---|---|
| Go | 1.25+ | `go.mod` pins the toolchain; older Go auto-upgrades |
| Node | 22+ | Building the console |
| Docker | any recent | PostgreSQL with pgvector, and LiteLLM |
| Git | any recent | Agent workspaces are worktrees |
| An LLM API key | — | `OPENAI_API_KEY` is enough to start |

Plus at least one coding CLI on `PATH`, whichever you plan to use:

```bash
npm i -g @anthropic-ai/claude-code   # claude
npm i -g @openai/codex               # codex
npm i -g opencode-ai                 # opencode
npm i -g command-code                # command-code
```

Each keeps its own authentication. iofi does not manage their credentials.

---

## Quick start

```bash
git clone <your-fork> iofi && cd iofi

cp .env.example .env
# Edit .env: set OPENAI_API_KEY at minimum.

docker compose up -d          # PostgreSQL :5432, LiteLLM :4000
make web                      # npm ci && npm run build → web/dist
go run ./cmd/iofi             # migrations and seeds run on boot
```

You should see:

```
level=INFO msg="database ready"
level=INFO msg="skills loaded" count=5
level=WARN msg="IOFI_AUTH_TOKEN was not set; generated one for this run" token=xxxxxxxx
level=INFO msg="iofi listening" addr=:8080 mcp=http://localhost:8080/mcp
```

Open **http://localhost:8080** and paste that token.

Set `IOFI_AUTH_TOKEN` in `.env` to keep the same token across restarts.

The first boot seeds ten roles with their system prompts, four coding tools, a
default board, five skills, and a token price table. Nothing else is required
before you start.

### Build a binary

```bash
make build        # builds the console, then embeds it — bin/iofi
./bin/iofi
```

One file. The console ships inside it.

---

## Configuration

Environment variables only; no config file. Full list in
[`.env.example`](.env.example).

| Variable | Default | Notes |
|---|---|---|
| `IOFI_ADDR` | `:8080` | Listen address. Keep it on loopback — see [Security](#security) |
| `IOFI_DATABASE_URL` | `postgres://iofi:iofi@localhost:5432/iofi?sslmode=disable` | Needs the `vector` and `pgcrypto` extensions |
| `IOFI_DATA_DIR` | `data` | Bare clones and worktrees live here |
| `IOFI_AUTH_TOKEN` | *generated* | Guards every `/api` route. Generated and logged when unset, so the API is never accidentally open |
| `IOFI_RATE_LIMIT_PER_MIN` | `600` | Per client address |
| `IOFI_PUBLIC_URL` | `http://localhost:8080` | The MCP URL agents are told to reach. Change it if iofi runs behind a proxy or in a container |
| `IOFI_LITELLM_URL` | `http://localhost:4000` | Coordinator brain |
| `IOFI_LITELLM_KEY` | — | Bearer for the proxy |
| `IOFI_BRAIN_MODEL` | `gpt-4o-mini` | Triage, skill selection, facilitation |
| `IOFI_EMBED_MODEL` | `text-embedding-3-small` | Blank disables embeddings; memory falls back to keyword search |
| `IOFI_EMBED_DIM` | `1536` | Must match the `vector(N)` column. Checked at boot, with the `ALTER` to run if it does not |
| `IOFI_GITHUB_TOKEN` | — | Blank disables sync and `pr_open`; everything else still runs |
| `IOFI_GITHUB_WEBHOOK_SECRET` | — | Fallback when a repository has no secret of its own |
| `IOFI_MAX_RUNS` | `4` | Concurrent agent processes across the whole company |
| `IOFI_EVENT_RETENTION_DAYS` | `30` | Run events of finished runs are pruned past this |

### LiteLLM

`deploy/litellm.yaml` defines the models iofi can name. Add yours there and
reference it in a role's **model preference** or an agent's **model** field.
Only the providers you actually use need a key.

---

## Your first hour

### 1 — Connect a repository

**Command → Repositories.** Enter `owner/name`, the clone URL, the default
branch, and a webhook secret.

Connecting a repository also **creates a board bound to it**. That binding is
what gives agents a real git worktree; without it they get a scratch directory
and cannot open pull requests. Bind an existing board instead under
**Kanban → board settings**.

### 2 — Hire agents

**Org → + hire.** A name, a role, and a coding tool.

Start with two: a **Lead Programmer** on Claude Code and a **QA** on whatever
else you have installed. Hire more roles as you find yourself wanting them.

*LLM auth:* `native` uses the CLI's own login — cheapest if you already pay for a
subscription, but iofi cannot see the token cost. `proxy` routes the CLI through
LiteLLM, which costs API rates but puts every token in one ledger.

### 3 — Brief the company

**Kanban → + brief the company.** Write what you want and any constraints the
agents cannot infer. The coordinator returns the owning role, a priority, an
estimate, acceptance criteria, and subtasks when the work spans more than one
role. The reason it chose that role is stored on the task, so a bad routing can
be corrected rather than merely noticed.

Drag cards between columns. A column at its WIP limit refuses the drop and tells
you why.

### 4 — Watch it work

**Runs.** Live output, tool calls, token usage, and a cancel button.

### 5 — Hold a meeting

**Meetings → + call a meeting.** Pick an agenda phrased as the questions that
must be settled, choose participants, and run it. The facilitator picks who
speaks, cuts off agreement, and forces a decision by the round limit. The
decision becomes board tasks and memories automatically.

### 6 — Check what it knows

**Memory.** Everything the company has learned, searchable. You can write
entries yourself — a convention you want every agent to follow belongs here, not
in a prompt you retype.

---

## Concepts

### Roles

A role is a job, not a person. It owns:

- a **system prompt** — the persona every agent in that role runs with;
- a **model preference** — which model its reasoning uses;
- a **tool policy** — which MCP tools its agents may call.

The policy is applied when the MCP server is built for a run, so a forbidden
tool is never advertised to the model. A QA agent does not see `pr_open` at all,
which is stronger than refusing the call.

Roles are fully editable under **Org → Roles**, including the prompt.

### Skills

A reusable markdown procedure, many-to-many with roles. See
[Writing skills](#writing-skills).

### Agents and tools

An **agent** is a name + role + coding tool + capacity. A **tool** is a row in
`agent_tools` describing a CLI. See
[Adding a coding tool](#adding-a-coding-tool).

### Board and tasks

Tasks carry a spec (what to do), acceptance criteria (how anyone knows it is
done), an owning role, dependencies, labels and a column.

Two guards are enforced, not decorative:

- **Dependencies** — dispatch is refused while a prerequisite is open, and names
  the blockers.
- **WIP limits** — a move into a full column is refused with HTTP 409.

Every change is an attributed event: `founder`, `agent:<id>`, `github` or
`brain`.

### Memory

Scoped `global`, `role` or `project`. Kinds: `fact`, `decision`, `convention`,
`pitfall`, `artifact`. Each carries a confidence and an optional expiry.

Search ranks by semantic similarity **weighted by confidence**, so a near-match
nobody trusts does not outrank a slightly worse one recorded as settled. With no
embedding model configured it degrades to keyword search rather than returning
nothing.

The eight most relevant memories are injected into every agent brief before the
agent asks for anything.

### Meetings

`open → agenda → round → resolve → decided → closed`, capped at a round limit.

Participants answer in character through the coordinator model — a meeting is
talking, not building, and spawning four coding CLIs to hold a conversation
would cost a hundred times more for the same words. Free-form agent chat is
deliberately not offered: it does not terminate.

### Runs and workspaces

A **run** is one execution of one agent against one task, with its normalised
event stream and token accounting. A **workspace** is a git worktree on
`iofi/<task-id>`, released when the task closes — not when the run ends, because
the branch still has to be reviewed.

---

## What agents can do

Agents reach iofi over MCP with a bearer token scoped to their run. The token
carries the agent, role, task, board and workspace, so `task_update` with no
arguments means "the task I was dispatched for".

| Tool | What it does |
|---|---|
| `task_list` | Board tasks with assignee, owning role, idle hours and dependency-blocked status. Filter to stalled work with `stalled_hours` |
| `task_get` | Full spec, acceptance criteria and activity of one task |
| `task_claim` | Take an unassigned task. Atomic — two agents racing, exactly one wins |
| `task_create` | Put new work on the board for a role |
| `task_split` | Break a task into child tasks, each with an owning role |
| `task_update` | Report progress and the final summary |
| `kanban_move` | Move a task between columns |
| `memory_write` | Record a decision, convention or pitfall, with confidence and optional expiry |
| `memory_search` | Read what the company already knows |
| `meeting_open` | Open a discussion; the facilitator starts immediately |
| `meeting_post` / `meeting_read` | Speak in and read a room |
| `handoff` | Pass a task to another role with a note |
| `ask_role` | Ask another role a question and get an answer now, in character |
| `cost_report` | Token and cost totals per agent, for Finance |
| `repo_status` | `git status` of the agent's own workspace |
| `pr_open` | Push the branch and open a pull request linked to the issue |

`ask_role` answers through the coordinator rather than spawning the other
agent's CLI. Two agents blocking on one capacity slot would deadlock.

---

## Adding a coding tool

**A new CLI is a database row, not new Go code.** Edit it under
**Org → Coding tools**.

| Field | Meaning |
|---|---|
| `command` | The executable |
| `args` | JSON array, templated |
| `env` | JSON object, templated |
| `parse_mode` | `claude_stream_json`, `jsonl`, or `plain` |
| `transport` | `spawn` or `mcp_client` |
| `mcp_style` | `flag`, `config_file`, or `env` |
| `mcp_target` | The flag name, the config file path, or the remote tool name |

Template variables available in `args`, `env` and `mcp_target`:
`{{.Prompt}}` `{{.Workspace}}` `{{.MCPConfig}}` `{{.Model}}` `{{.RunID}}`

Two transports:

- **`spawn`** — run the CLI headless and read its stdout. Claude Code, Codex,
  OpenCode, CommandCode.
- **`mcp_client`** — the tool is itself an MCP server exposing a whole agent turn
  as a tool (`codex mcp-server`); iofi connects over stdio and calls it.
  `mcp_target` names the remote tool, or is left blank to take the first
  advertised.

Whatever the tool emits is normalised into one event stream — `text`,
`tool_call`, `tool_result`, `usage`, `error`, `done` — so the database and the
console never learn a tool-specific format.

### The shipped four

All are configured and enabled. Their MCP config shapes differ, and getting one
wrong is the most expensive silent failure in the system: a mis-keyed entry means
the server is ignored and the agent runs with **no iofi tools at all**, which
looks like an agent that simply chose not to use them.

| Tool | Headless | MCP config location | Entry shape |
|---|---|---|---|
| Claude Code | `claude -p --output-format stream-json` | `--mcp-config <file>` | `mcpServers.iofi.type = "http"` |
| Codex | `codex exec --json` | `$CODEX_HOME/config.toml` | `[mcp_servers.iofi]` with `url`, `bearer_token` |
| OpenCode | `opencode run` | `opencode.json` | `mcp.iofi.type = "remote"`, `enabled` |
| CommandCode | `command-code -p --output-format json` | `.mcp.json` in the project root | `mcpServers.iofi.transport = "http"`, `enabled` |

For CommandCode the binary is `command-code`. The short alias is `cmd` on
macOS/Linux and `cmdc` on Windows, so only the full name is safe to hardcode.

---

## Writing skills

A skill is markdown with YAML frontmatter and **six mandatory sections**:

```markdown
---
slug: api-design
version: 1
applies_to: [lead-programmer, tech-leader]
tools_required: [memory_search]
priority: 20
description: one line, used when the coordinator narrows the skill set
---

## Purpose
What this procedure achieves.

## When To Use
The conditions that make it apply. The coordinator matches against this.

## Procedure
Numbered steps.

## Constraints
What must never happen.

## Output Contract
Exactly what the agent produces, in which tool calls.

## Examples
A good one and a bad one.
```

A file missing or leaving empty any of those six is **rejected at load** —
consistency is enforced by the loader, not by convention, so an agent never
receives a half-written procedure. A skill declaring `tools_required` that its
target role's policy forbids produces a warning rather than a failure halfway
through a run.

Files in `skills/` load at boot and wire themselves to the roles in
`applies_to`. They are also editable under **Org → Skills** with identical
validation. `POST /api/skills/reload` re-reads the directory without a restart.

Five skills ship as working examples: `task-breakdown`, `implement-task`,
`verify-acceptance`, `architecture-decision`, `triage-ticket`.

---

## GitHub integration

Set `IOFI_GITHUB_TOKEN` to a token with `repo` scope. Blank disables sync and
`pr_open`; everything else still runs.

**Webhook setup.** In the repository: *Settings → Webhooks → Add webhook*.

| Field | Value |
|---|---|
| Payload URL | `https://<your-host>/webhooks/github` |
| Content type | `application/json` |
| Secret | The same secret you entered for the repository in iofi |
| Events | Issues, Issue comments, Pull requests |

**Inbound.** Issues become tasks on the bound board. The `iofi:<column>` label
sets the column. Comments become task events. A merged pull request closes its
task; a closed one moves it to blocked.

**Outbound.** Board changes are mirrored to the issue automatically, coalesced
over two seconds so an update and a move in the same breath are one API call.
`pr_open` pushes the branch and opens a pull request that closes the issue.

**The loop guard.** Every outbound write stores a hash of the state it pushed;
an inbound event hashing to the same value is dropped as our own echo. Without
it the two sides answer each other forever.

**Webhooks fail closed.** A repository with no secret has its deliveries
rejected. "No secret" is far more often an oversight than a decision, and an
unverified webhook can create tasks, move the board and close work. A repository
that genuinely wants unsigned deliveries sets its secret to the explicit value
`insecure-no-verification`.

---

## Security

Read this before exposing iofi to anything but localhost.

- **The API is authenticated.** Every `/api` route except `/health` requires a
  bearer token, compared in constant time. It is not a read-only dashboard: it
  hires agents, dispatches runs that execute code, and returns the entire shared
  memory. When `IOFI_AUTH_TOKEN` is unset one is generated at boot and logged, so
  there is no configuration in which the API is accidentally open.
- **Requests are rate limited** per client address.
- **MCP authenticates per run.** Tokens expire after three hours and are revoked
  when the run ends; the cached MCP server behind them is dropped with them.
- **Webhooks fail closed**, as above.
- **Agents are not sandboxed.** They run as the iofi user, in a git worktree, in
  the coding CLI's permissive mode. They can read the filesystem your user can
  read. iofi warns at boot when bound to a non-loopback address.

**Run iofi on a machine you would hand to a contractor, and keep it on
loopback.** Use it with repositories you trust. Container-per-agent isolation
does not exist yet.

---

## Operations

**Restarting is safe.** Nothing can resume a coding CLI whose parent died, so at
boot every run left `queued` or `running` is failed with that reason and every
agent is reset to idle. Skipping this would leave agents permanently "busy" and
the company looking full while accepting no work.

**Janitors** run in the background: expired MCP sessions every five minutes;
run events of finished runs past `IOFI_EVENT_RETENTION_DAYS` and expired
memories hourly.

**Cost.** Claude Code reports its own spend, so those figures are exact.
Everything else is priced from the rate table under **Command → Spend**
(`GET`/`PUT /api/settings/prices`), and is labelled as an estimate. Check the
table against your provider's current prices — iofi seeds plausible defaults, not
authoritative ones.

**Workspaces.** Repository worktrees are released when their task closes; scratch
directories when the run ends. Release one by hand from **Command → Live
workspaces** if a task is abandoned.

**Health.** `GET /api/health` is unauthenticated and reports database and
LiteLLM liveness. The console shows a warning in the header when the coordinator
is unreachable — triage and meetings fail without it, but runs still dispatch
with degraded skill selection and keyword-only memory recall.

---

## Development

```
cmd/iofi           entrypoint, wiring, boot recovery
internal/api       chi router, handlers, SSE, auth and rate limiting
internal/mcpsrv    MCP server and the agent-facing tools
internal/runner    worktrees, tool adapters, prompt composition, supervision
internal/brain     LiteLLM client: triage, skill selection, facilitation
internal/meeting   discussion state machine
internal/github    webhook in, REST out, loop guard
internal/skills    markdown loader and validator
internal/store     pgx queries, models, embedded migrations
internal/session   run-scoped MCP tokens
internal/bus       in-process pub/sub feeding SSE
internal/config    environment configuration
skills/            skill library
web/               React console, embedded via embed.FS
docs/              PRD, ERD, FLOWS, ARCHITECTURE
```

### Dev loop

```bash
go run ./cmd/iofi    # terminal 1 — API on :8080
make dev             # terminal 2 — Vite on :5173, proxying /api
```

### Make targets

| Target | Does |
|---|---|
| `make dev` | Vite dev server with hot reload |
| `make web` | `npm ci && npm run build` into `web/dist` |
| `make build` | Console, then `bin/iofi` with it embedded |
| `make run` | `go run ./cmd/iofi` |
| `make up` / `make down` | docker compose |
| `make test` | `go test ./...` |
| `make tidy` | `go mod tidy` and `gofmt -w` |

### Tests

```bash
go test ./...                                      # unit tests

IOFI_TEST_DATABASE_URL=postgres://iofi:iofi@localhost:5432/iofi?sslmode=disable \
  go test ./internal/store/...                     # + every SQL statement
```

Unit tests cover the skill parser, tool-adapter templating, MCP config shapes
per tool, output stream normalisation, and the GitHub webhook signature and loop
guard.

The store tests run every query against a real PostgreSQL with pgvector, and
skip when `IOFI_TEST_DATABASE_URL` is unset. There is nothing worth mocking
there: what breaks in that layer is the SQL, the jsonb casts and the array
types, and a mock reproduces none of them. CI provides the database on every
push, alongside `go vet`, a `gofmt` check, `-race`, and the console build.

### Migrations

Plain SQL in `internal/store/migrations/`, embedded in the binary, applied in
filename order at boot, each in its own transaction, recorded in
`schema_migrations`. Add a file with the next number; it runs on the next start.

Forward-only by decision — every migration would otherwise need a hand-written
inverse that is almost never run. Roll back by restoring the database.

### API surface

`/mcp` for agents · `/webhooks/github` for GitHub · everything else under
`/api`: `health` `dashboard` `events` `roles` `skills` `tools` `agents` `boards`
`tasks` `memories` `meetings` `runs` `repos` `workspaces` `costs`
`settings/prices`.

`GET /api/events?topic=…&token=…` is the SSE stream. Topics: `run:<id>`,
`board:<id>`, `boards`, `runs`, `meetings`, `meeting:<id>`, `memory`, and `*`.

---

## Troubleshooting

| Symptom | Cause and fix |
|---|---|
| `memories.embedding is vector(1536) but IOFI_EMBED_DIM is …` | Your embedding model has different dimensions. The error prints the `ALTER` to run, or set `IOFI_EMBED_DIM` back. |
| Every panel says "unauthorized" | The token changed. `IOFI_AUTH_TOKEN` was unset, so a new one was generated at boot — check the log, or set it in `.env`. |
| Agent ran but ignored every iofi tool | Its MCP config shape is wrong for that CLI. Compare the row against [the table above](#the-shipped-four). |
| `no board is bound to this repository` | Bind one under **Kanban → board settings**. Connecting a repository normally creates it. |
| `pr_open`: "no repository workspace" | The board has no repository, so the agent got a scratch directory. Same fix. |
| `pr_open`: "branch has no commits" | The agent did not commit. Its brief tells it to; check the run output for what stopped it. |
| Dispatch refused, names other tasks | Dependencies are open. Finish them or clear `depends_on` in the task modal. |
| Drag refused with 409 | The destination column is at its WIP limit. |
| Webhook returns 401 | The repository has no secret, or the signature does not match. |
| "coordinator offline" in the header | LiteLLM is unreachable. Triage and meetings fail; runs still work with degraded recall. |
| Agent stuck "busy" | Should be impossible after a restart — boot recovery resets it. If it persists during a run, cancel from **Runs**. |

---

## Known limits

- **No sandbox.** Agents run as the iofi user on the host.
- **Forward-only migrations.** Roll back by restoring the database.
- **Cost is exact only where the tool reports it.** Everything else is an
  estimate from a rate table you must maintain.
- **Single tenant.** No accounts, no roles for humans, one Founder per
  deployment.
- **`mcp_client` does not stream.** It calls the remote agent once and returns
  the result; spawned CLIs stream intermediate output, this transport does not.

What was deliberately left out, and why, is in [docs/PRD.md](docs/PRD.md) §12.

---

## Documentation

| Document | Covers |
|---|---|
| [PRD](docs/PRD.md) | Problem, users, goals, 44 numbered functional requirements, security model, what was deliberately left out |
| [ERD](docs/ERD.md) | Data model, every table and why it is shaped that way, deletion behaviour, indexes |
| [FLOWS](docs/FLOWS.md) | End-to-end behaviour in 14 flows: intake, run lifecycle, MCP auth, meetings, GitHub sync, failure handling |
| [ARCHITECTURE](docs/ARCHITECTURE.md) | Topology and the decisions on record |
