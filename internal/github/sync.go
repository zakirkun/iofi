package github

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"iofi/internal/bus"
	"iofi/internal/runner"
	"iofi/internal/store"
)

// Sync is the two-way bridge. Every outbound write records a hash of what it
// pushed; the inbound webhook drops events whose payload matches that hash.
// Without the guard, each side keeps answering the other's echo.
type Sync struct {
	Store  *store.Store
	Client *Client
	Bus    *bus.Bus
	Git    *runner.Git
	Secret string // fallback webhook secret when the repo row has none
}

func NewSync(st *store.Store, c *Client, b *bus.Bus, g *runner.Git, secret string) *Sync {
	return &Sync{Store: st, Client: c, Bus: b, Git: g, Secret: secret}
}

func stateHash(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	return hex.EncodeToString(sum[:8])
}

// ---------------------------------------------------------------- outbound

// PushTask mirrors a task to its GitHub issue, creating the issue on first push.
func (s *Sync) PushTask(ctx context.Context, taskID string) error {
	if !s.Client.Enabled() {
		return nil
	}
	task, err := s.Store.GetTask(ctx, taskID)
	if err != nil {
		return err
	}
	board, err := s.Store.GetBoard(ctx, task.BoardID)
	if err != nil || board.RepoID == nil {
		return err
	}
	repo, err := s.Store.GetRepo(ctx, *board.RepoID)
	if err != nil {
		return err
	}

	state := "open"
	if task.ClosedAt != nil || task.ColumnKey == "done" {
		state = "closed"
	}
	labels := append(append([]string{}, task.Labels...), "iofi:"+task.ColumnKey)
	hash := stateHash(task.Title, task.Spec, task.ColumnKey, state)

	link, err := s.Store.GetLinkByTask(ctx, task.ID, "issue")
	if err == store.ErrNotFound {
		issue, err := s.Client.CreateIssue(ctx, repo.Slug, task.Title, issueBody(task), labels)
		if err != nil {
			return err
		}
		_, err = s.Store.UpsertGitHubLink(ctx, store.GitHubLink{
			TaskID: task.ID, RepoID: repo.ID, Kind: "issue",
			Number: issue.Number, NodeID: issue.NodeID, LastPushedHash: hash,
		})
		return err
	}
	if err != nil {
		return err
	}
	if link.LastPushedHash == hash {
		return nil // nothing changed since the last push
	}
	if _, err := s.Client.UpdateIssue(ctx, repo.Slug, link.Number, map[string]any{
		"title": task.Title, "body": issueBody(task), "state": state, "labels": labels,
	}); err != nil {
		return err
	}
	return s.Store.SetPushedHash(ctx, link.ID, hash)
}

func issueBody(t *store.Task) string {
	var b strings.Builder
	b.WriteString(t.Spec)
	if t.Acceptance != "" {
		b.WriteString("\n\n### Acceptance criteria\n" + t.Acceptance)
	}
	fmt.Fprintf(&b, "\n\n<!-- iofi:task:%s -->", t.ID)
	return b.String()
}

// OpenPR pushes the task's branch and opens a pull request. Implements
// mcpsrv.PRService.
func (s *Sync) OpenPR(ctx context.Context, taskID, title, body string) (string, error) {
	if !s.Client.Enabled() {
		return "", fmt.Errorf("github token not configured")
	}
	task, err := s.Store.GetTask(ctx, taskID)
	if err != nil {
		return "", err
	}
	ws, err := s.workspaceFor(ctx, taskID)
	if err != nil {
		return "", err
	}
	if ws.RepoID == nil {
		return "", fmt.Errorf("task %s has no repository workspace", taskID)
	}
	repo, err := s.Store.GetRepo(ctx, *ws.RepoID)
	if err != nil {
		return "", err
	}

	has, err := s.Git.HasCommits(ctx, ws.Path, repo.DefaultBranch)
	if err != nil {
		return "", err
	}
	if !has {
		return "", fmt.Errorf("branch %s has no commits; commit your work first", ws.Branch)
	}
	if err := s.Git.Push(ctx, ws.Path, ws.Branch); err != nil {
		return "", err
	}

	if body == "" {
		body = task.Spec
	}
	body += fmt.Sprintf("\n\n<!-- iofi:task:%s -->", task.ID)
	if link, err := s.Store.GetLinkByTask(ctx, task.ID, "issue"); err == nil {
		body += fmt.Sprintf("\n\nCloses #%d", link.Number)
	}

	pr, err := s.Client.CreatePR(ctx, repo.Slug, orStr(title, task.Title), body,
		ws.Branch, repo.DefaultBranch)
	if err != nil {
		return "", err
	}
	if _, err := s.Store.UpsertGitHubLink(ctx, store.GitHubLink{
		TaskID: task.ID, RepoID: repo.ID, Kind: "pull_request",
		Number: pr.Number, NodeID: pr.NodeID,
		LastPushedHash: stateHash("pr", pr.Head.Ref),
	}); err != nil {
		return pr.HTMLURL, err
	}
	review := "review"
	_, _ = s.Store.UpdateTask(ctx, task.ID, store.TaskPatch{ColumnKey: &review}, "github")
	s.Bus.Publish("board:"+task.BoardID, "task_moved", map[string]any{"task_id": task.ID, "column": review})
	return pr.HTMLURL, nil
}

