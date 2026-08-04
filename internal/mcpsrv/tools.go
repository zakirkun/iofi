package mcpsrv

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"iofi/internal/brain"
	"iofi/internal/session"
	"iofi/internal/store"
)

func addTool[In any](srv *mcp.Server, policy store.ToolPolicy, name, desc string,
	h func(ctx context.Context, in In) (*mcp.CallToolResult, error)) {
	if !policy.Permits(name) {
		return
	}
	mcp.AddTool(srv, &mcp.Tool{Name: name, Description: desc},
		func(ctx context.Context, _ *mcp.CallToolRequest, in In) (*mcp.CallToolResult, any, error) {
			res, err := h(ctx, in)
			if err != nil {
				return text("failed: %v", err), nil, nil // tool-level failure, not transport failure
			}
			return res, nil, nil
		})
}

func (s *Service) register(srv *mcp.Server, sess *session.Session) {
	p := sess.Policy
	actor := "agent:" + sess.AgentID

	// ------------------------------------------------------------ kanban

	type listIn struct {
		Column       string `json:"column,omitempty" jsonschema:"restrict to one board column, e.g. backlog or in_progress"`
		Mine         bool   `json:"mine,omitempty" jsonschema:"only tasks assigned to you"`
		MyRole       bool   `json:"my_role,omitempty" jsonschema:"only tasks requiring your role"`
		StalledHours int    `json:"stalled_hours,omitempty" jsonschema:"only tasks untouched for this many hours and not in backlog or done"`
		Limit        int    `json:"limit,omitempty" jsonschema:"maximum tasks to return, default 25"`
	}
	addTool(srv, p, "task_list",
		"List tasks on your board. Each entry carries its assignee and how long it has sat, "+
			"so stalled work is visible without opening every task.",
		func(ctx context.Context, in listIn) (*mcp.CallToolResult, error) {
			if in.StalledHours > 0 {
				tasks, err := s.Store.StalledTasks(ctx, sess.BoardID,
					time.Duration(in.StalledHours)*time.Hour)
				if err != nil {
					return nil, err
				}
				return jsonText(s.briefTasks(ctx, tasks)), nil
			}
			f := store.TaskFilter{BoardID: sess.BoardID, ColumnKey: in.Column, Open: true, Limit: orInt(in.Limit, 25)}
			if in.Mine {
				f.AgentID = sess.AgentID
			}
			if in.MyRole {
				f.RoleID = sess.RoleID
			}
			tasks, err := s.Store.ListTasks(ctx, f)
			if err != nil {
				return nil, err
			}
			return jsonText(s.briefTasks(ctx, tasks)), nil
		})

	type idIn struct {
		TaskID string `json:"task_id,omitempty" jsonschema:"task id; defaults to the task you are working on"`
	}
	addTool(srv, p, "task_get", "Read the full spec and acceptance criteria of a task.",
		func(ctx context.Context, in idIn) (*mcp.CallToolResult, error) {
			t, err := s.Store.GetTask(ctx, orStr(in.TaskID, sess.TaskID))
			if err != nil {
				return nil, err
			}
			events, _ := s.Store.ListTaskEvents(ctx, t.ID)
			return jsonText(map[string]any{"task": t, "events": events}), nil
		})

	addTool(srv, p, "task_claim", "Claim an unassigned task. Fails if another agent claimed it first.",
		func(ctx context.Context, in idIn) (*mcp.CallToolResult, error) {
			t, err := s.Store.ClaimTask(ctx, in.TaskID, sess.AgentID)
			if err == store.ErrNotFound {
				return text("task %s is already claimed or closed", in.TaskID), nil
			}
			if err != nil {
				return nil, err
			}
			s.publishBoard(sess.BoardID, "task_claimed", t)
			return text("claimed task #%d %q", t.Seq, t.Title), nil
		})

	type createIn struct {
		Title      string   `json:"title" jsonschema:"short imperative title"`
		Spec       string   `json:"spec" jsonschema:"what must be done and why, in markdown"`
		Acceptance string   `json:"acceptance,omitempty" jsonschema:"observable criteria QA can verify without asking you"`
		RoleSlug   string   `json:"role_slug,omitempty" jsonschema:"role that should own this task, e.g. lead-programmer"`
		Priority   int      `json:"priority,omitempty" jsonschema:"1 highest to 5 lowest, default 3"`
		Column     string   `json:"column,omitempty" jsonschema:"starting column, default backlog"`
		Labels     []string `json:"labels,omitempty"`
		ChildOf    string   `json:"child_of,omitempty" jsonschema:"parent task id"`
	}
	addTool(srv, p, "task_create", "Create a task on the board for a role to pick up.",
		func(ctx context.Context, in createIn) (*mcp.CallToolResult, error) {
			t, err := s.newTask(ctx, sess, in.Title, in.Spec, in.Acceptance, in.RoleSlug,
				in.Column, in.ChildOf, in.Priority, in.Labels, actor)
			if err != nil {
				return nil, err
			}
			s.publishBoard(sess.BoardID, "task_created", t)
			return text("created task #%d %q (id %s)", t.Seq, t.Title, t.ID), nil
		})

	type splitIn struct {
		TaskID   string `json:"task_id,omitempty" jsonschema:"parent task, defaults to yours"`
		Subtasks []struct {
			Title      string `json:"title"`
			Spec       string `json:"spec"`
			RoleSlug   string `json:"role_slug,omitempty"`
			Acceptance string `json:"acceptance,omitempty"`
		} `json:"subtasks" jsonschema:"the child tasks to create"`
	}
	addTool(srv, p, "task_split", "Break a task into child tasks, each owned by a role.",
		func(ctx context.Context, in splitIn) (*mcp.CallToolResult, error) {
			parent := orStr(in.TaskID, sess.TaskID)
			var made []string
			for _, sub := range in.Subtasks {
				t, err := s.newTask(ctx, sess, sub.Title, sub.Spec, sub.Acceptance, sub.RoleSlug,
					"backlog", parent, 3, nil, actor)
				if err != nil {
					return nil, err
				}
				made = append(made, fmt.Sprintf("#%d %s", t.Seq, t.Title))
			}
			s.publishBoard(sess.BoardID, "task_split", map[string]any{"parent": parent, "count": len(made)})
			return text("created %d subtasks:\n%s", len(made), strings.Join(made, "\n")), nil
		})

	type updateIn struct {
		TaskID     string `json:"task_id,omitempty" jsonschema:"defaults to the task you are working on"`
		Summary    string `json:"summary" jsonschema:"what you did and how you verified it; required"`
		Spec       string `json:"spec,omitempty" jsonschema:"replace the spec"`
		Acceptance string `json:"acceptance,omitempty" jsonschema:"replace the acceptance criteria"`
		Priority   int    `json:"priority,omitempty"`
		Close      bool   `json:"close,omitempty" jsonschema:"mark the task finished"`
	}
	addTool(srv, p, "task_update", "Report progress on a task. Call this before you finish.",
		func(ctx context.Context, in updateIn) (*mcp.CallToolResult, error) {
			id := orStr(in.TaskID, sess.TaskID)
			patch := store.TaskPatch{}
			if in.Spec != "" {
				patch.Spec = &in.Spec
			}
			if in.Acceptance != "" {
				patch.Acceptance = &in.Acceptance
			}
			if in.Priority > 0 {
				patch.Priority = &in.Priority
			}
			if in.Close {
				patch.Close = &in.Close
			}
			t, err := s.Store.UpdateTask(ctx, id, patch, actor)
			if err != nil {
				return nil, err
			}
			if in.Summary != "" {
				_ = s.Store.AddTaskEvent(ctx, id, actor, "commented", store.JSONMap{"summary": in.Summary})
			}
			s.publishBoard(sess.BoardID, "task_updated", t)
			return text("task #%d updated", t.Seq), nil
		})

	type moveIn struct {
		TaskID string `json:"task_id,omitempty" jsonschema:"defaults to the task you are working on"`
		Column string `json:"column" jsonschema:"target column key, e.g. review, qa, blocked, done"`
		Reason string `json:"reason,omitempty" jsonschema:"why it moved"`
	}
	addTool(srv, p, "kanban_move", "Move a task to another column.",
		func(ctx context.Context, in moveIn) (*mcp.CallToolResult, error) {
			id := orStr(in.TaskID, sess.TaskID)
			board, err := s.Store.GetBoard(ctx, sess.BoardID)
			if err != nil {
				return nil, err
			}
			if !hasColumn(board.Columns, in.Column) {
				return text("no column %q on this board; valid columns: %s",
					in.Column, columnKeys(board.Columns)), nil
			}
			t, err := s.Store.UpdateTask(ctx, id, store.TaskPatch{ColumnKey: &in.Column}, actor)
			if err != nil {
				return nil, err
			}
			if in.Reason != "" {
				_ = s.Store.AddTaskEvent(ctx, id, actor, "commented", store.JSONMap{"reason": in.Reason})
			}
			s.publishBoard(sess.BoardID, "task_moved", t)
			return text("task #%d moved to %s", t.Seq, in.Column), nil
		})

	// ------------------------------------------------------------ memory

	type memWriteIn struct {
		Title         string   `json:"title" jsonschema:"one line naming the thing learned"`
		Body          string   `json:"body" jsonschema:"the knowledge itself, self-contained"`
		Kind          string   `json:"kind,omitempty" jsonschema:"fact, decision, convention, pitfall or artifact"`
		Scope         string   `json:"scope,omitempty" jsonschema:"global (default), role for your role only, or project for this board only"`
		Tags          []string `json:"tags,omitempty"`
		Confidence    float64  `json:"confidence,omitempty" jsonschema:"0 to 1, how settled this is; low confidence ranks lower in search. Default 0.5"`
		ExpiresInDays int      `json:"expires_in_days,omitempty" jsonschema:"drop this memory after N days; use it for anything temporary"`
	}
	addTool(srv, p, "memory_write",
		"Record something other agents need to know. Use it for decisions, conventions and pitfalls.",
		func(ctx context.Context, in memWriteIn) (*mcp.CallToolResult, error) {
			scope, ref := "global", ""
			switch in.Scope {
			case "role":
				scope, ref = "role", sess.RoleID
			case "project":
				scope, ref = "project", sess.BoardID
			}
			m := store.Memory{
				Scope: scope, ScopeRef: ref, Kind: orStr(in.Kind, "fact"),
				Title: in.Title, Body: in.Body, Tags: in.Tags,
				SourceAgent: &sess.AgentID, SourceTask: nilIfBlank(sess.TaskID),
				Confidence: clamp01(in.Confidence),
			}
			if in.ExpiresInDays > 0 {
				exp := time.Now().AddDate(0, 0, in.ExpiresInDays)
				m.ExpiresAt = &exp
			}
			emb, _ := s.Brain.Embed(ctx, in.Title+"\n"+in.Body)
			saved, err := s.Store.CreateMemory(ctx, m, emb)
			if err != nil {
				return nil, err
			}
			s.Bus.Publish("memory", "created", saved)
			return text("stored memory %s", saved.ID), nil
		})

	type memSearchIn struct {
		Query string `json:"query" jsonschema:"what you want to know"`
		Kind  string `json:"kind,omitempty"`
		Limit int    `json:"limit,omitempty" jsonschema:"default 8"`
	}
	addTool(srv, p, "memory_search",
		"Search what the company already knows before deciding or writing new code.",
		func(ctx context.Context, in memSearchIn) (*mcp.CallToolResult, error) {
			emb, _ := s.Brain.Embed(ctx, in.Query)
			mems, err := s.Store.SearchMemories(ctx, store.MemoryQuery{
				Text: in.Query, Embedding: emb, Kind: in.Kind,
				Scopes: []string{"global", "role:" + sess.RoleID, "project:" + sess.BoardID},
				Limit:  orInt(in.Limit, 8),
			})
			if err != nil {
				return nil, err
			}
			if len(mems) == 0 {
				return text("nothing recorded on that yet"), nil
			}
			var b strings.Builder
			for _, m := range mems {
				fmt.Fprintf(&b, "## [%s] %s\n%s\n\n", m.Kind, m.Title, m.Body)
			}
			return text("%s", b.String()), nil
		})

	// ------------------------------------------------------------ collaboration

	type meetOpenIn struct {
		Title  string   `json:"title"`
		Kind   string   `json:"kind,omitempty" jsonschema:"grooming, standup, design, retro or incident"`
		Agenda string   `json:"agenda" jsonschema:"what must be settled, stated as questions"`
		Roles  []string `json:"roles" jsonschema:"role slugs to invite"`
	}
	addTool(srv, p, "meeting_open", "Open a discussion with other roles and let the facilitator run it.",
		func(ctx context.Context, in meetOpenIn) (*mcp.CallToolResult, error) {
			participants, err := s.agentsForRoles(ctx, in.Roles)
			if err != nil {
				return nil, err
			}
			participants = appendUnique(participants, sess.AgentID)
			m, err := s.Store.CreateMeeting(ctx, store.Meeting{
				Kind: orStr(in.Kind, "design"), Title: in.Title, Agenda: in.Agenda,
				Participants: participants, BoardID: nilIfBlank(sess.BoardID),
			})
			if err != nil {
				return nil, err
			}
			s.Bus.Publish("meetings", "opened", m)

			// Opening a room nobody facilitates leaves it empty forever. Drive
			// it in the background — the caller must not block on a discussion.
			if s.Meetings != nil {
				go func() {
					bg := context.WithoutCancel(ctx)
					if err := s.Meetings.Run(bg, m.ID); err != nil {
						s.Bus.Publish("meeting:"+m.ID, "error", map[string]string{"error": err.Error()})
						slog.Warn("meeting run failed", "meeting", m.ID, "err", err)
					}
				}()
			}
			return text("meeting %s opened with %d participants; the facilitator is running it now. "+
				"Use meeting_read to follow it.", m.ID, len(participants)), nil
		})

	type meetPostIn struct {
		MeetingID string `json:"meeting_id"`
		Content   string `json:"content" jsonschema:"your position, with the reason behind it"`
	}
	addTool(srv, p, "meeting_post", "Say something in a meeting.",
		func(ctx context.Context, in meetPostIn) (*mcp.CallToolResult, error) {
			m, err := s.Store.GetMeeting(ctx, in.MeetingID)
			if err != nil {
				return nil, err
			}
			msg, err := s.Store.PostMessage(ctx, store.MeetingMessage{
				MeetingID: m.ID, AgentID: &sess.AgentID, Speaker: sess.AgentName,
				Round: m.Round, Content: in.Content,
			})
			if err != nil {
				return nil, err
			}
			s.Bus.Publish("meeting:"+m.ID, "message", msg)
			return text("posted"), nil
		})

	type meetReadIn struct {
		MeetingID string `json:"meeting_id"`
		SinceID   int64  `json:"since_id,omitempty" jsonschema:"only messages after this id"`
	}
	addTool(srv, p, "meeting_read", "Read a meeting transcript.",
		func(ctx context.Context, in meetReadIn) (*mcp.CallToolResult, error) {
			msgs, err := s.Store.ListMessages(ctx, in.MeetingID, in.SinceID)
			if err != nil {
				return nil, err
			}
			var b strings.Builder
			for _, m := range msgs {
				fmt.Fprintf(&b, "[%d] %s: %s\n", m.ID, m.Speaker, m.Content)
			}
			if b.Len() == 0 {
				return text("no messages yet"), nil
			}
			return text("%s", b.String()), nil
		})

	type handoffIn struct {
		TaskID   string `json:"task_id,omitempty" jsonschema:"defaults to the task you are working on"`
		RoleSlug string `json:"role_slug" jsonschema:"role that should take it over"`
		Note     string `json:"note" jsonschema:"what you did and what they need to do next"`
		Column   string `json:"column,omitempty" jsonschema:"column to move it to, default ready"`
	}
	addTool(srv, p, "handoff", "Pass a task to another role with a note.",
		func(ctx context.Context, in handoffIn) (*mcp.CallToolResult, error) {
			role, err := s.Store.GetRoleBySlug(ctx, in.RoleSlug)
			if err != nil {
				return nil, fmt.Errorf("unknown role %q", in.RoleSlug)
			}
			id := orStr(in.TaskID, sess.TaskID)
			empty, col := "", orStr(in.Column, "ready")
			t, err := s.Store.UpdateTask(ctx, id, store.TaskPatch{
				RoleRequired: &role.ID, AssigneeID: &empty, ColumnKey: &col,
			}, actor)
			if err != nil {
				return nil, err
			}
			_ = s.Store.AddTaskEvent(ctx, id, actor, "commented",
				store.JSONMap{"handoff_to": role.Slug, "note": in.Note})
			s.publishBoard(sess.BoardID, "task_handoff", t)
			return text("task #%d handed to %s", t.Seq, role.Name), nil
		})

	type askIn struct {
		RoleSlug string `json:"role_slug" jsonschema:"role to ask, e.g. cto or tech-leader"`
		Question string `json:"question" jsonschema:"a specific question, not a general topic"`
	}
	addTool(srv, p, "ask_role",
		"Ask another role a question and get an answer now. Use it instead of guessing.",
		func(ctx context.Context, in askIn) (*mcp.CallToolResult, error) {
			return s.askRole(ctx, sess, in.RoleSlug, in.Question)
		})

	// ------------------------------------------------------------ repo

	addTool(srv, p, "repo_status", "Show git status of your workspace.",
		func(ctx context.Context, _ struct{}) (*mcp.CallToolResult, error) {
			if s.Git == nil || sess.Workspace == "" {
				return text("no repository workspace attached to this run"), nil
			}
			out, err := s.Git.Status(ctx, sess.Workspace)
			if err != nil {
				return nil, err
			}
			return text("%s", out), nil
		})

	type costIn struct {
		Days int `json:"days,omitempty" jsonschema:"window in days, default 7"`
	}
	addTool(srv, p, "cost_report",
		"Token and cost totals per agent over a window. Figures from tools that do not "+
			"report money are priced from the configured rate table and are estimates.",
		func(ctx context.Context, in costIn) (*mcp.CallToolResult, error) {
			days := orInt(in.Days, 7)
			lines, err := s.Store.CostReport(ctx, time.Duration(days)*24*time.Hour)
			if err != nil {
				return nil, err
			}
			return jsonText(map[string]any{
				"window_days": days,
				"note":        "cost_usd is actual for Claude Code runs and estimated elsewhere",
				"by_agent":    lines,
			}), nil
		})

	type prIn struct {
		Title string `json:"title"`
		Body  string `json:"body" jsonschema:"what changed, why, and how it was verified"`
	}
	addTool(srv, p, "pr_open", "Push your branch and open a pull request for this task.",
		func(ctx context.Context, in prIn) (*mcp.CallToolResult, error) {
			if s.PR == nil {
				return text("GitHub is not configured; commit locally and report in task_update"), nil
			}
			url, err := s.PR.OpenPR(ctx, sess.TaskID, in.Title, in.Body)
			if err != nil {
				return nil, err
			}
			return text("pull request opened: %s", url), nil
		})
}

