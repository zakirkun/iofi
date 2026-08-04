package runner

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"strings"
	"sync"
	"time"

	"iofi/internal/brain"
	"iofi/internal/bus"
	"iofi/internal/session"
	"iofi/internal/store"
)

type Runner struct {
	Store     *store.Store
	Brain     *brain.Client
	Git       *Git
	Bus       *bus.Bus
	Sessions  *session.Registry
	MCPURL    string
	MaxActive int

	// Used only by agents with llm_mode=proxy, to point their CLI at LiteLLM.
	LiteLLMURL string
	LiteLLMKey string

	sem chan struct{}
	mu  sync.Mutex
	// cancel func per run id, so the web UI can stop a run
	cancels map[string]context.CancelFunc
}

func New(st *store.Store, br *brain.Client, git *Git, b *bus.Bus, reg *session.Registry, mcpURL string, maxActive int) *Runner {
	if maxActive <= 0 {
		maxActive = 4
	}
	return &Runner{
		Store: st, Brain: br, Git: git, Bus: b, Sessions: reg,
		MCPURL: mcpURL, MaxActive: maxActive,
		sem:     make(chan struct{}, maxActive),
		cancels: map[string]context.CancelFunc{},
	}
}

// Dispatch prepares a workspace, composes the prompt and queues a run for the
// task. It returns as soon as the run row exists; execution is asynchronous.
func (r *Runner) Dispatch(ctx context.Context, taskID string) (*store.Run, error) {
	task, err := r.Store.GetTask(ctx, taskID)
	if err != nil {
		return nil, err
	}

	// A task whose prerequisites are still open produces work in the wrong
	// order, and the agent has no way to know that from its brief.
	if blockers, err := r.Store.UnmetDependencies(ctx, task.ID); err != nil {
		return nil, err
	} else if len(blockers) > 0 {
		return nil, fmt.Errorf("blocked by unfinished task(s): %s", strings.Join(blockers, "; "))
	}

	agent, err := r.resolveAgent(ctx, task)
	if err != nil {
		return nil, err
	}
	role, err := r.Store.GetRole(ctx, agent.RoleID)
	if err != nil {
		return nil, err
	}

	ws, err := r.prepareWorkspace(ctx, agent, task)
	if err != nil {
		return nil, err
	}

	prompt, err := r.composePrompt(ctx, role, agent, task)
	if err != nil {
		return nil, err
	}

	run, err := r.Store.CreateRun(ctx, store.Run{
		TaskID: &task.ID, AgentID: agent.ID, WorkspaceID: &ws.ID, Prompt: prompt,
	})
	if err != nil {
		return nil, err
	}

	if task.AssigneeID == nil {
		assignee := agent.ID
		_, _ = r.Store.UpdateTask(ctx, task.ID, store.TaskPatch{AssigneeID: &assignee}, "system")
	}

	go r.execute(context.WithoutCancel(ctx), *run, *agent, *role, *task, *ws)
	return run, nil
}

func (r *Runner) resolveAgent(ctx context.Context, task *store.Task) (*store.Agent, error) {
	if task.AssigneeID != nil && *task.AssigneeID != "" {
		return r.Store.GetAgent(ctx, *task.AssigneeID)
	}
	if task.RoleRequired == nil || *task.RoleRequired == "" {
		return nil, fmt.Errorf("task %s has neither an assignee nor a required role", task.ID)
	}
	agent, err := r.Store.PickAgentForRole(ctx, *task.RoleRequired)
	if err == store.ErrNotFound {
		return nil, fmt.Errorf("no agent available for the required role")
	}
	return agent, err
}

