CREATE EXTENSION IF NOT EXISTS pgcrypto;
CREATE EXTENSION IF NOT EXISTS vector;

-- ---------------------------------------------------------------- org

CREATE TABLE roles (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    slug          text UNIQUE NOT NULL,
    name          text NOT NULL,
    description   text NOT NULL DEFAULT '',
    system_prompt text NOT NULL,
    model_pref    text NOT NULL DEFAULT '',
    -- {"allow":["task_update","memory_search"],"deny":["pr_open"]}
    tool_policy   jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE skills (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    slug         text UNIQUE NOT NULL,
    version      int  NOT NULL DEFAULT 1,
    body         text NOT NULL,               -- markdown, sections validated on load
    frontmatter  jsonb NOT NULL DEFAULT '{}'::jsonb,
    source_path  text NOT NULL DEFAULT '',    -- file it was loaded from, '' if web-authored
    created_at   timestamptz NOT NULL DEFAULT now(),
    updated_at   timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE role_skills (
    role_id  uuid NOT NULL REFERENCES roles(id)  ON DELETE CASCADE,
    skill_id uuid NOT NULL REFERENCES skills(id) ON DELETE CASCADE,
    priority int  NOT NULL DEFAULT 100,
    PRIMARY KEY (role_id, skill_id)
);

-- ---------------------------------------------------------------- tools & agents

-- One row per coding tool binary. GenericAdapter reads cmd/args/parse from here,
-- so a new CLI is a row, not a code change.
CREATE TABLE agent_tools (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    kind        text UNIQUE NOT NULL,        -- claude_code | codex | opencode | command_code | <any>
    display     text NOT NULL,
    transport   text NOT NULL DEFAULT 'spawn',  -- spawn | mcp_client
    command     text NOT NULL DEFAULT '',
    args        jsonb NOT NULL DEFAULT '[]'::jsonb,   -- templated: {{.Prompt}} {{.Workspace}} {{.MCPConfig}}
    env         jsonb NOT NULL DEFAULT '{}'::jsonb,
    parse_mode  text NOT NULL DEFAULT 'plain',        -- jsonl | plain | claude_stream_json
    mcp_style   text NOT NULL DEFAULT 'flag',         -- flag | config_file | env
    mcp_target  text NOT NULL DEFAULT '',             -- flag name or config file path template
    enabled     boolean NOT NULL DEFAULT true,
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE agents (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    name        text UNIQUE NOT NULL,
    role_id     uuid NOT NULL REFERENCES roles(id) ON DELETE RESTRICT,
    tool_id     uuid NOT NULL REFERENCES agent_tools(id) ON DELETE RESTRICT,
    llm_mode    text NOT NULL DEFAULT 'native',  -- native (CLI subscription) | proxy (LiteLLM)
    model       text NOT NULL DEFAULT '',        -- overrides roles.model_pref
    status      text NOT NULL DEFAULT 'idle',    -- idle | busy | paused | error
    capacity    int  NOT NULL DEFAULT 1,         -- concurrent runs allowed
    config      jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at  timestamptz NOT NULL DEFAULT now(),
    updated_at  timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX agents_role_idx ON agents(role_id);
CREATE INDEX agents_status_idx ON agents(status);

-- ---------------------------------------------------------------- repos & workspaces

CREATE TABLE repos (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    slug        text UNIQUE NOT NULL,       -- owner/name
    remote_url  text NOT NULL,
    default_branch text NOT NULL DEFAULT 'main',
    bare_path   text NOT NULL DEFAULT '',   -- data/repos/<slug>.git
    webhook_secret text NOT NULL DEFAULT '',
    installed   boolean NOT NULL DEFAULT false,
    created_at  timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE workspaces (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id   uuid NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    repo_id    uuid REFERENCES repos(id) ON DELETE SET NULL,
    task_id    uuid,                        -- FK added after tasks exists
    path       text NOT NULL,               -- worktree dir
    branch     text NOT NULL DEFAULT '',
    state      text NOT NULL DEFAULT 'ready', -- ready | dirty | released | error
    created_at timestamptz NOT NULL DEFAULT now(),
    released_at timestamptz
);
CREATE INDEX workspaces_agent_idx ON workspaces(agent_id);

-- ---------------------------------------------------------------- kanban

CREATE TABLE boards (
    id         uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    slug       text UNIQUE NOT NULL,
    name       text NOT NULL,
    repo_id    uuid REFERENCES repos(id) ON DELETE SET NULL,
    -- [{"key":"backlog","name":"Backlog","wip":0}, ...]
    columns    jsonb NOT NULL DEFAULT '[]'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);

CREATE TABLE tasks (
    id            uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    board_id      uuid NOT NULL REFERENCES boards(id) ON DELETE CASCADE,
    parent_id     uuid REFERENCES tasks(id) ON DELETE SET NULL,
    seq           bigserial,                 -- human-facing number
    title         text NOT NULL,
    spec          text NOT NULL DEFAULT '',  -- markdown brief handed to the agent
    column_key    text NOT NULL DEFAULT 'backlog',
    position      double precision NOT NULL DEFAULT 0,  -- ordering within column
    priority      int  NOT NULL DEFAULT 3,   -- 1 highest
    estimate      numeric,
    role_required uuid REFERENCES roles(id) ON DELETE SET NULL,
    assignee_id   uuid REFERENCES agents(id) ON DELETE SET NULL,
    depends_on    uuid[] NOT NULL DEFAULT '{}',
    labels        text[] NOT NULL DEFAULT '{}',
    acceptance    text NOT NULL DEFAULT '',
    created_by    text NOT NULL DEFAULT 'founder', -- founder | agent:<id> | github
    created_at    timestamptz NOT NULL DEFAULT now(),
    updated_at    timestamptz NOT NULL DEFAULT now(),
    closed_at     timestamptz
);
CREATE INDEX tasks_board_col_idx ON tasks(board_id, column_key, position);
CREATE INDEX tasks_assignee_idx  ON tasks(assignee_id);
CREATE INDEX tasks_role_idx      ON tasks(role_required);

ALTER TABLE workspaces
    ADD CONSTRAINT workspaces_task_fk
    FOREIGN KEY (task_id) REFERENCES tasks(id) ON DELETE SET NULL;

CREATE TABLE task_events (
    id         bigserial PRIMARY KEY,
    task_id    uuid NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    actor      text NOT NULL,               -- founder | agent:<id> | github | brain
    kind       text NOT NULL,               -- created | moved | assigned | commented | closed
    data       jsonb NOT NULL DEFAULT '{}'::jsonb,
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX task_events_task_idx ON task_events(task_id, id);

-- ---------------------------------------------------------------- shared memory

CREATE TABLE memories (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    scope       text NOT NULL DEFAULT 'global',  -- global | role | project | agent
    scope_ref   text NOT NULL DEFAULT '',        -- id of the scoped entity
    kind        text NOT NULL DEFAULT 'fact',    -- fact | decision | convention | pitfall | artifact
    title       text NOT NULL DEFAULT '',
    body        text NOT NULL,
    embedding   vector(1536),
    tags        text[] NOT NULL DEFAULT '{}',
    source_agent uuid REFERENCES agents(id) ON DELETE SET NULL,
    source_task  uuid REFERENCES tasks(id)  ON DELETE SET NULL,
    confidence  real NOT NULL DEFAULT 0.5,
    created_at  timestamptz NOT NULL DEFAULT now(),
    expires_at  timestamptz
);
CREATE INDEX memories_scope_idx ON memories(scope, scope_ref);
CREATE INDEX memories_tags_idx  ON memories USING gin(tags);
CREATE INDEX memories_vec_idx   ON memories USING hnsw (embedding vector_cosine_ops);

-- ---------------------------------------------------------------- meetings

CREATE TABLE meetings (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    kind         text NOT NULL DEFAULT 'design', -- grooming | standup | design | retro | incident
    title        text NOT NULL,
    agenda       text NOT NULL DEFAULT '',
    state        text NOT NULL DEFAULT 'open',   -- open | agenda | round | resolve | decided | closed
    round        int  NOT NULL DEFAULT 0,
    max_rounds   int  NOT NULL DEFAULT 3,
    participants uuid[] NOT NULL DEFAULT '{}',   -- agent ids
    facilitator  text NOT NULL DEFAULT 'brain',
    board_id     uuid REFERENCES boards(id) ON DELETE SET NULL,
    decision     text NOT NULL DEFAULT '',
    created_at   timestamptz NOT NULL DEFAULT now(),
    closed_at    timestamptz
);

CREATE TABLE meeting_messages (
    id         bigserial PRIMARY KEY,
    meeting_id uuid NOT NULL REFERENCES meetings(id) ON DELETE CASCADE,
    agent_id   uuid REFERENCES agents(id) ON DELETE SET NULL,
    speaker    text NOT NULL,              -- founder | brain | agent name
    round      int  NOT NULL DEFAULT 0,
    content    text NOT NULL,
    refs       jsonb NOT NULL DEFAULT '{}'::jsonb, -- {"tasks":[],"memories":[]}
    created_at timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX meeting_messages_idx ON meeting_messages(meeting_id, id);

-- ---------------------------------------------------------------- runs

CREATE TABLE runs (
    id           uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id      uuid REFERENCES tasks(id) ON DELETE SET NULL,
    agent_id     uuid NOT NULL REFERENCES agents(id) ON DELETE CASCADE,
    workspace_id uuid REFERENCES workspaces(id) ON DELETE SET NULL,
    state        text NOT NULL DEFAULT 'queued', -- queued | running | done | failed | cancelled
    prompt       text NOT NULL DEFAULT '',
    command      text NOT NULL DEFAULT '',
    exit_code    int,
    tokens_in    bigint NOT NULL DEFAULT 0,
    tokens_out   bigint NOT NULL DEFAULT 0,
    cost_usd     numeric(12,6) NOT NULL DEFAULT 0,
    error        text NOT NULL DEFAULT '',
    started_at   timestamptz,
    finished_at  timestamptz,
    created_at   timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX runs_agent_idx ON runs(agent_id, created_at DESC);
CREATE INDEX runs_task_idx  ON runs(task_id);

CREATE TABLE run_events (
    id       bigserial PRIMARY KEY,
    run_id   uuid NOT NULL REFERENCES runs(id) ON DELETE CASCADE,
    seq      int  NOT NULL,
    kind     text NOT NULL,                -- text | tool_call | tool_result | usage | error | done
    payload  jsonb NOT NULL DEFAULT '{}'::jsonb,
    at       timestamptz NOT NULL DEFAULT now()
);
CREATE INDEX run_events_idx ON run_events(run_id, seq);

-- ---------------------------------------------------------------- github sync

CREATE TABLE github_links (
    id          uuid PRIMARY KEY DEFAULT gen_random_uuid(),
    task_id     uuid NOT NULL REFERENCES tasks(id) ON DELETE CASCADE,
    repo_id     uuid NOT NULL REFERENCES repos(id) ON DELETE CASCADE,
    kind        text NOT NULL DEFAULT 'issue',  -- issue | pull_request
    number      int  NOT NULL,
    node_id     text NOT NULL DEFAULT '',
    -- loop guard: hash of the payload we last pushed; matching inbound events are dropped
    last_pushed_hash text NOT NULL DEFAULT '',
    last_synced_at   timestamptz,
    UNIQUE (repo_id, kind, number)
);
CREATE INDEX github_links_task_idx ON github_links(task_id);

CREATE TABLE settings (
    key        text PRIMARY KEY,
    value      jsonb NOT NULL,
    updated_at timestamptz NOT NULL DEFAULT now()
);
