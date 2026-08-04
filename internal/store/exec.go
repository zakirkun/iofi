package store

import (
	"context"
	"fmt"
	"strings"
	"time"
)

const runCols = `id, task_id, agent_id, workspace_id, state, prompt, command, exit_code,
	tokens_in, tokens_out, cost_usd, error, started_at, finished_at, created_at`

func (s *Store) CreateRun(ctx context.Context, r Run) (*Run, error) {
	return selectOne[Run](ctx, s, `
		INSERT INTO runs (task_id, agent_id, workspace_id, prompt, command)
		VALUES ($1,$2,$3,$4,$5) RETURNING `+runCols,
		r.TaskID, r.AgentID, r.WorkspaceID, r.Prompt, r.Command)
}

func (s *Store) GetRun(ctx context.Context, id string) (*Run, error) {
	return selectOne[Run](ctx, s, `SELECT `+runCols+` FROM runs WHERE id=$1`, id)
}

type RunFilter struct {
	AgentID string
	TaskID  string
	State   string // queued | running | done | failed | cancelled
	Limit   int
}

func (s *Store) ListRuns(ctx context.Context, f RunFilter) ([]Run, error) {
	var where []string
	var args []any
	add := func(col string, v any) {
		args = append(args, v)
		where = append(where, fmt.Sprintf("%s=$%d", col, len(args)))
	}
	if f.AgentID != "" {
		add("agent_id", f.AgentID)
	}
	if f.TaskID != "" {
		add("task_id", f.TaskID)
	}
	if f.State != "" {
		add("state", f.State)
	}
	sql := `SELECT ` + runCols + ` FROM runs`
	if len(where) > 0 {
		sql += " WHERE " + strings.Join(where, " AND ")
	}
	if f.Limit <= 0 {
		f.Limit = 50
	}
	args = append(args, f.Limit)
	return selectAll[Run](ctx, s, sql+fmt.Sprintf(" ORDER BY created_at DESC LIMIT $%d", len(args)), args...)
}

func (s *Store) StartRun(ctx context.Context, id, command string) error {
	_, err := s.Pool.Exec(ctx,
		`UPDATE runs SET state='running', command=$2, started_at=now() WHERE id=$1`, id, command)
	return err
}