func (r *Runner) prepareWorkspace(ctx context.Context, agent *store.Agent, task *store.Task) (*store.Workspace, error) {
	board, err := r.Store.GetBoard(ctx, task.BoardID)
	if err != nil {
		return nil, err
	}

	// No repo on the board: analyst-style roles get a scratch directory.
	if board.RepoID == nil || *board.RepoID == "" {
		path, err := r.Git.ScratchWorkspace(agent.ID, task.ID)
		if err != nil {
			return nil, err
		}
		return r.Store.CreateWorkspace(ctx, store.Workspace{AgentID: agent.ID, TaskID: &task.ID, Path: path})
	}

	repo, err := r.Store.GetRepo(ctx, *board.RepoID)
	if err != nil {
		return nil, err
	}
	bare, err := r.Git.EnsureClone(ctx, repo.Slug, repo.RemoteURL)
	if err != nil {
		return nil, err
	}
	if !repo.Installed {
		_ = r.Store.MarkRepoInstalled(ctx, repo.ID, bare)
	}
	path, branch, err := r.Git.AddWorktree(ctx, bare, agent.ID, task.ID, repo.DefaultBranch)
	if err != nil {
		return nil, err
	}
	return r.Store.CreateWorkspace(ctx, store.Workspace{
		AgentID: agent.ID, RepoID: &repo.ID, TaskID: &task.ID, Path: path, Branch: branch,
	})
}

func (r *Runner) execute(ctx context.Context, run store.Run, agent store.Agent, role store.Role, task store.Task, ws store.Workspace) {
	r.sem <- struct{}{}
	defer func() { <-r.sem }()

	ctx, cancel := context.WithTimeout(ctx, 2*time.Hour)
	defer cancel()
	r.mu.Lock()
	r.cancels[run.ID] = cancel
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		delete(r.cancels, run.ID)
		r.mu.Unlock()
	}()

	topic := "run:" + run.ID
	fail := func(err error) {
		_ = r.Store.FinishRun(ctx, run.ID, "failed", -1, err.Error())
		_ = r.Store.SetAgentStatus(ctx, agent.ID, "error")
		r.Bus.Publish(topic, "error", map[string]string{"error": err.Error()})
		r.Bus.Publish("runs", "finished", map[string]string{"run_id": run.ID, "state": "failed"})
		slog.Error("run failed", "run", run.ID, "agent", agent.Name, "err", err)
	}

	tool, err := r.Store.GetTool(ctx, agent.ToolID)
	if err != nil {
		fail(err)
		return
	}
	adapter, err := NewAdapter(*tool)
	if err != nil {
		fail(err)
		return
	}

	sess := r.Sessions.Create(session.Session{
		RunID: run.ID, AgentID: agent.ID, AgentName: agent.Name,
		RoleID: role.ID, RoleSlug: role.Slug, TaskID: task.ID, BoardID: task.BoardID,
		Workspace: ws.Path, Policy: role.ToolPolicy,
	})
	defer r.Sessions.Revoke(sess.Token)

	_ = r.Store.SetAgentStatus(ctx, agent.ID, "busy")
	defer r.Store.SetAgentStatus(context.WithoutCancel(ctx), agent.ID, "idle")

	model := firstNonEmpty(agent.Model, role.ModelPref)
	events, cmdLine, err := adapter.Start(ctx, RunRequest{
		RunID:     run.ID,
		Workspace: ws.Path,
		Prompt:    run.Prompt,
		Model:     model,
		MCPURL:    r.MCPURL,
		MCPToken:  sess.Token,
		Env:       r.proxyEnv(agent, *tool),
	})
	if err != nil {
		fail(err)
		return
	}
	_ = r.Store.StartRun(ctx, run.ID, cmdLine)
	r.Bus.Publish("runs", "started", map[string]string{"run_id": run.ID, "agent_id": agent.ID})

	seq, exit := 0, 0
	var errText string
	var tokIn, tokOut int64
	var reportedCost float64
	for ev := range events {
		seq++
		_ = r.Store.AppendRunEvent(ctx, run.ID, seq, ev.Kind, store.JSONMap{
			"text": ev.Text, "name": ev.Name, "payload": ev.Payload,
		})
		switch ev.Kind {
		case "usage":
			tokIn, tokOut, reportedCost = tokIn+ev.TokensIn, tokOut+ev.TokensOut, reportedCost+ev.CostUSD
			_ = r.Store.AddRunUsage(ctx, run.ID, ev.TokensIn, ev.TokensOut, ev.CostUSD)
		case "error":
			errText = ev.Text
		case "done":
			exit = ev.ExitCode
		}
		r.Bus.Publish(topic, ev.Kind, ev)
	}

	// Only Claude Code reports money. For the others, price the tokens from the
	// operator's table so Finance is not looking at zeros.
	if reportedCost == 0 && (tokIn > 0 || tokOut > 0) {
		if cost := store.CostOf(r.Store.Prices(ctx), model, tokIn, tokOut); cost > 0 {
			_ = r.Store.AddRunUsage(ctx, run.ID, 0, 0, cost)
		}
	}

	state := "done"
	if exit != 0 || errText != "" {
		state = "failed"
	}
	_ = r.Store.FinishRun(ctx, run.ID, state, exit, errText)

	// A scratch workspace has no branch and nothing to review; keeping it only
	// grows the disk. Repository worktrees stay until the task closes.
	if ws.RepoID == nil {
		_ = r.Store.ReleaseWorkspace(ctx, ws.ID)
	}

	r.Bus.Publish("runs", "finished", map[string]string{"run_id": run.ID, "state": state})
	slog.Info("run finished", "run", run.ID, "agent", agent.Name, "state", state)
}

