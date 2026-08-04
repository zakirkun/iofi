# iofi — Product Requirements

Status: implemented unless a requirement is explicitly marked **Not built**.
Companion documents: [ERD](ERD.md), [FLOWS](FLOWS.md), [ARCHITECTURE](ARCHITECTURE.md).

---

## 1. Problem

A person running a software project alone can drive one coding agent at a time.
Everything the agent does not carry in its own context is lost: what was decided
last week, which convention the codebase follows, who is waiting on what. Adding
a second agent makes it worse — two agents with no shared state duplicate work,
contradict each other's decisions, and collide in the same working directory.

Existing coding agents are excellent at *doing a task*. None of them run a
company: none hold a backlog, none know which role owns a problem, none
remember what another agent learned.

## 2. What iofi is

A control plane that turns a set of coding CLIs into an organisation.

The user is the **Founder**. They state intent. iofi decides which role owns the
work, hands it to an agent bound to that role, gives that agent a private
workspace and a set of tools to run the company with, and records what was
learned so the next agent starts from it.

**One sentence:** iofi is the company; the coding agents are its staff.

## 3. Users

| User | Who they are | What they need |
|---|---|---|
| **Founder** (primary) | One person, technical, running a project or a small product | To state what they want and see it happen, without hand-holding each agent |
| **Agent** (machine) | A coding CLI process, bound to a role, spawned per task | An unambiguous brief, a workspace nobody else touches, tools to report progress and ask questions |
| **Operator** (secondary) | The same person wearing an ops hat | To see cost, catch stalled work, and stop a run that has gone wrong |

There is no multi-tenancy and no user model. One deployment serves one Founder.

## 4. Goals

- **G1** The Founder describes work in plain language; iofi decides the owning role and the acceptance criteria.
- **G2** Any coding CLI can be attached without writing Go. A new tool is a database row.
- **G3** Agents share knowledge. What one agent learns, the next one reads before it starts.
- **G4** Agents run the board themselves — claim, update, move, split, hand off — rather than a human transcribing their output.
- **G5** Disagreement is resolved in a bounded discussion that terminates in a decision and produces tasks.
- **G6** Every agent action is visible live and attributable after the fact.
- **G7** Work lands in GitHub as branches, pull requests and issues, both directions.

## 5. Non-goals

- **N1** Not a coding agent. iofi never writes code; it dispatches tools that do.
- **N2** Not a hosted product. Single deployment, single Founder, no accounts, no billing.
- **N3** Not a sandbox. Isolation is per-workspace, not per-process. See §10.
- **N4** Not a general project management tool. The board exists to feed agents; a human-only workflow would be better served elsewhere.
- **N5** Not a chat interface. The Founder briefs and reviews; they do not converse turn by turn with an agent.

## 6. Core concepts

| Concept | Definition |
|---|---|
| **Role** | A job in the company: CTO, Tech Leader, Lead Programmer, QA, DevOps, Business Analyst, Business Process, Finance, Sprint Manager, Support Ticket. Owns a system prompt, a model preference, and a tool policy. |
| **Skill** | A reusable markdown procedure with six mandatory sections. Many-to-many with roles. |
| **Agent Tool** | A coding CLI, described entirely by a database row: command, args, output parsing, MCP config style. |
| **Agent** | A named worker: one role + one tool + a capacity. Stateless between runs. |
| **Board / Task** | The kanban. Tasks carry a spec, acceptance criteria, an owning role, dependencies and a column. |
| **Memory** | Shared knowledge, scoped global / role / project, searchable by meaning. |
| **Meeting** | A bounded, facilitated discussion between roles that ends in a decision. |
| **Run** | One execution of one agent against one task, with its event stream and token accounting. |
| **Workspace** | A git worktree on a dedicated branch, owned by one agent for one task. |

## 7. Functional requirements

### 7.1 Organisation design

