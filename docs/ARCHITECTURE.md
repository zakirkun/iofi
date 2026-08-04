# iofi — Architecture

Agent coordinator. User = CEO/Founder, orchestrates a company of AI agents.
Each agent is bound to a **role**, executes through a **coding tool** (Claude Code,
Codex, OpenCode, CommandCode), and shares knowledge through a common memory.

## 1. Decisions on record

| Topic | Decision |
|---|---|
| Sandbox | Git worktree per agent on host. No containers yet. |
| GitHub | Two-way sync: issues/PRs <-> kanban tasks. |
| MCP | Bidirectional (see §3). |
| Agent LLM auth | Native CLI subscription by default; `agents.llm_mode` switches to LiteLLM proxy. |
| API auth | Single bearer token on every `/api` route, generated at boot if unset. |
| Webhooks | A repository with no secret is refused, not trusted. |
| Migrations | Forward-only. Roll back by restoring the database. |
| Module path | `iofi` (bare, no VCS host). Rename is a single sed if published. |

### Why the API is authenticated

The API hires agents, dispatches runs that execute code as the iofi user, and
returns the whole shared memory. Treating it as a read-only dashboard and
leaving it open would mean anyone who can reach the port can run code on the
host. The token is the cheapest control that closes that, and the generated
default means there is no configuration in which the API is accidentally open.

### Why a missing webhook secret rejects rather than accepts

An unverified webhook can create tasks, move the board and close work. "No
secret configured" is far more often an oversight than a deliberate choice, so
it fails closed. A repository that genuinely wants unsigned deliveries sets its
secret to the explicit value `insecure-no-verification`.

## 2. Topology

```
            Browser  (React + Vite + Tailwind, served from embed.FS)
                |  REST + SSE
        +-------v--------------------------------------+
        |  Go / Chi  — control plane                   |
        |                                              |
        |  api/      HTTP handlers, SSE hubs           |
        |  brain/    LiteLLM client (coordinator LLM)  |
        |  mcpsrv/   MCP server exposed to agents      |
        |  runner/   worktrees + agent process spawn   |
        |  meeting/  turn-based discussion engine      |
        |  github/   webhook in, REST out              |
        |  skills/   markdown skill loader             |
        |  store/    pgx queries                       |
        +---+--------------------+---------------------+
            |                    |
    PostgreSQL + pgvector   LiteLLM proxy (docker, OpenAI-compatible)
```

The control plane holds all state. Coding agents are **stateless workers** —
they are spawned per task, given a prompt, given MCP access, and reaped.

## 3. MCP: which way does it point?

Both ways, because the tools are not symmetric.

**Inbound — iofi as MCP server.** Every coding tool can consume an MCP server.
This is the main channel. The runner writes a per-run MCP config pointing at
iofi's stdio bridge or HTTP endpoint, then launches the tool. The agent now has
tools to run the company:

| Tool | Purpose |
|---|---|
| `task_list`, `task_claim`, `task_update`, `kanban_move` | Kanban control |
| `task_create`, `task_split` | Decompose work |
| `memory_write`, `memory_search` | Shared knowledge |
| `meeting_post`, `meeting_read`, `meeting_open` | Discussion rooms |
| `handoff` | Pass a task to another role |
| `ask_role` | Synchronous question to another role's agent |
| `repo_status`, `pr_open` | GitHub actions |

Exposure is filtered per role by `roles.tool_policy`. A QA agent gets
`task_update` but not `pr_open`; a Tech Leader gets `task_split` and `handoff`.

**Outbound — iofi as MCP client.** Some tools expose a whole agent turn as a
single MCP tool (`codex mcp-server`). For those, iofi connects as a client over
stdio and calls the agent instead of reading a CLI's stdout. Same `Adapter`
interface, different transport, selected by `agent_tools.transport`.

```go
type Adapter interface {
    Kind() string
    Start(ctx context.Context, req RunRequest) (<-chan Event, error)
    Probe(ctx context.Context) error
}
```

`RunRequest` carries workspace path, composed prompt, MCP endpoint, env, and
model hint. `Event` is a normalized stream: `text`, `tool_call`, `token_usage`,
`error`, `done`. Every adapter's native output format (Claude Code
`stream-json`, Codex JSONL, OpenCode) collapses into that one stream so the web
UI and the `runs` table never learn tool-specific shapes.