// PRStatus reports the live state of the pull request opened for a task.
// Implements mcpsrv.PRService. It reads GitHub rather than the local link row
// because merge and close happen there, and a webhook that never arrived would
// leave the local copy claiming a merged PR is still open.
func (s *Sync) PRStatus(ctx context.Context, taskID string) (string, error) {
	if !s.Client.Enabled() {
		return "", fmt.Errorf("github token not configured")
	}
	link, err := s.Store.GetLinkByTask(ctx, taskID, "pull_request")
	if err == store.ErrNotFound {
		return "", fmt.Errorf("no pull request has been opened for task %s", taskID)
	}
	if err != nil {
		return "", err
	}
	repo, err := s.Store.GetRepo(ctx, link.RepoID)
	if err != nil {
		return "", err
	}
	pr, err := s.Client.GetPR(ctx, repo.Slug, link.Number)
	if err != nil {
		return "", err
	}
	state := pr.State
	if pr.Merged {
		state = "merged"
	}
	return fmt.Sprintf("PR #%d on %s is %s\n%s", pr.Number, repo.Slug, state, pr.HTMLURL), nil
}

func (s *Sync) workspaceFor(ctx context.Context, taskID string) (*store.Workspace, error) {
	ws, err := s.Store.WorkspaceForTask(ctx, taskID)
	if err == store.ErrNotFound {
		return nil, fmt.Errorf("no active workspace for task %s", taskID)
	}
	return ws, err
}

// Watch mirrors board changes to GitHub as they happen. Without it the outbound
// half of the sync only runs when somebody presses a button.
//
// The loop guard in PushTask drops pushes whose state already matches what we
// last sent, so an inbound event that triggers a local update does not bounce
// straight back out.
func (s *Sync) Watch(ctx context.Context) {
	if !s.Client.Enabled() {
		return
	}
	ch, cancel := s.Bus.Subscribe("boards")
	defer cancel()

	// Coalesce bursts: an agent that updates and then moves a task in the same
	// second should produce one API call, not two.
	pending := map[string]bool{}
	tick := time.NewTicker(2 * time.Second)
	defer tick.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case msg, ok := <-ch:
			if !ok {
				return
			}
			if id := taskIDOf(msg.Data); id != "" && strings.HasPrefix(msg.Kind, "task_") {
				pending[id] = true
			}
		case <-tick.C:
			for id := range pending {
				delete(pending, id)
				if err := s.PushTask(ctx, id); err != nil {
					slog.Warn("github push failed", "task", id, "err", err)
				}
			}
		}
	}
}

// taskIDOf digs the task id out of whatever shape the publisher used.
func taskIDOf(data any) string {
	switch v := data.(type) {
	case *store.Task:
		return v.ID
	case store.Task:
		return v.ID
	case map[string]any:
		if id, ok := v["task_id"].(string); ok {
			return id
		}
		if id, ok := v["id"].(string); ok {
			return id
		}
	}
	return ""
}

// ---------------------------------------------------------------- inbound

type webhookPayload struct {
	Action     string `json:"action"`
	Repository struct {
		FullName string `json:"full_name"`
	} `json:"repository"`
	Issue       *Issue       `json:"issue"`
	PullRequest *PullRequest `json:"pull_request"`
	Comment     *struct {
		Body string `json:"body"`
		User struct {
			Login string `json:"login"`
		} `json:"user"`
	} `json:"comment"`
}