- **FR-1** The Founder can create, edit and delete roles, including the system prompt and the list of MCP tools that role's agents may call.
- **FR-2** Skills are markdown with YAML frontmatter and exactly these sections: *Purpose, When To Use, Procedure, Constraints, Output Contract, Examples*. A skill missing or leaving empty any of them is rejected at load. Consistency is enforced by the loader, not by convention.
- **FR-3** Skills load from `skills/` on boot and are also editable in the web console with identical validation.
- **FR-4** A skill declaring `tools_required` that its target role's policy forbids produces a warning, not a silent failure at run time.
- **FR-5** The Founder can hire, edit, pause and fire agents. A paused agent is never selected for work.
- **FR-6** Coding tools are editable rows: command, args, env, output parse mode, MCP config style and target, transport.

### 7.2 Intake and the board

- **FR-7** The Founder submits a title and free-text detail. The coordinator returns an owning role, a priority, an estimate, acceptance criteria and, when the work spans more than one role or session, a set of subtasks.
- **FR-8** The routing rationale is recorded on the task, so a wrong routing can be reviewed rather than only observed.
- **FR-9** Tasks can be dragged between columns. A column with a WIP limit refuses a move that would exceed it; the refusal is shown, not swallowed.
- **FR-10** A task may depend on other tasks. Dispatch is refused while any prerequisite is open, naming the blockers.
- **FR-11** Every task change is recorded as an attributed event: founder, an agent, the coordinator, or GitHub.

### 7.3 Execution

- **FR-12** Dispatching a task selects the assigned agent, or the least-loaded non-paused agent of the required role that is under capacity.
- **FR-13** Each run gets a git worktree on branch `iofi/<task-id>` off the repository's default branch. Roles with no repository get a scratch directory.
- **FR-14** The agent prompt is composed from: role system prompt, the skills selected for that task, memories recalled for that task, the task spec and acceptance criteria, and the reporting contract.
- **FR-15** Prompts too large for a command line are written to `.iofi/brief.md` in the workspace and the agent is pointed at it.
- **FR-16** Agent output is normalised into one event stream — `text`, `tool_call`, `tool_result`, `usage`, `error`, `done` — regardless of tool. The UI and the database never learn a tool-specific format.
- **FR-17** Runs are visible live over SSE and can be cancelled.
- **FR-18** Token counts are recorded per run. Cost is exact where the tool reports it and estimated from an operator-editable rate table where it does not; estimates are labelled as such.

### 7.4 Agent tooling (MCP)

- **FR-19** iofi exposes an MCP server. Each run receives a scoped bearer token that identifies the agent, its role, its task and its workspace.
- **FR-20** The tool set offered to a run is filtered by the role's policy. A QA agent has no `pr_open`; a Finance agent has no `kanban_move` over code.
- **FR-21** Tools available: `task_list`, `task_get`, `task_claim`, `task_create`, `task_split`, `task_update`, `kanban_move`, `memory_write`, `memory_search`, `meeting_open`, `meeting_post`, `meeting_read`, `handoff`, `ask_role`, `cost_report`, `repo_status`, `pr_open`.
- **FR-22** `task_list` returns assignee, owning role, idle time and dependency-blocked status, and can filter to stalled work — otherwise a Sprint Manager cannot do its job.
- **FR-23** `ask_role` answers immediately, in character, using the target role's prompt and memories. It does not spawn the other agent: two agents blocking on one capacity slot would deadlock.
- **FR-24** `task_claim` is atomic. Two agents racing for the same task: exactly one wins.

### 7.5 Shared memory

- **FR-25** Agents write memories of kind `fact`, `decision`, `convention`, `pitfall` or `artifact`, scoped `global`, `role` or `project`, with a confidence and an optional expiry.
- **FR-26** Search ranks by semantic similarity weighted by confidence, and falls back to keyword matching when no embedding provider is configured — memory must work before an API key exists.
- **FR-27** Relevant memories are injected into every agent prompt without the agent asking.
- **FR-28** Expired memories are excluded from search and deleted by a janitor.

### 7.6 Meetings

