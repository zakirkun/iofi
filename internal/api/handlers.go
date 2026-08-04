package api

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"time"

	"github.com/go-chi/chi/v5"

	"iofi/internal/skills"
	"iofi/internal/store"
)

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func fail(w http.ResponseWriter, code int, err error) {
	writeJSON(w, code, map[string]string{"error": err.Error()})
}

// handle maps store errors onto status codes so every handler stays two lines.
func handle(w http.ResponseWriter, v any, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		fail(w, http.StatusNotFound, err)
	case err != nil:
		fail(w, http.StatusInternalServerError, err)
	default:
		writeJSON(w, http.StatusOK, v)
	}
}

func decode(r *http.Request, dst any) error {
	defer r.Body.Close()
	return json.NewDecoder(r.Body).Decode(dst)
}

func (s *Server) health(w http.ResponseWriter, r *http.Request) {
	d := s.healthDetail(r.Context())
	d["ok"] = d["database"] == "up"
	writeJSON(w, http.StatusOK, d)
}

// dashboard is the founder's single view: who exists, what is moving, what it costs.
func (s *Server) dashboard(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	agents, err := s.Store.ListAgents(ctx)
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	roles, _ := s.Store.ListRoles(ctx)
	boards, _ := s.Store.ListBoards(ctx)
	runs, _ := s.Store.ListRuns(ctx, "", 20)
	meetings, _ := s.Store.ListMeetings(ctx, true)

	var tasks []store.Task
	if len(boards) > 0 {
		tasks, _ = s.Store.ListTasks(ctx, store.TaskFilter{BoardID: boards[0].ID, Open: true})
	}
	var cost float64
	var tokens int64
	for _, run := range runs {
		cost += run.CostUSD
		tokens += run.TokensIn + run.TokensOut
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"agents": agents, "roles": roles, "boards": boards, "tasks": tasks,
		"runs": runs, "meetings": meetings,
		"totals": map[string]any{"cost_usd": cost, "tokens": tokens, "open_tasks": len(tasks)},
	})
}

// ---------------------------------------------------------------- roles & skills

func (s *Server) listRoles(w http.ResponseWriter, r *http.Request) {
	reply(w)(nil2empty(s.Store.ListRoles(r.Context())))
}

func (s *Server) getRole(w http.ResponseWriter, r *http.Request) {
	reply(w)(nil2empty(s.Store.GetRole(r.Context(), chi.URLParam(r, "id"))))
}

func (s *Server) upsertRole(w http.ResponseWriter, r *http.Request) {
	var role store.Role
	if err := decode(r, &role); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	reply(w)(nil2empty(s.Store.UpsertRole(r.Context(), role)))
}

func (s *Server) deleteRole(w http.ResponseWriter, r *http.Request) {
	if err := s.Store.DeleteRole(r.Context(), chi.URLParam(r, "id")); err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) roleSkills(w http.ResponseWriter, r *http.Request) {
	reply(w)(nil2empty(s.Store.SkillsForRole(r.Context(), chi.URLParam(r, "id"))))
}

func (s *Server) attachSkill(w http.ResponseWriter, r *http.Request) {
	var body struct {
		SkillID  string `json:"skill_id"`
		Priority int    `json:"priority"`
	}
	if err := decode(r, &body); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	if body.Priority == 0 {
		body.Priority = 100
	}
	if err := s.Store.AttachSkill(r.Context(), chi.URLParam(r, "id"), body.SkillID, body.Priority); err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) detachSkill(w http.ResponseWriter, r *http.Request) {
	if err := s.Store.DetachSkill(r.Context(), chi.URLParam(r, "id"), chi.URLParam(r, "skillID")); err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listSkills(w http.ResponseWriter, r *http.Request) {
	reply(w)(nil2empty(s.Store.ListSkills(r.Context())))
}

// upsertSkill accepts raw markdown and validates it exactly like a file on disk,
// so a skill authored in the web UI cannot skip a required section.
func (s *Server) upsertSkill(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Markdown string `json:"markdown"`
	}
	if err := decode(r, &body); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	sk, err := skills.Parse("web", []byte(body.Markdown))
	if err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	if err := sk.Save(r.Context(), s.Store); err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	reply(w)(nil2empty(s.Store.GetSkillBySlug(r.Context(), sk.Frontmatter.Slug)))
}

