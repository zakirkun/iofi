# iofi — Flows

How the system actually behaves, end to end. Each flow names the code that
implements it. Companion documents: [PRD](PRD.md), [ERD](ERD.md),
[ARCHITECTURE](ARCHITECTURE.md).

---

## 1. Boot

```mermaid
flowchart TD
    A[start] --> B[load config]
    B --> C{IOFI_AUTH_TOKEN set?}
    C -->|no| C1[generate one, print it to the log]
    C -->|yes| D
    C1 --> D[connect to PostgreSQL]
    D --> E[apply pending migrations, one transaction each]
    E --> F{embedding dimension matches the column?}
    F -->|no| F1[fail with the ALTER statement to run] --> Z([exit])
    F -->|yes| G[seed the token price table if absent]
    G --> H[fail runs left queued or running<br/>reset every agent to idle]
    H --> I[load skills/*.md, warn on rejects]
    I --> J[start janitors: sessions, run events, expired memories]
    J --> K{GitHub token present?}
    K -->|yes| K1[start the outbound sync watcher]
    K -->|no| L
    K1 --> L[serve HTTP: /api, /mcp, /webhooks/github, console]
```

Step **H** matters more than it looks. Nothing can resume a coding CLI whose
parent process died. Skipping it leaves agents permanently `busy`, and
`PickAgentForRole` then treats the whole company as full — the system looks
alive and accepts no work.

`cmd/iofi/main.go`, `internal/store/maintenance.go`

---

## 2. Founder intake → dispatch

The core loop. The Founder writes what they want; iofi decides the rest.

```mermaid
sequenceDiagram
    actor F as Founder
    participant API as API
    participant B as Brain (LiteLLM)
    participant DB as PostgreSQL
    participant R as Runner

    F->>API: POST /api/tasks/intake {title, spec, dispatch}
    API->>DB: list roles
    API->>B: triage(title, spec, role catalogue)
    B-->>API: {role_slug, reason, priority, estimate,<br/>acceptance, labels, subtasks[]}
    Note over API: reject a role_slug that is not in the catalogue<br/>rather than creating an unassignable task
    API->>DB: create parent task (column=ready)
    API->>DB: record routing rationale as a task event
    loop each subtask
        API->>DB: create child task (column=backlog)
    end
    API->>API: publish task_created on the board and boards topics

    alt single role and dispatch requested
        API->>R: Dispatch(task)
    else split across roles
        Note over API: children are left for their roles to claim
    end
    API-->>F: {task, subtasks, triage}
```

The rationale is stored, not just the choice. A routing decision nobody can
review is a routing decision nobody can correct.

`internal/api/handlers.go:intake`, `internal/brain/coordinator.go`

---

## 3. Run lifecycle

```mermaid
sequenceDiagram
    participant R as Runner
    participant DB as PostgreSQL
    participant G as Git
    participant S as Session registry
    participant CLI as Coding CLI
    participant M as MCP server
    participant UI as Browser (SSE)

    R->>DB: unmet dependencies?
    alt any prerequisite open
        R-->>R: refuse, naming the blockers
    end
    R->>DB: resolve agent (assignee, else least-loaded of the role under capacity)
    R->>G: ensure bare clone, add worktree on branch iofi/TASK-ID
    R->>DB: create workspace
    R->>R: compose prompt<br/>role prompt + skills + memories + task + reporting contract
    R->>DB: create run (queued)

    R->>S: create run-scoped token
    R->>DB: agent status = busy
    R->>CLI: spawn with templated args, MCP config file, env
    R->>DB: run = running

    loop normalised events
        CLI-->>R: text / tool_call / usage / error
        R->>DB: append run_event
        R->>UI: publish on topic run:RUN-ID
    end

    par agent runs the company
        CLI->>M: task_update, memory_write, kanban_move, ask_role …
        M->>S: resolve bearer token → agent, role, task, workspace
        M->>DB: apply, attributed to this agent
        M->>UI: publish on topic board:BOARD-ID
    end

    CLI-->>R: done(exit code)
    R->>DB: price tokens from the rate table if the tool reported none
    R->>DB: run = done | failed
    R->>DB: release the workspace if it was a scratch directory
    R->>S: revoke token → MCP server cache dropped
    R->>DB: agent status = idle
```

Two guards worth naming:

- **Dependencies** are checked before anything is created. An agent cannot know
  from its brief that a prerequisite is unfinished, so the check belongs here.
