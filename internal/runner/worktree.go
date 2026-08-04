package runner

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// Workspaces are git worktrees off one bare clone per repo. Each agent gets its
// own working directory and branch, so two agents never share an index.
type Git struct {
	DataDir string // <DataDir>/repos/*.git and <DataDir>/work/<agent>/<task>
}

func NewGit(dataDir string) *Git { return &Git{DataDir: dataDir} }

func (g *Git) BarePath(slug string) string {
	return filepath.Join(g.DataDir, "repos", safeName(slug)+".git")
}

// EnsureClone creates the bare clone if missing, otherwise fetches.
func (g *Git) EnsureClone(ctx context.Context, slug, remoteURL string) (string, error) {
	bare := g.BarePath(slug)
	if _, err := os.Stat(bare); err == nil {
		if out, err := g.run(ctx, "", "-C", bare, "fetch", "--prune", "origin"); err != nil {
			return bare, fmt.Errorf("fetch %s: %w: %s", slug, err, out)
		}
		return bare, nil
	}
	if err := os.MkdirAll(filepath.Dir(bare), 0o755); err != nil {
		return "", err
	}
	if out, err := g.run(ctx, "", "clone", "--bare", remoteURL, bare); err != nil {
		return "", fmt.Errorf("clone %s: %w: %s", slug, err, out)
	}
	// A bare clone has no refspec for fetching branches; add one.
	if out, err := g.run(ctx, "", "-C", bare, "config", "remote.origin.fetch",
		"+refs/heads/*:refs/remotes/origin/*"); err != nil {
		return bare, fmt.Errorf("configure refspec: %w: %s", err, out)
	}
	if out, err := g.run(ctx, "", "-C", bare, "fetch", "origin"); err != nil {
		return bare, fmt.Errorf("initial fetch: %w: %s", err, out)
	}
	return bare, nil
}

// AddWorktree checks out a fresh branch off origin/<base> into its own directory.
func (g *Git) AddWorktree(ctx context.Context, bare, agentID, taskID, base string) (path, branch string, err error) {
	path = filepath.Join(g.DataDir, "work", safeName(agentID), safeName(taskID))
	branch = "iofi/" + safeName(taskID)

	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return "", "", err
	}
	if _, err := os.Stat(path); err == nil {
		return path, branch, nil // already checked out
	}
	out, err := g.run(ctx, "", "-C", bare, "worktree", "add", "-B", branch, path, "origin/"+base)
	if err != nil {
		return "", "", fmt.Errorf("worktree add: %w: %s", err, out)
	}
	return path, branch, nil
}

func (g *Git) RemoveWorktree(ctx context.Context, bare, path string) error {
	if out, err := g.run(ctx, "", "-C", bare, "worktree", "remove", "--force", path); err != nil {
		return fmt.Errorf("worktree remove: %w: %s", err, out)
	}
	return nil
}

// ScratchWorkspace is used when an agent has no repo (analysts, finance, …).
func (g *Git) ScratchWorkspace(agentID, taskID string) (string, error) {
	path := filepath.Join(g.DataDir, "work", safeName(agentID), safeName(taskID))
	return path, os.MkdirAll(path, 0o755)
}

func (g *Git) Status(ctx context.Context, path string) (string, error) {
	return g.run(ctx, path, "status", "--short", "--branch")
}

func (g *Git) HasCommits(ctx context.Context, path, base string) (bool, error) {
	out, err := g.run(ctx, path, "rev-list", "--count", "origin/"+base+"..HEAD")
	if err != nil {
		return false, err
	}
	return strings.TrimSpace(out) != "0", nil
}

func (g *Git) Push(ctx context.Context, path, branch string) error {
	if out, err := g.run(ctx, path, "push", "-u", "origin", branch); err != nil {
		return fmt.Errorf("push: %w: %s", err, out)
	}
	return nil
}

func (g *Git) run(ctx context.Context, dir string, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, "git", args...)
	if dir != "" {
		cmd.Dir = dir
	}
	// Never let git stop for credentials inside a headless run.
	cmd.Env = append(os.Environ(), "GIT_TERMINAL_PROMPT=0")
	out, err := cmd.CombinedOutput()
	return string(out), err
}

var unsafeChars = regexp.MustCompile(`[^A-Za-z0-9._-]+`)

func safeName(s string) string {
	s = unsafeChars.ReplaceAllString(s, "-")
	return strings.Trim(s, "-")
}