// askRole answers as the target role using the coordinator model plus that
// role's system prompt and memories. Spawning the other agent's CLI and waiting
// would deadlock two agents against one capacity slot.
func (s *Service) askRole(ctx context.Context, sess *session.Session, roleSlug, question string) (*mcp.CallToolResult, error) {
	role, err := s.Store.GetRoleBySlug(ctx, roleSlug)
	if err != nil {
		return nil, fmt.Errorf("unknown role %q", roleSlug)
	}
	emb, _ := s.Brain.Embed(ctx, question)
	mems, _ := s.Store.SearchMemories(ctx, store.MemoryQuery{
		Text: question, Embedding: emb,
		Scopes: []string{"global", "role:" + role.ID}, Limit: 6,
	})
	var known strings.Builder
	for _, m := range mems {
		fmt.Fprintf(&known, "- [%s] %s: %s\n", m.Kind, m.Title, m.Body)
	}

	answer, _, err := s.Brain.Chat(ctx, role.ModelPref, []brain.Message{
		{Role: "system", Content: role.SystemPrompt + "\n\nAnswer the question directly and briefly. " +
			"If the answer is not determined by what you know, say so and state the assumption you would make."},
		{Role: "user", Content: fmt.Sprintf(
			"WHAT YOU KNOW:\n%s\n\nASKED BY: %s (%s)\nQUESTION: %s",
			known.String(), sess.AgentName, sess.RoleSlug, question)},
	}, false)
	if err != nil {
		return nil, err
	}
	if sess.TaskID != "" {
		_ = s.Store.AddTaskEvent(ctx, sess.TaskID, "agent:"+sess.AgentID, "commented",
			store.JSONMap{"asked_role": role.Slug, "question": question, "answer": answer})
	}
	return text("%s says:\n\n%s", role.Name, answer), nil
}