// ReleaseTaskWorkspaces removes the worktrees a finished task no longer needs.
// Called when a task closes; unreleased worktrees otherwise accumulate forever.
func (r *Runner) ReleaseTaskWorkspaces(ctx context.Context, taskID string) error {
	list, err := r.Store.WorkspacesForTask(ctx, taskID)
	if err != nil {
		return err
	}
	for _, ws := range list {
		if ws.ReleasedAt != nil {
			continue
		}
		if ws.RepoID != nil {
			repo, err := r.Store.GetRepo(ctx, *ws.RepoID)
			if err == nil && repo.BarePath != "" {
				if err := r.Git.RemoveWorktree(ctx, repo.BarePath, ws.Path); err != nil {
					// The branch is already pushed; a leftover directory is not
					// worth failing the task transition over.
					slog.Warn("worktree removal failed", "path", ws.Path, "err", err)
				}
			}
		} else {
			_ = os.RemoveAll(ws.Path)
		}
		if err := r.Store.ReleaseWorkspace(ctx, ws.ID); err != nil {
			return err
		}
	}
	return nil
}

// Cancel stops a running agent process.
func (r *Runner) Cancel(runID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if c, ok := r.cancels[runID]; ok {
		c()
		return true
	}
	return false
}

// proxyEnv points a CLI at LiteLLM when the agent is set to llm_mode=proxy.
// Native mode leaves the CLI's own subscription auth alone.
//
// Each CLI reads a different pair of variables, so the mapping is by tool kind
// with the OpenAI-compatible pair as the default. Anything the operator sets as
// an "env.NAME" key in agents.config wins over these.
func (r *Runner) proxyEnv(a store.Agent, tool store.AgentTool) map[string]string {
	if a.LLMMode != "proxy" {
		return nil
	}
	env := map[string]string{}
	if r.LiteLLMURL != "" {
		switch tool.Kind {
		case "claude_code":
			env["ANTHROPIC_BASE_URL"] = r.LiteLLMURL
			env["ANTHROPIC_AUTH_TOKEN"] = r.LiteLLMKey
			env["ANTHROPIC_API_KEY"] = r.LiteLLMKey
		default:
			env["OPENAI_BASE_URL"] = r.LiteLLMURL + "/v1"
			env["OPENAI_API_KEY"] = r.LiteLLMKey
		}
	}
	for k, v := range a.Config {
		if s, ok := v.(string); ok && strings.HasPrefix(k, "env.") {
			env[strings.TrimPrefix(k, "env.")] = s
		}
	}
	return env
}

func firstNonEmpty(v ...string) string {
	for _, s := range v {
		if s != "" {
			return s
		}
	}
	return ""
}