func (s *Server) reloadSkills(w http.ResponseWriter, r *http.Request) {
	loaded, errs := skills.LoadDir(r.Context(), s.Store, "skills")
	msgs := make([]string, 0, len(errs))
	for _, e := range errs {
		msgs = append(msgs, e.Error())
	}
	writeJSON(w, http.StatusOK, map[string]any{"loaded": loaded, "errors": msgs})
}

// health is the one unauthenticated route: the browser calls it before the
// founder has entered a token, and it reveals nothing but liveness.
func (s *Server) healthDetail(ctx context.Context) map[string]any {
	out := map[string]any{"database": "up", "litellm": "unknown"}
	if err := s.Store.Pool.Ping(ctx); err != nil {
		out["database"] = err.Error()
	}
	if s.LiteLLMURL != "" {
		client := &http.Client{Timeout: 3 * time.Second}
		req, _ := http.NewRequestWithContext(ctx, http.MethodGet, s.LiteLLMURL+"/health/liveliness", nil)
		resp, err := client.Do(req)
		switch {
		case err != nil:
			out["litellm"] = err.Error()
		case resp.StatusCode >= 300:
			out["litellm"] = resp.Status
			resp.Body.Close()
		default:
			out["litellm"] = "up"
			resp.Body.Close()
		}
	}
	return out
}