- **Token revocation** also drops the cached MCP server behind it. Without that,
  every run leaks a server for the process's lifetime.

`internal/runner/runner.go`, `internal/runner/adapter.go`, `internal/mcpsrv/`

---

## 4. Agent adapters

One interface, two transports, and a third case that needs no Go at all.

```mermaid
flowchart TD
    A[Dispatch] --> B{agent_tools.transport}
    B -->|spawn| C[SpawnAdapter]
    B -->|mcp_client| D[MCPClientAdapter]

    C --> C1[render args and env from the row's templates]
    C1 --> C2[write the MCP config in this tool's shape]
    C2 --> C3[exec the CLI in the workspace]
    C3 --> C4{parse_mode}
    C4 -->|claude_stream_json| E[normalised Event stream]
    C4 -->|jsonl| E
    C4 -->|plain| E

    D --> D1[start the tool as an MCP server over stdio]
    D1 --> D2[resolve the remote tool name<br/>mcp_target, or the first advertised]
    D2 --> D3[CallTool with prompt and cwd]
    D3 --> E
```

Every tool collapses into the same `Event` stream — `text`, `tool_call`,
`tool_result`, `usage`, `error`, `done` — so the database and the console never
learn a tool-specific format.

The MCP config shape differs per tool and is the easiest thing to get silently
wrong: a mis-keyed entry means the server is ignored and the agent runs with no
iofi tools at all, which looks like the agent simply choosing not to use them.

| Tool | Config location | Shape |
|---|---|---|
| Claude Code | `--mcp-config <file>` | `mcpServers.iofi.type = "http"` |
| Codex | `$CODEX_HOME/config.toml` | `[mcp_servers.iofi]` with `url`, `bearer_token` |
| OpenCode | `opencode.json` | `mcp.iofi.type = "remote"`, `enabled` |
| CommandCode | `.mcp.json` in the project root | `mcpServers.iofi.transport = "http"`, `enabled` |

Adding a fifth tool is a row: command, args, parse mode, config style, config
target. No Go changes.

`internal/runner/adapter.go`, `internal/runner/mcpclient.go`

---

## 5. Prompt composition

What an agent actually receives.

```mermaid
flowchart LR
    A[roles.system_prompt] --> P[Brief]
    B[identity: name and role] --> P
    C[selected skills] --> P
    D[recalled memories] --> P
    E[task: spec + acceptance] --> P
    F[reporting contract:<br/>which MCP tools to call and when] --> P
    P --> G{longer than 16k chars?}
    G -->|yes| H[write .iofi/brief.md,<br/>pass a pointer on the command line]
    G -->|no| I[pass inline]
```

**Skill selection.** All of the role's skills if they fit the budget; otherwise
the coordinator picks the ones whose *When To Use* matches the task, falling back
to `role_skills.priority` order if that call fails. A failed narrowing must not
fail the run.

**Memory recall.** The task title and spec are embedded and searched across
`global`, `role:<id>` and `project:<board>`, ranked by similarity weighted by
confidence. The agent does not have to ask — the eight most relevant items are
already in the brief, and it can call `memory_search` for more.

`internal/runner/prompt.go`

---

## 6. MCP: authentication and tool filtering

```mermaid
sequenceDiagram
    participant CLI as Coding CLI
    participant H as /mcp handler
    participant S as Session registry
    participant Srv as Per-token MCP server
    participant DB as PostgreSQL

    CLI->>H: request with Authorization: Bearer RUN-TOKEN
    H->>S: look up the token (constant-time, expiry checked)
    alt unknown or expired
        H-->>CLI: 401
    end
    S-->>H: session {agent, role, task, board, workspace, policy}
    H->>Srv: server for this token, built once and cached
    Note over Srv: tools are registered only if roles.tool_policy permits them
    CLI->>Srv: tools/list
    Srv-->>CLI: only the permitted tools
    CLI->>Srv: tools/call task_update {...}
    Srv->>DB: apply, attributed to this agent
    Srv-->>CLI: text result
```

Filtering happens at registration, not at call time. A tool the role cannot use
is never advertised — the model cannot be tempted by a capability it never sees,
and a refusal never wastes a turn.

Tokens carry the task and workspace, so `task_update` with no arguments means
"the task I was dispatched for". The agent cannot act on another agent's task by
omission.

`internal/mcpsrv/server.go`, `internal/session/session.go`

---

## 7. Kanban guards