func (s *Store) FinishRun(ctx context.Context, id, state string, exitCode int, errMsg string) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE runs SET state=$2, exit_code=$3, error=$4, finished_at=now() WHERE id=$1`,
		id, state, exitCode, errMsg)
	return err
}

func (s *Store) AddRunUsage(ctx context.Context, id string, in, out int64, cost float64) error {
	_, err := s.Pool.Exec(ctx, `
		UPDATE runs SET tokens_in=tokens_in+$2, tokens_out=tokens_out+$3, cost_usd=cost_usd+$4
		WHERE id=$1`, id, in, out, cost)
	return err
}

func (s *Store) AppendRunEvent(ctx context.Context, runID string, seq int, kind string, payload JSONMap) error {
	if payload == nil {
		payload = JSONMap{}
	}
	_, err := s.Pool.Exec(ctx,
		`INSERT INTO run_events (run_id, seq, kind, payload) VALUES ($1,$2,$3,$4)`,
		runID, seq, kind, payload)
	return err
}

func (s *Store) ListRunEvents(ctx context.Context, runID string, sinceSeq int) ([]RunEvent, error) {
	return selectAll[RunEvent](ctx, s,
		`SELECT id, run_id, seq, kind, payload, at FROM run_events
		 WHERE run_id=$1 AND seq>$2 ORDER BY seq`, runID, sinceSeq)
}

// TailRunEvents returns the last n events in chronological order. Diagnosing a
// failed run means reading its end, and a long run's beginning is noise.
func (s *Store) TailRunEvents(ctx context.Context, runID string, n int) ([]RunEvent, error) {
	if n <= 0 {
		n = 30
	}
	return selectAll[RunEvent](ctx, s, `
		SELECT id, run_id, seq, kind, payload, at FROM (
			SELECT id, run_id, seq, kind, payload, at FROM run_events
			WHERE run_id=$1 ORDER BY seq DESC LIMIT $2
		) t ORDER BY seq`, runID, n)
}

// CostLine is one row of the finance report.
type CostLine struct {
	AgentID   string  `db:"agent_id" json:"agent_id"`
	AgentName string  `db:"agent_name" json:"agent_name"`
	RoleSlug  string  `db:"role_slug" json:"role_slug"`
	Runs      int64   `db:"runs" json:"runs"`
	Failed    int64   `db:"failed" json:"failed"`
	TokensIn  int64   `db:"tokens_in" json:"tokens_in"`
	TokensOut int64   `db:"tokens_out" json:"tokens_out"`
	CostUSD   float64 `db:"cost_usd" json:"cost_usd"`
}

// CostReport aggregates spend per agent over a window. Finance has no way to do
// its job without this.
func (s *Store) CostReport(ctx context.Context, since time.Duration) ([]CostLine, error) {
	return selectAll[CostLine](ctx, s, `
		SELECT a.id AS agent_id, a.name AS agent_name, r0.slug AS role_slug,
		       count(r.id) AS runs,
		       count(*) FILTER (WHERE r.state='failed') AS failed,
		       COALESCE(sum(r.tokens_in),0)  AS tokens_in,
		       COALESCE(sum(r.tokens_out),0) AS tokens_out,
		       COALESCE(sum(r.cost_usd),0)   AS cost_usd
		FROM agents a
		JOIN roles r0 ON r0.id = a.role_id
		LEFT JOIN runs r ON r.agent_id = a.id AND r.created_at > now() - $1::interval
		GROUP BY a.id, a.name, r0.slug
		ORDER BY cost_usd DESC, runs DESC`, since.String())
}

// ---------------------------------------------------------------- workspaces

const wsCols = `id, agent_id, repo_id, task_id, path, branch, state, created_at, released_at`

func (s *Store) CreateWorkspace(ctx context.Context, w Workspace) (*Workspace, error) {
	return selectOne[Workspace](ctx, s, `
		INSERT INTO workspaces (agent_id, repo_id, task_id, path, branch)
		VALUES ($1,$2,$3,$4,$5) RETURNING `+wsCols,
		w.AgentID, w.RepoID, w.TaskID, w.Path, w.Branch)
}

func (s *Store) GetWorkspace(ctx context.Context, id string) (*Workspace, error) {
	return selectOne[Workspace](ctx, s, `SELECT `+wsCols+` FROM workspaces WHERE id=$1`, id)
}

func (s *Store) ListWorkspaces(ctx context.Context) ([]Workspace, error) {
	return selectAll[Workspace](ctx, s,
		`SELECT `+wsCols+` FROM workspaces WHERE released_at IS NULL ORDER BY created_at DESC`)
}

func (s *Store) ReleaseWorkspace(ctx context.Context, id string) error {
	_, err := s.Pool.Exec(ctx,
		`UPDATE workspaces SET state='released', released_at=now() WHERE id=$1`, id)
	return err
}

// WorkspaceForTask returns the live workspace of a task. Scanning every
// workspace to find one is both slow and ambiguous once a task has been
// retried.
func (s *Store) WorkspaceForTask(ctx context.Context, taskID string) (*Workspace, error) {
	return selectOne[Workspace](ctx, s, `
		SELECT `+wsCols+` FROM workspaces
		WHERE task_id=$1 AND released_at IS NULL
		ORDER BY created_at DESC LIMIT 1`, taskID)
}

// WorkspacesForTask includes released ones, for cleanup.
func (s *Store) WorkspacesForTask(ctx context.Context, taskID string) ([]Workspace, error) {
	return selectAll[Workspace](ctx, s,
		`SELECT `+wsCols+` FROM workspaces WHERE task_id=$1`, taskID)
}

// ---------------------------------------------------------------- repos

const repoCols = `id, slug, remote_url, default_branch, bare_path, webhook_secret, installed, created_at`

func (s *Store) ListRepos(ctx context.Context) ([]Repo, error) {
	return selectAll[Repo](ctx, s, `SELECT `+repoCols+` FROM repos ORDER BY slug`)
}

func (s *Store) GetRepo(ctx context.Context, id string) (*Repo, error) {
	return selectOne[Repo](ctx, s, `SELECT `+repoCols+` FROM repos WHERE id=$1`, id)
}

func (s *Store) GetRepoBySlug(ctx context.Context, slug string) (*Repo, error) {
	return selectOne[Repo](ctx, s, `SELECT `+repoCols+` FROM repos WHERE slug=$1`, slug)
}

func (s *Store) UpsertRepo(ctx context.Context, r Repo) (*Repo, error) {
	return selectOne[Repo](ctx, s, `
		INSERT INTO repos (slug, remote_url, default_branch, bare_path, webhook_secret)
		VALUES ($1,$2,$3,$4,$5)
		ON CONFLICT (slug) DO UPDATE SET remote_url=EXCLUDED.remote_url,
			default_branch=EXCLUDED.default_branch, bare_path=EXCLUDED.bare_path,
			webhook_secret=EXCLUDED.webhook_secret
		RETURNING `+repoCols,
		r.Slug, r.RemoteURL, orDefault(r.DefaultBranch, "main"), r.BarePath, r.WebhookSecret)
}

func (s *Store) MarkRepoInstalled(ctx context.Context, id, barePath string) error {
	_, err := s.Pool.Exec(ctx,
		`UPDATE repos SET installed=true, bare_path=$2 WHERE id=$1`, id, barePath)
	return err
}

// ---------------------------------------------------------------- github links

const linkCols = `id, task_id, repo_id, kind, number, node_id, last_pushed_hash, last_synced_at`

func (s *Store) UpsertGitHubLink(ctx context.Context, l GitHubLink) (*GitHubLink, error) {
	return selectOne[GitHubLink](ctx, s, `
		INSERT INTO github_links (task_id, repo_id, kind, number, node_id, last_pushed_hash, last_synced_at)
		VALUES ($1,$2,$3,$4,$5,$6,now())
		ON CONFLICT (repo_id, kind, number) DO UPDATE SET
			task_id=EXCLUDED.task_id, node_id=EXCLUDED.node_id,
			last_pushed_hash=EXCLUDED.last_pushed_hash, last_synced_at=now()
		RETURNING `+linkCols,
		l.TaskID, l.RepoID, orDefault(l.Kind, "issue"), l.Number, l.NodeID, l.LastPushedHash)
}

func (s *Store) GetLinkByNumber(ctx context.Context, repoID, kind string, number int) (*GitHubLink, error) {
	return selectOne[GitHubLink](ctx, s,
		`SELECT `+linkCols+` FROM github_links WHERE repo_id=$1 AND kind=$2 AND number=$3`,
		repoID, kind, number)
}

func (s *Store) GetLinkByTask(ctx context.Context, taskID, kind string) (*GitHubLink, error) {
	return selectOne[GitHubLink](ctx, s,
		`SELECT `+linkCols+` FROM github_links WHERE task_id=$1 AND kind=$2`, taskID, kind)
}

func (s *Store) SetPushedHash(ctx context.Context, id, hash string) error {
	_, err := s.Pool.Exec(ctx,
		`UPDATE github_links SET last_pushed_hash=$2, last_synced_at=now() WHERE id=$1`, id, hash)
	return err
}