func (s *Server) deleteSkill(w http.ResponseWriter, r *http.Request) {
	if err := s.Store.DeleteSkill(r.Context(), chi.URLParam(r, "id")); err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---------------------------------------------------------------- tools & agents

func (s *Server) listTools(w http.ResponseWriter, r *http.Request) {
	reply(w)(nil2empty(s.Store.ListTools(r.Context())))
}

func (s *Server) updateTool(w http.ResponseWriter, r *http.Request) {
	var t store.AgentTool
	if err := decode(r, &t); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	t.ID = chi.URLParam(r, "id")
	reply(w)(nil2empty(s.Store.UpdateTool(r.Context(), t)))
}

func (s *Server) listAgents(w http.ResponseWriter, r *http.Request) {
	reply(w)(nil2empty(s.Store.ListAgents(r.Context())))
}

func (s *Server) createAgent(w http.ResponseWriter, r *http.Request) {
	var a store.Agent
	if err := decode(r, &a); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	reply(w)(nil2empty(s.Store.CreateAgent(r.Context(), a)))
}

func (s *Server) updateAgent(w http.ResponseWriter, r *http.Request) {
	var a store.Agent
	if err := decode(r, &a); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	a.ID = chi.URLParam(r, "id")
	reply(w)(nil2empty(s.Store.UpdateAgent(r.Context(), a)))
}

func (s *Server) deleteAgent(w http.ResponseWriter, r *http.Request) {
	if err := s.Store.DeleteAgent(r.Context(), chi.URLParam(r, "id")); err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---------------------------------------------------------------- boards & tasks

func (s *Server) listBoards(w http.ResponseWriter, r *http.Request) {
	reply(w)(nil2empty(s.Store.ListBoards(r.Context())))
}

func (s *Server) createBoard(w http.ResponseWriter, r *http.Request) {
	var b store.Board
	if err := decode(r, &b); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	reply(w)(nil2empty(s.Store.CreateBoard(r.Context(), b)))
}

func (s *Server) updateBoard(w http.ResponseWriter, r *http.Request) {
	var b store.Board
	if err := decode(r, &b); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	b.ID = chi.URLParam(r, "id")
	reply(w)(nil2empty(s.Store.UpdateBoard(r.Context(), b)))
}

func (s *Server) boardTasks(w http.ResponseWriter, r *http.Request) {
	reply(w)(nil2empty(s.Store.ListTasks(r.Context(), store.TaskFilter{
		BoardID: chi.URLParam(r, "id"),
		Open:    r.URL.Query().Get("all") != "1",
	})))
}

func (s *Server) listTasks(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	limit, _ := strconv.Atoi(q.Get("limit"))
	reply(w)(nil2empty(s.Store.ListTasks(r.Context(), store.TaskFilter{
		BoardID: q.Get("board_id"), ColumnKey: q.Get("column"),
		RoleID: q.Get("role_id"), AgentID: q.Get("agent_id"),
		Open: q.Get("all") != "1", Limit: limit,
	})))
}

func (s *Server) getTask(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	task, err := s.Store.GetTask(ctx, chi.URLParam(r, "id"))
	if err != nil {
		handle(w, nil, err)
		return
	}
	events, _ := s.Store.ListTaskEvents(ctx, task.ID)
	writeJSON(w, http.StatusOK, map[string]any{"task": task, "events": events})
}

func (s *Server) createTask(w http.ResponseWriter, r *http.Request) {
	var t store.Task
	if err := decode(r, &t); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	task, err := s.Store.CreateTask(r.Context(), t, "founder")
	if err == nil {
		s.publishBoard(task.BoardID, "task_created", task)
	}
	handle(w, task, err)
}

func (s *Server) patchTask(w http.ResponseWriter, r *http.Request) {
	var p store.TaskPatch
	if err := decode(r, &p); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	ctx := r.Context()
	task, err := s.Store.UpdateTask(ctx, chi.URLParam(r, "id"), p, "founder")

	var wip store.ErrWIPExceeded
	if errors.As(err, &wip) {
		fail(w, http.StatusConflict, err)
		return
	}
	if err == nil {
		s.publishBoard(task.BoardID, "task_updated", task)
		s.afterTaskChange(ctx, task)
	}
	handle(w, task, err)
}

// publishBoard feeds both the per-board stream the browser watches and the
// global "boards" topic the GitHub sync listens on.
func (s *Server) publishBoard(boardID, kind string, data any) {
	s.Bus.Publish("board:"+boardID, kind, data)
	s.Bus.Publish("boards", kind, data)
}

// afterTaskChange releases the worktree once a task is finished. Worktrees are
// kept while work is in flight and would otherwise accumulate forever.
func (s *Server) afterTaskChange(ctx context.Context, t *store.Task) {
	if t.ClosedAt == nil && t.ColumnKey != "done" {
		return
	}
	if err := s.Runner.ReleaseTaskWorkspaces(context.WithoutCancel(ctx), t.ID); err != nil {
		slog.Warn("workspace release failed", "task", t.ID, "err", err)
	}
}

func (s *Server) deleteTask(w http.ResponseWriter, r *http.Request) {
	if err := s.Store.DeleteTask(r.Context(), chi.URLParam(r, "id")); err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) dispatchTask(w http.ResponseWriter, r *http.Request) {
	reply(w)(nil2empty(s.Runner.Dispatch(r.Context(), chi.URLParam(r, "id"))))
}

func (s *Server) pushTask(w http.ResponseWriter, r *http.Request) {
	if s.GitHub == nil {
		fail(w, http.StatusPreconditionFailed, errors.New("github is not configured"))
		return
	}
	if err := s.GitHub.PushTask(r.Context(), chi.URLParam(r, "id")); err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// intake turns a founder's plain request into a triaged task, plus subtasks
// when the coordinator judges the work spans more than one role.
func (s *Server) intake(w http.ResponseWriter, r *http.Request) {
	var body struct {
		BoardID  string `json:"board_id"`
		Title    string `json:"title"`
		Spec     string `json:"spec"`
		Dispatch bool   `json:"dispatch"`
	}
	if err := decode(r, &body); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	ctx := r.Context()
	roles, err := s.Store.ListRoles(ctx)
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	tri, err := s.Brain.Triage(ctx, body.Title, body.Spec, roles)
	if err != nil {
		fail(w, http.StatusBadGateway, err)
		return
	}
	roleID := roleIDBySlug(roles, tri.RoleSlug)
	newTask := store.Task{
		BoardID: body.BoardID, Title: body.Title, Spec: body.Spec,
		Acceptance: tri.Acceptance, Priority: tri.Priority, Labels: tri.Labels,
		RoleRequired: &roleID, ColumnKey: "ready",
	}
	if tri.Estimate > 0 {
		newTask.Estimate = &tri.Estimate
	}
	parent, err := s.Store.CreateTask(ctx, newTask, "founder")
	if err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	// Keep why this role was chosen: a routing decision nobody can review is a
	// routing decision nobody can correct.
	if tri.Reason != "" {
		_ = s.Store.AddTaskEvent(ctx, parent.ID, "brain", "commented",
			store.JSONMap{"triage_role": tri.RoleSlug, "reason": tri.Reason})
	}

	children := make([]*store.Task, 0, len(tri.Subtasks))
	for _, sub := range tri.Subtasks {
		childRole := roleIDBySlug(roles, sub.RoleSlug)
		child, err := s.Store.CreateTask(ctx, store.Task{
			BoardID: body.BoardID, ParentID: &parent.ID, Title: sub.Title, Spec: sub.Spec,
			Acceptance: sub.Acceptance, RoleRequired: &childRole, ColumnKey: "backlog",
		}, "founder")
		if err == nil {
			children = append(children, child)
		}
	}
	s.publishBoard(parent.BoardID, "task_created", parent)

	if body.Dispatch && len(children) == 0 {
		if _, err := s.Runner.Dispatch(ctx, parent.ID); err != nil {
			writeJSON(w, http.StatusOK, map[string]any{
				"task": parent, "subtasks": children, "triage": tri, "dispatch_error": err.Error()})
			return
		}
	}
	writeJSON(w, http.StatusOK, map[string]any{"task": parent, "subtasks": children, "triage": tri})
}

func roleIDBySlug(roles []store.Role, slug string) string {
	for _, r := range roles {
		if r.Slug == slug {
			return r.ID
		}
	}
	return ""
}

// ---------------------------------------------------------------- memory

func (s *Server) listMemories(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	reply(w)(nil2empty(s.Store.ListMemories(r.Context(), limit)))
}

func (s *Server) createMemory(w http.ResponseWriter, r *http.Request) {
	var m store.Memory
	if err := decode(r, &m); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	emb, _ := s.Brain.Embed(r.Context(), m.Title+"\n"+m.Body)
	reply(w)(nil2empty(s.Store.CreateMemory(r.Context(), m, emb)))
}

func (s *Server) searchMemories(w http.ResponseWriter, r *http.Request) {
	var q store.MemoryQuery
	if err := decode(r, &q); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	q.Embedding, _ = s.Brain.Embed(r.Context(), q.Text)
	reply(w)(nil2empty(s.Store.SearchMemories(r.Context(), q)))
}

func (s *Server) deleteMemory(w http.ResponseWriter, r *http.Request) {
	if err := s.Store.DeleteMemory(r.Context(), chi.URLParam(r, "id")); err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// ---------------------------------------------------------------- meetings

func (s *Server) listMeetings(w http.ResponseWriter, r *http.Request) {
	reply(w)(nil2empty(s.Store.ListMeetings(r.Context(), r.URL.Query().Get("open") == "1")))
}

func (s *Server) createMeeting(w http.ResponseWriter, r *http.Request) {
	var m store.Meeting
	if err := decode(r, &m); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	reply(w)(nil2empty(s.Store.CreateMeeting(r.Context(), m)))
}

func (s *Server) getMeeting(w http.ResponseWriter, r *http.Request) {
	reply(w)(nil2empty(s.Store.GetMeeting(r.Context(), chi.URLParam(r, "id"))))
}

func (s *Server) meetingMessages(w http.ResponseWriter, r *http.Request) {
	since, _ := strconv.ParseInt(r.URL.Query().Get("since"), 10, 64)
	reply(w)(nil2empty(s.Store.ListMessages(r.Context(), chi.URLParam(r, "id"), since)))
}

func (s *Server) postMeetingMessage(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Content string `json:"content"`
	}
	if err := decode(r, &body); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	id := chi.URLParam(r, "id")
	msg, err := s.Store.PostMessage(r.Context(), store.MeetingMessage{
		MeetingID: id, Speaker: "Founder", Content: body.Content,
	})
	if err == nil {
		s.Bus.Publish("meeting:"+id, "message", msg)
	}
	handle(w, msg, err)
}

// runMeeting drives the discussion in the background; the browser follows it on
// the meeting SSE topic.
func (s *Server) runMeeting(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	go func() {
		if err := s.Meeting.Run(context.WithoutCancel(r.Context()), id); err != nil {
			s.Bus.Publish("meeting:"+id, "error", map[string]string{"error": err.Error()})
		}
	}()
	writeJSON(w, http.StatusAccepted, map[string]string{"meeting_id": id, "state": "running"})
}

// ---------------------------------------------------------------- runs & repos

func (s *Server) listRuns(w http.ResponseWriter, r *http.Request) {
	limit, _ := strconv.Atoi(r.URL.Query().Get("limit"))
	reply(w)(nil2empty(s.Store.ListRuns(r.Context(), r.URL.Query().Get("agent_id"), limit)))
}

func (s *Server) getRun(w http.ResponseWriter, r *http.Request) {
	reply(w)(nil2empty(s.Store.GetRun(r.Context(), chi.URLParam(r, "id"))))
}

func (s *Server) runEvents(w http.ResponseWriter, r *http.Request) {
	since, _ := strconv.Atoi(r.URL.Query().Get("since"))
	reply(w)(nil2empty(s.Store.ListRunEvents(r.Context(), chi.URLParam(r, "id"), since)))
}

func (s *Server) cancelRun(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]bool{"cancelled": s.Runner.Cancel(chi.URLParam(r, "id"))})
}

func (s *Server) listRepos(w http.ResponseWriter, r *http.Request) {
	reply(w)(nil2empty(s.Store.ListRepos(r.Context())))
}

// upsertRepo also guarantees the repository has a board. Without that binding,
// agents get a scratch directory instead of a worktree, pr_open cannot work,
// and inbound GitHub issues have nowhere to land — so it is not optional.
func (s *Server) upsertRepo(w http.ResponseWriter, r *http.Request) {
	var repo store.Repo
	if err := decode(r, &repo); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	ctx := r.Context()
	saved, err := s.Store.UpsertRepo(ctx, repo)
	if err != nil {
		handle(w, nil, err)
		return
	}

	board, err := s.Store.BoardForRepo(ctx, saved.ID)
	if errors.Is(err, store.ErrNotFound) {
		board, err = s.Store.CreateBoard(ctx, store.Board{
			Slug: saved.Slug, Name: saved.Slug, RepoID: &saved.ID,
			Columns: store.DefaultColumns(),
		})
	}
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]any{"repo": saved, "board_error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"repo": saved, "board": board})
}

func (s *Server) deleteRepo(w http.ResponseWriter, r *http.Request) {
	if _, err := s.Store.Pool.Exec(r.Context(),
		`DELETE FROM repos WHERE id=$1`, chi.URLParam(r, "id")); err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) listWorkspaces(w http.ResponseWriter, r *http.Request) {
	reply(w)(nil2empty(s.Store.ListWorkspaces(r.Context())))
}

func (s *Server) releaseWorkspace(w http.ResponseWriter, r *http.Request) {
	ctx := r.Context()
	ws, err := s.Store.GetWorkspace(ctx, chi.URLParam(r, "id"))
	if err != nil {
		handle(w, nil, err)
		return
	}
	if ws.TaskID != nil {
		if err := s.Runner.ReleaseTaskWorkspaces(ctx, *ws.TaskID); err != nil {
			fail(w, http.StatusInternalServerError, err)
			return
		}
	} else if err := s.Store.ReleaseWorkspace(ctx, ws.ID); err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) costReport(w http.ResponseWriter, r *http.Request) {
	days, _ := strconv.Atoi(r.URL.Query().Get("days"))
	if days <= 0 {
		days = 7
	}
	reply(w)(nil2empty(s.Store.CostReport(r.Context(), time.Duration(days)*24*time.Hour)))
}

func (s *Server) getPrices(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, s.Store.Prices(r.Context()))
}

