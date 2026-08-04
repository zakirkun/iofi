package store

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
	"time"
)

// jsonCol is the shared scan/value behaviour for every jsonb column mapped to a
// Go type. Embed it via the concrete types below.
func scanJSON(src any, dst any) error {
	switch v := src.(type) {
	case nil:
		return nil
	case []byte:
		if len(v) == 0 {
			return nil
		}
		return json.Unmarshal(v, dst)
	case string:
		if v == "" {
			return nil
		}
		return json.Unmarshal([]byte(v), dst)
	default:
		return fmt.Errorf("cannot scan %T into %T", src, dst)
	}
}

// ToolPolicy decides which MCP tools a role's agents may call.
// Empty Allow means "everything not denied".
type ToolPolicy struct {
	Allow []string `json:"allow,omitempty"`
	Deny  []string `json:"deny,omitempty"`
}

func (p *ToolPolicy) Scan(src any) error          { return scanJSON(src, p) }
func (p ToolPolicy) Value() (driver.Value, error) { return json.Marshal(p) }

func (p ToolPolicy) Permits(tool string) bool {
	for _, d := range p.Deny {
		if d == tool {
			return false
		}
	}
	if len(p.Allow) == 0 {
		return true
	}
	for _, a := range p.Allow {
		if a == tool {
			return true
		}
	}
	return false
}

type Column struct {
	Key  string `json:"key"`
	Name string `json:"name"`
	WIP  int    `json:"wip"`
}

type Columns []Column

func (c *Columns) Scan(src any) error          { return scanJSON(src, c) }
func (c Columns) Value() (driver.Value, error) { return json.Marshal(c) }

type JSONMap map[string]any

func (m *JSONMap) Scan(src any) error          { return scanJSON(src, m) }
func (m JSONMap) Value() (driver.Value, error) { return json.Marshal(m) }

type JSONList []string

func (l *JSONList) Scan(src any) error          { return scanJSON(src, l) }
func (l JSONList) Value() (driver.Value, error) { return json.Marshal(l) }

// ---------------------------------------------------------------- org

type Role struct {
	ID           string     `db:"id" json:"id"`
	Slug         string     `db:"slug" json:"slug"`
	Name         string     `db:"name" json:"name"`
	Description  string     `db:"description" json:"description"`
	SystemPrompt string     `db:"system_prompt" json:"system_prompt"`
	ModelPref    string     `db:"model_pref" json:"model_pref"`
	ToolPolicy   ToolPolicy `db:"tool_policy" json:"tool_policy"`
	CreatedAt    time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time  `db:"updated_at" json:"updated_at"`
}

type Skill struct {
	ID          string    `db:"id" json:"id"`
	Slug        string    `db:"slug" json:"slug"`
	Version     int       `db:"version" json:"version"`
	Body        string    `db:"body" json:"body"`
	Frontmatter JSONMap   `db:"frontmatter" json:"frontmatter"`
	SourcePath  string    `db:"source_path" json:"source_path"`
	CreatedAt   time.Time `db:"created_at" json:"created_at"`
	UpdatedAt   time.Time `db:"updated_at" json:"updated_at"`
}

// ---------------------------------------------------------------- tools & agents

type AgentTool struct {
	ID        string    `db:"id" json:"id"`
	Kind      string    `db:"kind" json:"kind"`
	Display   string    `db:"display" json:"display"`
	Transport string    `db:"transport" json:"transport"`
	Command   string    `db:"command" json:"command"`
	Args      JSONList  `db:"args" json:"args"`
	Env       JSONMap   `db:"env" json:"env"`
	ParseMode string    `db:"parse_mode" json:"parse_mode"`
	MCPStyle  string    `db:"mcp_style" json:"mcp_style"`
	MCPTarget string    `db:"mcp_target" json:"mcp_target"`
	Enabled   bool      `db:"enabled" json:"enabled"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

type Agent struct {
	ID        string    `db:"id" json:"id"`
	Name      string    `db:"name" json:"name"`
	RoleID    string    `db:"role_id" json:"role_id"`
	ToolID    string    `db:"tool_id" json:"tool_id"`
	LLMMode   string    `db:"llm_mode" json:"llm_mode"`
	Model     string    `db:"model" json:"model"`
	Status    string    `db:"status" json:"status"`
	Capacity  int       `db:"capacity" json:"capacity"`
	Config    JSONMap   `db:"config" json:"config"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}

// ---------------------------------------------------------------- repos & workspaces

