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

// maxDiffBytes caps what a tool hands back to a model. A diff of a vendored
// directory is megabytes, and pasting it costs the run its whole context
// window before the agent has read a line of it.
const maxDiffBytes = 60 << 10

// Diff shows the branch against its base plus anything not yet committed.
// A reviewer that sees only committed work misses the change still sitting in
// the working tree, which is exactly where a half-finished task lives.
func (g *Git) Diff(ctx context.Context, path, base string, summary bool) (string, error) {
	args := []string{"diff", "origin/" + base + "...HEAD"}
	if summary {
		args = append(args, "--stat")
	}
	committed, err := g.run(ctx, path, args...)
	if err != nil {
		return "", fmt.Errorf("diff against origin/%s: %w: %s", base, err, committed)
	}
	working, _ := g.run(ctx, path, append([]string{"diff", "HEAD"}, args[2:]...)...)

	var b strings.Builder
	b.WriteString("# committed on this branch vs origin/" + base + "\n")
	b.WriteString(orPlaceholder(committed, "(no commits yet)"))
	if strings.TrimSpace(working) != "" {
		b.WriteString("\n# uncommitted in the working tree\n")
		b.WriteString(working)
	}
	return capDiff(b.String(), maxDiffBytes), nil
}

func (g *Git) Log(ctx context.Context, path, base string, limit int) (string, error) {
	if limit <= 0 {
		limit = 20
	}
	out, err := g.run(ctx, path, "log", "--no-merges", "--format=%h %an: %s",
		fmt.Sprintf("-%d", limit), "origin/"+base+"..HEAD")
	if err != nil {
		return "", fmt.Errorf("log against origin/%s: %w: %s", base, err, out)
	}
	return orPlaceholder(out, "(no commits on this branch yet)"), nil
}

func orPlaceholder(s, placeholder string) string {
	if strings.TrimSpace(s) == "" {
		return placeholder + "\n"
	}
	return s
}

// capDiff truncates with a note telling the reader how to get the rest. The
// plain truncate in adapter.go ends with "..." , which in a diff reads as part
// of the patch rather than as a warning that the patch is incomplete.
func capDiff(s string, max int) string {
	if len(s) <= max {
		return s
	}
	return s[:max] + fmt.Sprintf("\n\n… truncated at %d bytes. Narrow it down with repo_diff "+
		"summary=true, then read the files you care about directly.", max)
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