func (s *Service) newTask(ctx context.Context, sess *session.Session,
	title, spec, acceptance, roleSlug, column, parent string, priority int,
	labels []string, actor string) (*store.Task, error) {

	t := store.Task{
		BoardID: sess.BoardID, Title: title, Spec: spec, Acceptance: acceptance,
		ColumnKey: column, Priority: priority, Labels: labels,
	}
	if parent != "" {
		t.ParentID = &parent
	}
	if roleSlug != "" {
		role, err := s.Store.GetRoleBySlug(ctx, roleSlug)
		if err != nil {
			return nil, fmt.Errorf("unknown role %q", roleSlug)
		}
		t.RoleRequired = &role.ID
	}
	return s.Store.CreateTask(ctx, t, actor)
}

func (s *Service) agentsForRoles(ctx context.Context, slugs []string) ([]string, error) {
	var out []string
	for _, slug := range slugs {
		role, err := s.Store.GetRoleBySlug(ctx, slug)
		if err != nil {
			return nil, fmt.Errorf("unknown role %q", slug)
		}
		a, err := s.Store.PickAgentForRole(ctx, role.ID)
		if err == store.ErrNotFound {
			continue // a role with no free agent simply does not attend
		}
		if err != nil {
			return nil, err
		}
		out = appendUnique(out, a.ID)
	}
	return out, nil
}

