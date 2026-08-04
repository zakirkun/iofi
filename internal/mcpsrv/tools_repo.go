package mcpsrv

import (
	"context"
	"errors"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"iofi/internal/session"
)

func (s *Service) registerRepo(r *registrar, sess *session.Session) {
	addTool(r, "repo_status", func(ctx context.Context, _ struct{}) (*mcp.CallToolResult, error) {
		if s.Git == nil || sess.Workspace == "" {
			return text("no repository workspace attached to this run"), nil
		}
		out, err := s.Git.Status(ctx, sess.Workspace)
		if err != nil {
			return nil, err
		}
		return text("%s", out), nil
	})

	type diffIn struct {
		Summary bool `json:"summary,omitempty" jsonschema:"file-level stat instead of the full patch; start here when the change is large"`
	}
	addTool(r, "repo_diff", func(ctx context.Context, in diffIn) (*mcp.CallToolResult, error) {
		base, err := s.baseBranch(ctx, sess)
		if err != nil {
			return text("%v", err), nil
		}
		out, err := s.Git.Diff(ctx, sess.Workspace, base, in.Summary)
		if err != nil {
			return nil, err
		}
		return text("%s", out), nil
	})

	type logIn struct {
		Limit int `json:"limit,omitempty" jsonschema:"how many commits, default 20"`
	}
	addTool(r, "repo_log", func(ctx context.Context, in logIn) (*mcp.CallToolResult, error) {
		base, err := s.baseBranch(ctx, sess)
		if err != nil {
			return text("%v", err), nil
		}
		out, err := s.Git.Log(ctx, sess.Workspace, base, orInt(in.Limit, 20))
		if err != nil {
			return nil, err
		}
		return text("%s", out), nil
	})

	type prIn struct {
		Title string `json:"title"`
		Body  string `json:"body" jsonschema:"what changed, why, and how it was verified"`
	}
	addTool(r, "pr_open", func(ctx context.Context, in prIn) (*mcp.CallToolResult, error) {
		if s.PR == nil {
			return text("GitHub is not configured; commit locally and report in task_update"), nil
		}
		url, err := s.PR.OpenPR(ctx, sess.TaskID, in.Title, in.Body)
		if err != nil {
			return nil, err
		}
		return text("pull request opened: %s", url), nil
	})

	type prStatusIn struct {
		TaskID string `json:"task_id,omitempty" jsonschema:"defaults to the task you are working on"`
	}
	addTool(r, "pr_status", func(ctx context.Context, in prStatusIn) (*mcp.CallToolResult, error) {
		if s.PR == nil {
			return text("GitHub is not configured, so there is no pull request to check"), nil
		}
		out, err := s.PR.PRStatus(ctx, orStr(in.TaskID, sess.TaskID))
		if err != nil {
			return text("%v", err), nil
		}
		return text("%s", out), nil
	})
}

// baseBranch resolves what this workspace was branched off. Diffing against a
// guessed "main" reports the whole repository as the agent's own change on any
// project that names its trunk differently.
func (s *Service) baseBranch(ctx context.Context, sess *session.Session) (string, error) {
	if s.Git == nil || sess.Workspace == "" {
		return "", errNoWorkspace
	}
	ws, err := s.Store.WorkspaceForTask(ctx, sess.TaskID)
	if err != nil || ws.RepoID == nil {
		return "", errNoWorkspace
	}
	repo, err := s.Store.GetRepo(ctx, *ws.RepoID)
	if err != nil {
		return "", errNoWorkspace
	}
	return repo.DefaultBranch, nil
}

// errNoWorkspace is surfaced as tool text rather than a failure: an analyst
// with no repository is a normal configuration, not something to report as
// broken.
var errNoWorkspace = errors.New(
	"no repository workspace attached to this run, so there is nothing to diff")