```mermaid
flowchart TD
    A[move a task to a column] --> B{column exists on this board?}
    B -->|no| B1[refuse, list the valid keys]
    B -->|yes| C{column has a WIP limit?}
    C -->|no| E[apply]
    C -->|yes| D{open tasks in it, excluding this one, under the limit?}
    D -->|no| D1[refuse: ErrWIPExceeded<br/>API returns 409, the card snaps back]
    D -->|yes| E
    E --> F[record a task event]
    F --> G[publish to board:id and boards]
    G --> H[GitHub watcher picks it up]
```

The count excludes the task being moved, so a move within the same column never
trips its own limit. The check lives in `store.UpdateTask`, so both callers —
the console and the agents' `kanban_move` — are covered by one guard rather than
two that can drift.

`internal/store/kanban.go`

---

## 8. Meeting

Not free chat. A facilitator drives a state machine with a hard round cap.

```mermaid
stateDiagram-v2
    [*] --> open: meeting created
    open --> round: Run() starts
    round --> round: facilitator asks one participant,<br/>that agent answers in character
    round --> decided: facilitator judges it converged
    round --> decided: round limit reached (forced)
    round --> closed: facilitator says nothing further is needed
    decided --> closed: tasks and memories written
    closed --> [*]
```

```mermaid
sequenceDiagram
    participant E as Meeting engine
    participant B as Brain (facilitator)
    participant A as Participant role
    participant DB as PostgreSQL

    loop until decided, capped at max_rounds
        E->>DB: read transcript
        E->>B: facilitate(agenda, transcript, participants)
        B-->>E: {action: ask | decide | close, ...}
        alt ask
            E->>DB: recall memories for the agenda and the question
            E->>A: answer as this role, ≤ 6 sentences,<br/>disagree where you disagree
            A-->>E: position with its reason
            E->>DB: append to the transcript
            E->>DB: publish on topic meeting:MEETING-ID
        else decide
            E->>DB: post the decision
            E->>DB: create the tasks it produces
            E->>DB: write the decision memories
            E->>DB: close the meeting
        end
    end
```

Three deliberate choices:

- **Participants speak through the coordinator model, not their coding CLI.** A
  meeting is talking, not building; spawning four CLIs to hold a conversation
  would cost a hundred times more and produce the same words.
- **The round cap forces a decision.** A discussion that has not converged is
  decided on what was said rather than left open forever.
- **Agreement ends the meeting.** The facilitator is told that agreement is a
  reason to decide, not to run another round — otherwise agents restate each
  other politely until the token budget is gone.

A meeting opened by an agent through `meeting_open` is facilitated immediately
in the background. An unfacilitated room stays empty forever.

`internal/meeting/engine.go`, `internal/brain/coordinator.go`

---

## 9. GitHub two-way sync

### Outbound

```mermaid
sequenceDiagram
    participant Any as Console or agent
    participant Bus as Event bus
    participant W as Sync watcher
    participant GH as GitHub

    Any->>Bus: publish task_updated on "boards"
    W->>W: mark the task pending
    Note over W: coalesce for 2s — an update then a move<br/>in the same second is one API call
    W->>W: hash(title, spec, column, state)
    alt hash equals last_pushed_hash
        W-->>W: nothing changed, skip
    else
        W->>GH: create or patch the issue
        W->>W: store the new hash
    end
```

### Inbound

```mermaid
sequenceDiagram
    participant GH as GitHub
    participant H as /webhooks/github
    participant DB as PostgreSQL

    GH->>H: POST with X-Hub-Signature-256
    H->>DB: find the repository by full_name
    alt no secret configured for it
        H-->>GH: 401 — fails closed
    end
    H->>H: verify HMAC-SHA256, constant-time
    alt signature invalid
        H-->>GH: 401
    end
    H->>H: hash the incoming state
    alt hash equals last_pushed_hash
        H-->>GH: 204 — this is the echo of our own push
    end
    alt issues
        H->>DB: create or update the task, set the column from its iofi:COLUMN label
    else issue_comment
        H->>DB: append a task event
    else pull_request
        H->>DB: merged → done and closed; closed → blocked; otherwise review
    end
```

The loop guard is the whole design. Every outbound write stores a hash of what
it pushed; an inbound event hashing to the same value is dropped. Without it,
each side answers the other's echo and the two never settle.

Failing closed on a missing secret is deliberate: an unverified webhook can
create tasks, move the board and close work, and "no secret" is far more often an
oversight than a choice. A repository that genuinely wants unsigned deliveries
sets its secret to `insecure-no-verification`.