- **FR-29** A meeting is a state machine: `open → agenda → round → resolve → decided → closed`, capped at a round limit.
- **FR-30** A facilitator model chooses who speaks next and what they are asked, and decides when the discussion has converged. Free-form agent chat is explicitly rejected: it loops and burns tokens without terminating.
- **FR-31** Participants answer in character from their role prompt and memories, through the coordinator model rather than their coding CLI. A meeting is talking, not building.
- **FR-32** A decision produces board tasks and memories automatically.
- **FR-33** Meetings opened by an agent are facilitated immediately; an unfacilitated room stays empty forever.
- **FR-34** The Founder can speak in any room.

### 7.7 GitHub

- **FR-35** Connecting a repository clones it bare and creates a board bound to it. Without that binding, agents cannot get a worktree and inbound issues have nowhere to land, so it is not optional.
- **FR-36** Inbound: `issues`, `issue_comment` and `pull_request` webhooks reconcile into tasks, comments and column transitions.
- **FR-37** Outbound: board changes are mirrored to the issue automatically, coalesced over a short window.
- **FR-38** A loop guard hashes every outbound write; an inbound event matching that hash is dropped. Without it the two sides answer each other forever.
- **FR-39** `pr_open` pushes the task branch and opens a pull request linked to the issue, then moves the task to review.
- **FR-40** A webhook delivery is rejected unless its repository has a secret. Opting out requires an explicit sentinel value.

### 7.8 Console

- **FR-41** Six views: Command (overview, spend, workspaces, repositories), Kanban, Org, Meetings, Memory, Runs.
- **FR-42** Live updates over SSE for boards, runs and meetings.
- **FR-43** Everything the Founder can change is changeable in the console. No requirement is "edit the database".
- **FR-44** The console is served from the Go binary's embedded filesystem. One binary, no separate web deployment.

## 8. Constraints

| Constraint | Reason |
|---|---|
| Go + Chi + PostgreSQL + pgvector | Single binary, one datastore, vector search without a second service |
| LiteLLM for all coordinator reasoning | One place to route models, count tokens and swap providers |
| React + Vite + Tailwind, embedded via `embed.FS` | The console ships with the binary |
| Git worktrees, not containers | Isolation needed today is index-level. A container per agent means an image carrying four coding CLIs and their credentials |
| Forward-only migrations | Every migration would otherwise need a hand-written inverse that is almost never run |

## 9. Success criteria

- **S1** A Founder can go from a blank database to a merged pull request without editing a config file or a table.
- **S2** Attaching a coding CLI that iofi has never seen requires no Go code.
- **S3** An agent that solves a problem writes a memory that measurably shortens the next agent facing the same problem.
- **S4** A meeting between three roles terminates in a decision and at least one board task, within its round limit.
- **S5** Restarting iofi mid-run leaves no agent stuck busy and no run stuck running.
- **S6** Every figure the Founder sees is labelled actual or estimated.

## 10. Security model

iofi is designed for one Founder running it on a machine they control.

- The API can hire agents, dispatch runs that execute code, and return the entire shared memory. It is therefore **authenticated** — a single bearer token on every route except `/health`. When unset, one is generated at boot and printed, so there is no configuration in which the API is accidentally open.
- Requests are rate limited per client address.
- The MCP endpoint authenticates per run. Tokens expire, and are revoked when the run ends.
- Webhooks fail closed. No secret means no delivery.
- **Agents are not sandboxed.** They run as the iofi user with the coding CLI's permissive mode, in a git worktree on the host. iofi warns at boot when bound to a non-loopback address. Running untrusted repositories, or exposing this beyond loopback, needs container isolation that does not exist yet.

## 11. Known limits

- Cost is exact only where the tool reports it. Everything else is priced from a rate table the operator must keep current.
- No down migrations.
- No multi-tenancy, no audit export, no backup tooling.
- The `mcp_client` transport calls the remote agent once and returns its result; it does not stream intermediate output the way spawned CLIs do.

## 12. Not built

| Item | Why it was left out |
|---|---|
| Container-per-agent | The threat it addresses does not exist while the Founder runs their own repositories on their own machine |
| Down migrations | Maintenance cost on every future migration, for a rollback path that restoring the database already covers |
| Multi-tenant accounts | Non-goal N2 |
| Agent-to-agent free chat | Deliberately replaced by the facilitated meeting; free chat does not terminate |