func (s *Server) setPrices(w http.ResponseWriter, r *http.Request) {
	var prices map[string]store.TokenPrice
	if err := decode(r, &prices); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	if err := s.Store.SetSetting(r.Context(), store.PricesKey, prices); err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	writeJSON(w, http.StatusOK, prices)
}

func (s *Server) stalledTasks(w http.ResponseWriter, r *http.Request) {
	hours, _ := strconv.Atoi(r.URL.Query().Get("hours"))
	if hours <= 0 {
		hours = 24
	}
	reply(w)(nil2empty(s.Store.StalledTasks(r.Context(),
		chi.URLParam(r, "id"), time.Duration(hours)*time.Hour)))
}

func (s *Server) deleteBoard(w http.ResponseWriter, r *http.Request) {
	if _, err := s.Store.Pool.Exec(r.Context(),
		`DELETE FROM boards WHERE id=$1`, chi.URLParam(r, "id")); err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

// reply is handle in curried form, so a handler can forward a store call's
// (value, error) pair directly: reply(w)(nil2empty(s.Store.ListRoles(ctx))).
func reply(w http.ResponseWriter) func(any, error) {
	return func(v any, err error) { handle(w, v, err) }
}

// nil2empty widens a typed result to any so it can spread into reply.
func nil2empty[T any](v T, err error) (any, error) { return v, err }