`CommandCode` is unknown to us — it lands on `GenericAdapter`, configured by row
in `agent_tools`: command template, arg template, and a stdout parse mode
(`jsonl` / `plain`). Any future CLI attaches without new Go code.

## 4. Roles, skills, prompt composition

A **role** is the job (CTO, Tech Leader, Finance, Business Analyst, Business
Process, Lead Programmer, QA, DevOps, Sprint Manager, Support Ticket, …).
It owns a system prompt, a model preference, and a tool policy.

A **skill** is a reusable markdown procedure. One role has many skills; one
skill serves many roles. Stored as a file under `skills/` and mirrored into the
`skills` table so it is editable from the web.

```markdown
---
slug: api-design
version: 1
applies_to: [lead-programmer, tech-leader]
tools_required: [memory_search]
---
## Purpose
## When To Use
## Procedure
## Constraints
## Output Contract
## Examples
```

The loader rejects a skill missing any of those six sections — consistency is
enforced at load, not by convention.

Final agent prompt =
`role.system_prompt` + selected skill bodies + task spec + retrieved memories.
Skill selection is by `role_skills.priority`, narrowed by the brain when the
task text matches a skill's *When To Use*.

## 5. Shared memory

`memories` rows are scoped: `global`, `role:<id>`, `project:<id>`, `agent:<id>`.
Written through the `memory_write` MCP tool, read through `memory_search`
(pgvector cosine over embeddings + scope filter). Agents therefore learn from
each other's runs without the control plane copying context by hand.

Memory kinds: `fact`, `decision`, `convention`, `pitfall`, `artifact`.
Decisions from meetings are written here automatically.

## 6. Meeting room

Not free chat. A facilitator LLM drives a state machine:

```
open -> agenda -> round(opinion) -> conflict? -> resolve/vote -> decide
     -> emit tasks to kanban + write decision memories -> closed
```

Free-form agent-to-agent chat loops and burns tokens. Turn-based with an
explicit terminating state does not. Meeting kinds: `grooming`, `standup`,
`design`, `retro`, `incident`.

## 7. Sandbox

One bare clone per repo, one `git worktree` + branch per agent workspace.
Cheap, no image build, agents cannot collide on the index.

```
data/repos/<repo>.git          bare clone
data/work/<agent>/<task>/      worktree, branch iofi/<task-id>
```

A board must be bound to a repository for any of this to happen; connecting a
repository creates that board. Without the binding an agent gets a scratch
directory, which is right for analyst roles and useless for programmers.

Worktrees are released when the task closes, not when the run ends — the branch
still has to be reviewed. Scratch directories are released immediately.

`ponytail: host FS isolation only. Container-per-agent when running untrusted
repos or when a resource limit is actually needed.`

## 7a. Restarting

Nothing can resume a coding CLI whose parent died. At boot, runs left `queued`
or `running` are failed with that reason and every agent is reset to idle.
Skipping this leaves agents permanently "busy" and `PickAgentForRole` treating
the company as full.

## 8. GitHub two-way sync

- **In**: webhook (`issues`, `issue_comment`, `pull_request`) -> reconcile into
  `tasks` via `github_links`.
- **Out**: task transitions push labels/state; agent finishing a task opens a PR.
- Loop guard: every outbound write stamps `github_links.last_pushed_hash`;
  inbound events matching that hash are dropped. Without it, sync ping-pongs.

## 9. Realtime

SSE, not WebSocket — the traffic is server-to-browser only. One hub per topic:
`run:<id>` (live agent output), `board:<id>` (kanban moves), `meeting:<id>`.

## 10. Layout

```
cmd/iofi           entrypoint
internal/config    env config
internal/store     pgx, migrations, queries
internal/api       chi router, handlers, SSE
internal/mcpsrv    MCP server + tool implementations
internal/runner    worktrees, adapters, process supervision
internal/brain     LiteLLM client (role assign, triage, facilitate)
internal/meeting   discussion state machine
internal/github    webhook + REST client
internal/skills    markdown loader/validator
migrations         SQL, embedded and applied on boot
skills             markdown skill library
web                React/Vite/Tailwind, built into embed.FS
```