// Webhook handles issues, issue_comment and pull_request events.
func (s *Sync) Webhook(w http.ResponseWriter, r *http.Request) {
	raw, err := io.ReadAll(io.LimitReader(r.Body, 4<<20))
	if err != nil {
		http.Error(w, "read body", http.StatusBadRequest)
		return
	}
	var p webhookPayload
	if err := json.Unmarshal(raw, &p); err != nil {
		http.Error(w, "bad json", http.StatusBadRequest)
		return
	}

	ctx := r.Context()
	repo, err := s.Store.GetRepoBySlug(ctx, p.Repository.FullName)
	if err != nil {
		http.Error(w, "unknown repository", http.StatusNotFound)
		return
	}
	secret := repo.WebhookSecret
	if secret == "" {
		secret = s.Secret
	}
	if !validSignature(secret, raw, r.Header.Get("X-Hub-Signature-256")) {
		if secret == "" {
			slog.Warn("rejected an unverifiable webhook: no secret configured",
				"repo", p.Repository.FullName)
			http.Error(w, "no webhook secret configured for this repository", http.StatusUnauthorized)
			return
		}
		http.Error(w, "bad signature", http.StatusUnauthorized)
		return
	}

	switch r.Header.Get("X-GitHub-Event") {
	case "issues":
		err = s.onIssue(ctx, repo, &p)
	case "issue_comment":
		err = s.onComment(ctx, repo, &p)
	case "pull_request":
		err = s.onPullRequest(ctx, repo, &p)
	}
	if err != nil {
		slog.Error("github webhook", "event", r.Header.Get("X-GitHub-Event"), "err", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// OptOut is the explicit value a repository must carry to accept unsigned
// deliveries. Without it, no secret means no webhook — an unauthenticated
// webhook lets anyone create tasks and move the board.
const OptOut = "insecure-no-verification"

func validSignature(secret string, body []byte, header string) bool {
	if secret == "" {
		return false
	}
	if secret == OptOut {
		return true
	}
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	want := "sha256=" + hex.EncodeToString(mac.Sum(nil))
	return hmac.Equal([]byte(want), []byte(header))
}

func (s *Sync) onIssue(ctx context.Context, repo *store.Repo, p *webhookPayload) error {
	if p.Issue == nil || p.Issue.PR != nil {
		return nil // pull requests arrive again as pull_request events
	}
	board, err := s.boardFor(ctx, repo.ID)
	if err != nil {
		return err
	}
	state := p.Issue.State
	hash := stateHash(p.Issue.Title, p.Issue.Body, columnFromLabels(p.Issue.Labels), state)

	link, err := s.Store.GetLinkByNumber(ctx, repo.ID, "issue", p.Issue.Number)
	if err == nil {
		if link.LastPushedHash == hash {
			return nil // this is the echo of our own push
		}
		closed := state == "closed"
		patch := store.TaskPatch{Title: &p.Issue.Title, Spec: &p.Issue.Body, Close: &closed}
		if col := columnFromLabels(p.Issue.Labels); col != "" {
			patch.ColumnKey = &col
		}
		t, err := s.Store.UpdateTask(ctx, link.TaskID, patch, "github")
		if err != nil {
			return err
		}
		if err := s.Store.SetPushedHash(ctx, link.ID, hash); err != nil {
			return err
		}
		s.Bus.Publish("board:"+t.BoardID, "task_updated", t)
		return nil
	}
	if err != store.ErrNotFound {
		return err
	}

	task, err := s.Store.CreateTask(ctx, store.Task{
		BoardID: board.ID, Title: p.Issue.Title, Spec: p.Issue.Body,
		ColumnKey: "backlog", Labels: labelNames(p.Issue.Labels),
	}, "github")
	if err != nil {
		return err
	}
	if _, err := s.Store.UpsertGitHubLink(ctx, store.GitHubLink{
		TaskID: task.ID, RepoID: repo.ID, Kind: "issue",
		Number: p.Issue.Number, NodeID: p.Issue.NodeID, LastPushedHash: hash,
	}); err != nil {
		return err
	}
	s.Bus.Publish("board:"+board.ID, "task_created", task)
	s.Bus.Publish("github", "issue_imported", map[string]any{"task_id": task.ID, "number": p.Issue.Number})
	return nil
}

func (s *Sync) onComment(ctx context.Context, repo *store.Repo, p *webhookPayload) error {
	if p.Issue == nil || p.Comment == nil {
		return nil
	}
	link, err := s.Store.GetLinkByNumber(ctx, repo.ID, "issue", p.Issue.Number)
	if err != nil {
		return nil // comment on something iofi does not track
	}
	return s.Store.AddTaskEvent(ctx, link.TaskID, "github", "commented", store.JSONMap{
		"author": p.Comment.User.Login, "body": p.Comment.Body,
	})
}

func (s *Sync) onPullRequest(ctx context.Context, repo *store.Repo, p *webhookPayload) error {
	if p.PullRequest == nil {
		return nil
	}
	link, err := s.Store.GetLinkByNumber(ctx, repo.ID, "pull_request", p.PullRequest.Number)
	if err != nil {
		return nil
	}
	col := "review"
	closed := false
	switch {
	case p.PullRequest.Merged:
		col, closed = "done", true
	case p.PullRequest.State == "closed":
		col = "blocked"
	}
	t, err := s.Store.UpdateTask(ctx, link.TaskID,
		store.TaskPatch{ColumnKey: &col, Close: &closed}, "github")
	if err != nil {
		return err
	}
	s.Bus.Publish("board:"+t.BoardID, "task_moved", t)
	return nil
}

func (s *Sync) boardFor(ctx context.Context, repoID string) (*store.Board, error) {
	b, err := s.Store.BoardForRepo(ctx, repoID)
	if err == store.ErrNotFound {
		return nil, fmt.Errorf("no board is bound to this repository; "+
			"bind one under Kanban → board settings (repo %s)", repoID)
	}
	return b, err
}

func columnFromLabels(labels []Label) string {
	for _, l := range labels {
		if col, ok := strings.CutPrefix(l.Name, "iofi:"); ok {
			return col
		}
	}
	return ""
}

func labelNames(labels []Label) []string {
	out := make([]string, 0, len(labels))
	for _, l := range labels {
		if !strings.HasPrefix(l.Name, "iofi:") {
			out = append(out, l.Name)
		}
	}
	return out
}

func orStr(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
