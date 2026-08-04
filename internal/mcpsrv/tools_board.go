package mcpsrv

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"iofi/internal/session"
	"iofi/internal/store"
)

func (s *Service) registerBoard(r *registrar, sess *session.Session) {
	actor := actorOf(sess)

	type listIn struct {
		Column       string `json:"column,omitempty" jsonschema:"restrict to one board column, e.g. backlog or in_progress"`
		Mine         bool   `json:"mine,omitempty" jsonschema:"only tasks assigned to you"`
		MyRole       bool   `json:"my_role,omitempty" jsonschema:"only tasks requiring your role"`
		StalledHours int    `json:"stalled_hours,omitempty" jsonschema:"only tasks untouched for this many hours and not in backlog or done"`
		Limit        int    `json:"limit,omitempty" jsonschema:"maximum tasks to return, default 25"`
	}
	addTool(r, "task_list", func(ctx context.Context, in listIn) (*mcp.CallToolResult, error) {
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
	addTool(r, "task_get", func(ctx context.Context, in idIn) (*mcp.CallToolResult, error) {
		t, err := s.Store.GetTask(ctx, orStr(in.TaskID, sess.TaskID))
		if err != nil {
			return nil, err
		}
		events, _ := s.Store.ListTaskEvents(ctx, t.ID)
		out := map[string]any{"task": t, "events": events}
		// A task whose prerequisites are open cannot be worked on yet, and that
		// fact is invisible in the raw depends_on id list.
		if len(t.DependsOn) > 0 {
			if unmet, err := s.Store.UnmetDependencies(ctx, t.ID); err == nil && len(unmet) > 0 {
				out["blocked_by"] = unmet
			}
		}
		return jsonText(out), nil
	})

	type searchIn struct {
		Query    string `json:"query" jsonschema:"words to match in the title, spec or acceptance criteria"`
		OpenOnly bool   `json:"open_only,omitempty" jsonschema:"exclude finished tasks; off by default because finished work is usually the answer"`
		Limit    int    `json:"limit,omitempty" jsonschema:"default 20"`
	}
	addTool(r, "task_search", func(ctx context.Context, in searchIn) (*mcp.CallToolResult, error) {
		if strings.TrimSpace(in.Query) == "" {
			return text("give a query; an empty search returns the whole board"), nil
		}
		tasks, err := s.Store.SearchTasks(ctx, sess.BoardID, in.Query, in.OpenOnly, orInt(in.Limit, 20))
		if err != nil {
			return nil, err
		}
		if len(tasks) == 0 {
			return text("no task matches %q", in.Query), nil
		}
		return jsonText(s.briefTasks(ctx, tasks)), nil
	})

	addTool(r, "task_claim", func(ctx context.Context, in idIn) (*mcp.CallToolResult, error) {
		id := orStr(in.TaskID, sess.TaskID)
		// Claiming past an open prerequisite is how a board produces work in the
		// wrong order; the dependency is only useful if something enforces it.
		if unmet, err := s.Store.UnmetDependencies(ctx, id); err == nil && len(unmet) > 0 {
			return text("task %s depends on work that is not done yet:\n%s\nFinish those first "+
				"or call task_depend to drop the link if it no longer applies.",
				id, strings.Join(unmet, "\n")), nil
		}
		t, err := s.Store.ClaimTask(ctx, id, sess.AgentID)
		if err == store.ErrNotFound {
			return text("task %s is already claimed or closed", id), nil
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
	addTool(r, "task_create", func(ctx context.Context, in createIn) (*mcp.CallToolResult, error) {
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
	addTool(r, "task_split", func(ctx context.Context, in splitIn) (*mcp.CallToolResult, error) {
		parent := orStr(in.TaskID, sess.TaskID)
		var made []string
		for _, sub := range in.Subtasks {
			t, err := s.newTask(ctx, sess, sub.Title, sub.Spec, sub.Acceptance, sub.RoleSlug,
				"backlog", parent, 3, nil, actor)
			if err != nil {
				return nil, err
			}
			made = append(made, fmt.Sprintf("#%d %s (id %s)", t.Seq, t.Title, t.ID))
		}
		s.publishBoard(sess.BoardID, "task_split", map[string]any{"parent": parent, "count": len(made)})
		return text("created %d subtasks:\n%s\n\nIf they must run in a set order, call "+
			"task_depend now — split alone does not order them.",
			len(made), strings.Join(made, "\n")), nil
	})

	type updateIn struct {
		TaskID     string  `json:"task_id,omitempty" jsonschema:"defaults to the task you are working on"`
		Summary    string  `json:"summary" jsonschema:"what you did and how you verified it; required"`
		Spec       string  `json:"spec,omitempty" jsonschema:"replace the spec"`
		Acceptance string  `json:"acceptance,omitempty" jsonschema:"replace the acceptance criteria"`
		Priority   int     `json:"priority,omitempty"`
		Estimate   float64 `json:"estimate,omitempty" jsonschema:"effort estimate in hours"`
		Close      bool    `json:"close,omitempty" jsonschema:"mark the task finished"`
	}
	addTool(r, "task_update", func(ctx context.Context, in updateIn) (*mcp.CallToolResult, error) {
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
		if in.Estimate > 0 {
			patch.Estimate = &in.Estimate
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

	type assignIn struct {
		TaskID    string `json:"task_id,omitempty" jsonschema:"defaults to the task you are working on"`
		AgentName string `json:"agent_name,omitempty" jsonschema:"exact agent name from agent_roster; leave empty to unassign"`
		Reason    string `json:"reason,omitempty" jsonschema:"why this person"`
	}
	addTool(r, "task_assign", func(ctx context.Context, in assignIn) (*mcp.CallToolResult, error) {
		id := orStr(in.TaskID, sess.TaskID)
		if in.AgentName == "" {
			empty := ""
			t, err := s.Store.UpdateTask(ctx, id, store.TaskPatch{AssigneeID: &empty}, actor)
			if err != nil {
				return nil, err
			}
			s.publishBoard(sess.BoardID, "task_updated", t)
			return text("task #%d is now unassigned", t.Seq), nil
		}
		a, err := s.Store.GetAgentByName(ctx, in.AgentName)
		if err != nil {
			return nil, fmt.Errorf("no agent named %q; call agent_roster for the exact names", in.AgentName)
		}
		t, err := s.Store.UpdateTask(ctx, id, store.TaskPatch{AssigneeID: &a.ID}, actor)
		if err != nil {
			return nil, err
		}
		if in.Reason != "" {
			_ = s.Store.AddTaskEvent(ctx, id, actor, "commented", store.JSONMap{"assign_reason": in.Reason})
		}
		s.publishBoard(sess.BoardID, "task_updated", t)

		// Assigning across roles is allowed — a sprint manager sometimes must —
		// but silently doing it hides why the wrong person got the work.
		note := ""
		if t.RoleRequired != nil && *t.RoleRequired != a.RoleID {
			if want, err := s.Store.GetRole(ctx, *t.RoleRequired); err == nil {
				note = fmt.Sprintf(" (note: this task asks for role %s, which is not theirs)", want.Slug)
			}
		}
		return text("task #%d assigned to %s%s", t.Seq, a.Name, note), nil
	})

	type dependIn struct {
		TaskID    string   `json:"task_id,omitempty" jsonschema:"the task that must wait; defaults to yours"`
		DependsOn []string `json:"depends_on,omitempty" jsonschema:"task ids that must be done first"`
		Remove    []string `json:"remove,omitempty" jsonschema:"task ids to unlink"`
	}
	addTool(r, "task_depend", func(ctx context.Context, in dependIn) (*mcp.CallToolResult, error) {
		id := orStr(in.TaskID, sess.TaskID)
		t, err := s.Store.GetTask(ctx, id)
		if err != nil {
			return nil, err
		}
		next := without(t.DependsOn, in.Remove)
		for _, d := range in.DependsOn {
			if d == id {
				return text("a task cannot depend on itself"), nil
			}
			if _, err := s.Store.GetTask(ctx, d); err != nil {
				return nil, fmt.Errorf("no task with id %q", d)
			}
			next = appendUnique(next, d)
		}
		// A cycle makes every task in it permanently unstartable, and nothing
		// downstream would report why.
		if cyclic, err := s.Store.WouldCycle(ctx, id, next); err != nil {
			return nil, err
		} else if cyclic {
			return text("that link would create a dependency cycle; the tasks in it could never start"), nil
		}
		updated, err := s.Store.UpdateTask(ctx, id, store.TaskPatch{DependsOn: &next}, actor)
		if err != nil {
			return nil, err
		}
		s.publishBoard(sess.BoardID, "task_updated", updated)
		unmet, _ := s.Store.UnmetDependencies(ctx, id)
		if len(unmet) == 0 {
			return text("task #%d now has %d dependencies, all satisfied", updated.Seq, len(next)), nil
		}
		return text("task #%d waits on:\n%s", updated.Seq, strings.Join(unmet, "\n")), nil
	})

	type commentIn struct {
		TaskID string `json:"task_id,omitempty" jsonschema:"defaults to the task you are working on"`
		Note   string `json:"note" jsonschema:"what you found, asked or decided"`
	}
	addTool(r, "task_comment", func(ctx context.Context, in commentIn) (*mcp.CallToolResult, error) {
		id := orStr(in.TaskID, sess.TaskID)
		if strings.TrimSpace(in.Note) == "" {
			return text("an empty comment tells the next agent nothing"), nil
		}
		if err := s.Store.AddTaskEvent(ctx, id, actor, "commented",
			store.JSONMap{"note": in.Note}); err != nil {
			return nil, err
		}
		s.publishBoard(sess.BoardID, "task_updated", map[string]any{"id": id})
		return text("noted on task %s", id), nil
	})

	type labelIn struct {
		TaskID string   `json:"task_id,omitempty" jsonschema:"defaults to the task you are working on"`
		Add    []string `json:"add,omitempty"`
		Remove []string `json:"remove,omitempty"`
	}
	addTool(r, "task_label", func(ctx context.Context, in labelIn) (*mcp.CallToolResult, error) {
		id := orStr(in.TaskID, sess.TaskID)
		t, err := s.Store.GetTask(ctx, id)
		if err != nil {
			return nil, err
		}
		next := without(t.Labels, in.Remove)
		for _, l := range in.Add {
			next = appendUnique(next, l)
		}
		updated, err := s.Store.UpdateTask(ctx, id, store.TaskPatch{Labels: &next}, actor)
		if err != nil {
			return nil, err
		}
		s.publishBoard(sess.BoardID, "task_updated", updated)
		if len(next) == 0 {
			return text("task #%d has no labels", updated.Seq), nil
		}
		return text("task #%d labels: %s", updated.Seq, strings.Join(next, ", ")), nil
	})

	type moveIn struct {
		TaskID string `json:"task_id,omitempty" jsonschema:"defaults to the task you are working on"`
		Column string `json:"column" jsonschema:"target column key, e.g. review, qa, blocked, done"`
		Reason string `json:"reason,omitempty" jsonschema:"why it moved"`
	}
	addTool(r, "kanban_move", func(ctx context.Context, in moveIn) (*mcp.CallToolResult, error) {
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
			var wip store.ErrWIPExceeded
			if errors.As(err, &wip) {
				return text("%v — call board_get to see what is sitting there", wip), nil
			}
			return nil, err
		}
		if in.Reason != "" {
			_ = s.Store.AddTaskEvent(ctx, id, actor, "commented", store.JSONMap{"reason": in.Reason})
		}
		s.publishBoard(sess.BoardID, "task_moved", t)
		return text("task #%d moved to %s", t.Seq, in.Column), nil
	})

	addTool(r, "board_get", func(ctx context.Context, _ struct{}) (*mcp.CallToolResult, error) {
		board, err := s.Store.GetBoard(ctx, sess.BoardID)
		if err != nil {
			return nil, err
		}
		counts, err := s.Store.ColumnCounts(ctx, sess.BoardID)
		if err != nil {
			return nil, err
		}
		type colView struct {
			Key  string `json:"key"`
			Name string `json:"name"`
			WIP  int    `json:"wip_limit"`
			Open int    `json:"open_tasks"`
			Full bool   `json:"at_limit,omitempty"`
		}
		cols := make([]colView, 0, len(board.Columns))
		for _, c := range board.Columns {
			v := colView{Key: c.Key, Name: c.Name, WIP: c.WIP, Open: counts[c.Key]}
			v.Full = c.WIP > 0 && counts[c.Key] >= c.WIP
			cols = append(cols, v)
		}
		return jsonText(map[string]any{"board": board.Name, "columns": cols}), nil
	})
}