`internal/github/sync.go`

---

## 10. Pull request

```mermaid
sequenceDiagram
    participant A as Agent
    participant M as MCP
    participant G as Git
    participant GH as GitHub
    participant DB as PostgreSQL

    A->>M: pr_open {title, body}
    M->>DB: live workspace for this task
    alt no repository workspace
        M-->>A: "commit locally and report in task_update"
    end
    M->>G: any commits ahead of the default branch?
    alt none
        M-->>A: "commit your work first"
    end
    M->>G: push -u origin the task branch
    M->>GH: create PR, body linked with a Closes reference to the issue
    M->>DB: store the pull_request link
    M->>DB: move the task to review
    M-->>A: the PR URL
```

Both refusals are messages the agent can act on, not stack traces. An agent told
"commit your work first" commits; an agent shown a git error guesses.

---

## 11. Shared memory

```mermaid
flowchart LR
    subgraph write
        A[agent finishes something worth keeping] --> B[memory_write:<br/>title, body, kind, scope,<br/>confidence, expires_in_days]
        B --> C[embed the title and body]
        C --> D[(memories)]
    end
    subgraph read
        E[new task dispatched] --> F[embed title + spec]
        F --> G[search global + role + project]
        G --> H[rank by similarity × confidence]
        H --> I[top 8 injected into the brief]
        J[agent calls memory_search mid-run] --> G
    end
    D -.-> G
```

Confidence weights the ranking so a near-match nobody trusts does not outrank a
slightly worse match recorded as settled. With no embedding provider configured,
search degrades to keyword matching rather than returning nothing — memory has to
work before an API key exists.

`internal/store/knowledge.go`, `internal/runner/prompt.go`

---

## 12. Workspace lifecycle

```mermaid
stateDiagram-v2
    [*] --> ready: worktree added on branch iofi/TASK-ID
    ready --> ready: successive runs on the same task reuse it
    ready --> released: task closed or moved to done
    ready --> released: run finished and this was a scratch directory
    ready --> released: Founder releases it from the console
    released --> [*]: worktree removed, row marked
```

Repository worktrees survive the run that created them — the branch still has to
be reviewed, and deleting it at run end would throw away work in flight. They are
reclaimed when the task closes. Scratch directories, which have no branch and
nothing to review, go immediately.

`internal/runner/runner.go:ReleaseTaskWorkspaces`

---

## 13. Live updates

```mermaid
flowchart LR
    A[runner] --> BUS[(in-process bus)]
    B[MCP tools] --> BUS
    C[API handlers] --> BUS
    D[meeting engine] --> BUS
    BUS --> E["/api/events?topic=…&token=…"]
    E --> F[browser EventSource]
```

Topics: `run:<id>`, `board:<id>`, `boards`, `runs`, `meetings`,
`meeting:<id>`, `memory`, and `*` for everything.

Server-Sent Events rather than WebSockets, because the traffic is entirely
server-to-browser. `EventSource` cannot set headers, so the access token travels
as a query parameter on this one route.

Publishing never blocks: a subscriber that cannot keep up drops messages rather
than stalling the agent run that is producing them. A browser that falls behind
refetches; an agent that stalls loses work.

`internal/bus/bus.go`, `internal/api/sse.go`

---

## 14. Failure handling

| Failure | Behaviour |
|---|---|
| Coordinator (LiteLLM) unreachable | Intake and meetings fail with the error. Runs still dispatch: skill selection falls back to priority order and memory recall to keyword search. The console shows a coordinator-offline warning. |
| Embedding provider unavailable | Memories are stored without an embedding; search falls back to keyword matching. |
| Coding CLI missing or exits non-zero | Run marked `failed` with stderr's last decisive lines; agent returns to idle. |
| Agent exceeds two hours | Context deadline cancels the process; the run is marked failed. |
| iofi restarts mid-run | Boot fails the orphaned runs with that reason and resets every agent to idle. |
| Two agents claim one task | `ClaimTask` is a conditional update; exactly one wins, the loser is told it was already claimed. |
| Column at its WIP limit | Move refused, 409, the reason shown. |
| Prerequisite still open | Dispatch refused, blockers named. |
| Webhook without a secret | Rejected, logged. |
| GitHub API error on an automatic push | Logged and skipped; the next change to that task retries, since the stored hash was not updated. |