type Repo struct {
	ID            string    `db:"id" json:"id"`
	Slug          string    `db:"slug" json:"slug"`
	RemoteURL     string    `db:"remote_url" json:"remote_url"`
	DefaultBranch string    `db:"default_branch" json:"default_branch"`
	BarePath      string    `db:"bare_path" json:"bare_path"`
	WebhookSecret string    `db:"webhook_secret" json:"-"`
	Installed     bool      `db:"installed" json:"installed"`
	CreatedAt     time.Time `db:"created_at" json:"created_at"`
}

type Workspace struct {
	ID         string     `db:"id" json:"id"`
	AgentID    string     `db:"agent_id" json:"agent_id"`
	RepoID     *string    `db:"repo_id" json:"repo_id"`
	TaskID     *string    `db:"task_id" json:"task_id"`
	Path       string     `db:"path" json:"path"`
	Branch     string     `db:"branch" json:"branch"`
	State      string     `db:"state" json:"state"`
	CreatedAt  time.Time  `db:"created_at" json:"created_at"`
	ReleasedAt *time.Time `db:"released_at" json:"released_at"`
}

// ---------------------------------------------------------------- kanban

type Board struct {
	ID        string    `db:"id" json:"id"`
	Slug      string    `db:"slug" json:"slug"`
	Name      string    `db:"name" json:"name"`
	RepoID    *string   `db:"repo_id" json:"repo_id"`
	Columns   Columns   `db:"columns" json:"columns"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

type Task struct {
	ID           string     `db:"id" json:"id"`
	BoardID      string     `db:"board_id" json:"board_id"`
	ParentID     *string    `db:"parent_id" json:"parent_id"`
	Seq          int64      `db:"seq" json:"seq"`
	Title        string     `db:"title" json:"title"`
	Spec         string     `db:"spec" json:"spec"`
	ColumnKey    string     `db:"column_key" json:"column_key"`
	Position     float64    `db:"position" json:"position"`
	Priority     int        `db:"priority" json:"priority"`
	Estimate     *float64   `db:"estimate" json:"estimate"`
	RoleRequired *string    `db:"role_required" json:"role_required"`
	AssigneeID   *string    `db:"assignee_id" json:"assignee_id"`
	DependsOn    []string   `db:"depends_on" json:"depends_on"`
	Labels       []string   `db:"labels" json:"labels"`
	Acceptance   string     `db:"acceptance" json:"acceptance"`
	CreatedBy    string     `db:"created_by" json:"created_by"`
	CreatedAt    time.Time  `db:"created_at" json:"created_at"`
	UpdatedAt    time.Time  `db:"updated_at" json:"updated_at"`
	ClosedAt     *time.Time `db:"closed_at" json:"closed_at"`
}

type TaskEvent struct {
	ID        int64     `db:"id" json:"id"`
	TaskID    string    `db:"task_id" json:"task_id"`
	Actor     string    `db:"actor" json:"actor"`
	Kind      string    `db:"kind" json:"kind"`
	Data      JSONMap   `db:"data" json:"data"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

// ---------------------------------------------------------------- memory

type Memory struct {
	ID          string     `db:"id" json:"id"`
	Scope       string     `db:"scope" json:"scope"`
	ScopeRef    string     `db:"scope_ref" json:"scope_ref"`
	Kind        string     `db:"kind" json:"kind"`
	Title       string     `db:"title" json:"title"`
	Body        string     `db:"body" json:"body"`
	Tags        []string   `db:"tags" json:"tags"`
	SourceAgent *string    `db:"source_agent" json:"source_agent"`
	SourceTask  *string    `db:"source_task" json:"source_task"`
	Confidence  float32    `db:"confidence" json:"confidence"`
	CreatedAt   time.Time  `db:"created_at" json:"created_at"`
	ExpiresAt   *time.Time `db:"expires_at" json:"expires_at"`
	Score       float64    `db:"score" json:"score,omitempty"` // similarity, search results only
}

// KnowledgeDoc is a reference document the Founder uploaded. Content holds the
// whole extracted markdown; the retrievable passages live in KnowledgeChunk.
type KnowledgeDoc struct {
	ID         string    `db:"id" json:"id"`
	Scope      string    `db:"scope" json:"scope"`
	ScopeRef   string    `db:"scope_ref" json:"scope_ref"`
	Title      string    `db:"title" json:"title"`
	Filename   string    `db:"filename" json:"filename"`
	MIME       string    `db:"mime" json:"mime"`
	Bytes      int64     `db:"bytes" json:"bytes"`
	SHA256     string    `db:"sha256" json:"sha256"`
	SourcePath string    `db:"source_path" json:"-"` // absolute host path, not the founder's business
	Extractor  string    `db:"extractor" json:"extractor"`
	State      string    `db:"state" json:"state"`
	Error      string    `db:"error" json:"error"`
	Content    string    `db:"content" json:"content,omitempty"`
	Tags       []string  `db:"tags" json:"tags"`
	UploadedBy string    `db:"uploaded_by" json:"uploaded_by"`
	CreatedAt  time.Time `db:"created_at" json:"created_at"`
	UpdatedAt  time.Time `db:"updated_at" json:"updated_at"`

	Chunks int `db:"chunks" json:"chunks"` // list views only
}

type KnowledgeChunk struct {
	ID      int64  `db:"id" json:"id"`
	DocID   string `db:"doc_id" json:"doc_id"`
	Idx     int    `db:"idx" json:"idx"`
	Heading string `db:"heading" json:"heading"`
	Content string `db:"content" json:"content"`

	// Search results only.
	Score    float64 `db:"score" json:"score,omitempty"`
	DocTitle string  `db:"doc_title" json:"doc_title,omitempty"`
	DocScope string  `db:"doc_scope" json:"doc_scope,omitempty"`
}

// ---------------------------------------------------------------- meetings

type Meeting struct {
	ID           string     `db:"id" json:"id"`
	Kind         string     `db:"kind" json:"kind"`
	Title        string     `db:"title" json:"title"`
	Agenda       string     `db:"agenda" json:"agenda"`
	State        string     `db:"state" json:"state"`
	Round        int        `db:"round" json:"round"`
	MaxRounds    int        `db:"max_rounds" json:"max_rounds"`
	Participants []string   `db:"participants" json:"participants"`
	Facilitator  string     `db:"facilitator" json:"facilitator"`
	BoardID      *string    `db:"board_id" json:"board_id"`
	Decision     string     `db:"decision" json:"decision"`
	CreatedAt    time.Time  `db:"created_at" json:"created_at"`
	ClosedAt     *time.Time `db:"closed_at" json:"closed_at"`
}

type MeetingMessage struct {
	ID        int64     `db:"id" json:"id"`
	MeetingID string    `db:"meeting_id" json:"meeting_id"`
	AgentID   *string   `db:"agent_id" json:"agent_id"`
	Speaker   string    `db:"speaker" json:"speaker"`
	Round     int       `db:"round" json:"round"`
	Content   string    `db:"content" json:"content"`
	Refs      JSONMap   `db:"refs" json:"refs"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
}

// ---------------------------------------------------------------- runs

type Run struct {
	ID          string     `db:"id" json:"id"`
	TaskID      *string    `db:"task_id" json:"task_id"`
	AgentID     string     `db:"agent_id" json:"agent_id"`
	WorkspaceID *string    `db:"workspace_id" json:"workspace_id"`
	State       string     `db:"state" json:"state"`
	Prompt      string     `db:"prompt" json:"prompt"`
	Command     string     `db:"command" json:"command"`
	ExitCode    *int       `db:"exit_code" json:"exit_code"`
	TokensIn    int64      `db:"tokens_in" json:"tokens_in"`
	TokensOut   int64      `db:"tokens_out" json:"tokens_out"`
	CostUSD     float64    `db:"cost_usd" json:"cost_usd"`
	Error       string     `db:"error" json:"error"`
	StartedAt   *time.Time `db:"started_at" json:"started_at"`
	FinishedAt  *time.Time `db:"finished_at" json:"finished_at"`
	CreatedAt   time.Time  `db:"created_at" json:"created_at"`
}

type RunEvent struct {
	ID      int64     `db:"id" json:"id"`
	RunID   string    `db:"run_id" json:"run_id"`
	Seq     int       `db:"seq" json:"seq"`
	Kind    string    `db:"kind" json:"kind"`
	Payload JSONMap   `db:"payload" json:"payload"`
	At      time.Time `db:"at" json:"at"`
}

type GitHubLink struct {
	ID             string     `db:"id" json:"id"`
	TaskID         string     `db:"task_id" json:"task_id"`
	RepoID         string     `db:"repo_id" json:"repo_id"`
	Kind           string     `db:"kind" json:"kind"`
	Number         int        `db:"number" json:"number"`
	NodeID         string     `db:"node_id" json:"node_id"`
	LastPushedHash string     `db:"last_pushed_hash" json:"-"`
	LastSyncedAt   *time.Time `db:"last_synced_at" json:"last_synced_at"`
}