func (s *Service) publishBoard(boardID, kind string, data any) {
	s.Bus.Publish("board:"+boardID, kind, data)
	s.Bus.Publish("boards", kind, data)
}

type taskBrief struct {
	ID        string `json:"id"`
	Seq       int64  `json:"seq"`
	Title     string `json:"title"`
	Column    string `json:"column"`
	Prio      int    `json:"priority"`
	Role      string `json:"role,omitempty"`
	Assignee  string `json:"assignee,omitempty"`
	IdleHours int    `json:"idle_hours"`
	Blocked   bool   `json:"blocked_by_dependency,omitempty"`
}

// briefTasks resolves role and assignee names and reports how long each task
// has sat. A list without those is useless to a sprint manager.
func (s *Service) briefTasks(ctx context.Context, ts []store.Task) []taskBrief {
	roleName := map[string]string{}
	agentName := map[string]string{}
	out := make([]taskBrief, 0, len(ts))
	for _, t := range ts {
		b := taskBrief{
			ID: t.ID, Seq: t.Seq, Title: t.Title, Column: t.ColumnKey, Prio: t.Priority,
			IdleHours: int(time.Since(t.UpdatedAt).Hours()),
		}
		if t.RoleRequired != nil {
			if _, ok := roleName[*t.RoleRequired]; !ok {
				if r, err := s.Store.GetRole(ctx, *t.RoleRequired); err == nil {
					roleName[*t.RoleRequired] = r.Slug
				}
			}
			b.Role = roleName[*t.RoleRequired]
		}
		if t.AssigneeID != nil {
			if _, ok := agentName[*t.AssigneeID]; !ok {
				if a, err := s.Store.GetAgent(ctx, *t.AssigneeID); err == nil {
					agentName[*t.AssigneeID] = a.Name
				}
			}
			b.Assignee = agentName[*t.AssigneeID]
		}
		if len(t.DependsOn) > 0 {
			if unmet, err := s.Store.UnmetDependencies(ctx, t.ID); err == nil {
				b.Blocked = len(unmet) > 0
			}
		}
		out = append(out, b)
	}
	return out
}

func hasColumn(cols store.Columns, key string) bool {
	for _, c := range cols {
		if c.Key == key {
			return true
		}
	}
	return false
}

func columnKeys(cols store.Columns) string {
	keys := make([]string, 0, len(cols))
	for _, c := range cols {
		keys = append(keys, c.Key)
	}
	return strings.Join(keys, ", ")
}

func appendUnique(list []string, v string) []string {
	for _, x := range list {
		if x == v {
			return list
		}
	}
	return append(list, v)
}

func orStr(v, def string) string {
	if v == "" {
		return def
	}
	return v
}

func orInt(v, def int) int {
	if v <= 0 {
		return def
	}
	return v
}

func nilIfBlank(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

// clamp01 keeps a model-supplied confidence inside the range the ranking
// formula assumes. Zero means "unset" and falls back to the store default.
func clamp01(v float64) float32 {
	switch {
	case v <= 0:
		return 0
	case v > 1:
		return 1
	default:
		return float32(v)
	}
}
