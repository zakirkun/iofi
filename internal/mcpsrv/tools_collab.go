package mcpsrv

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"iofi/internal/session"
	"iofi/internal/store"
)

func (s *Service) registerCollab(r *registrar, sess *session.Session) {
	actor := actorOf(sess)

	type meetOpenIn struct {
		Title  string   `json:"title"`
		Kind   string   `json:"kind,omitempty" jsonschema:"grooming, standup, design, retro or incident"`
		Agenda string   `json:"agenda" jsonschema:"what must be settled, stated as questions"`
		Roles  []string `json:"roles" jsonschema:"role slugs to invite"`
	}
	addTool(r, "meeting_open", func(ctx context.Context, in meetOpenIn) (*mcp.CallToolResult, error) {
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
			"Use meeting_read to follow it and meeting_close to record what it decided.",
			m.ID, len(participants)), nil
	})

	type meetPostIn struct {
		MeetingID string `json:"meeting_id"`
		Content   string `json:"content" jsonschema:"your position, with the reason behind it"`
	}
	addTool(r, "meeting_post", func(ctx context.Context, in meetPostIn) (*mcp.CallToolResult, error) {
		m, err := s.Store.GetMeeting(ctx, in.MeetingID)
		if err != nil {
			return nil, err
		}
		if m.ClosedAt != nil {
			return text("meeting %s is closed; it decided: %s", m.ID, orStr(m.Decision, "(nothing recorded)")), nil
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
	addTool(r, "meeting_read", func(ctx context.Context, in meetReadIn) (*mcp.CallToolResult, error) {
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

	type meetListIn struct {
		OpenOnly bool `json:"open_only,omitempty" jsonschema:"hide meetings that already decided, default false"`
	}
	addTool(r, "meeting_list", func(ctx context.Context, in meetListIn) (*mcp.CallToolResult, error) {
		ms, err := s.Store.MeetingsForAgent(ctx, sess.AgentID, in.OpenOnly)
		if err != nil {
			return nil, err
		}
		if len(ms) == 0 {
			return text("you are not in any meeting"), nil
		}
		type view struct {
			ID       string `json:"id"`
			Kind     string `json:"kind"`
			Title    string `json:"title"`
			State    string `json:"state"`
			Round    int    `json:"round"`
			Decision string `json:"decision,omitempty"`
			Open     bool   `json:"open"`
		}
		out := make([]view, 0, len(ms))
		for _, m := range ms {
			out = append(out, view{
				ID: m.ID, Kind: m.Kind, Title: m.Title, State: m.State,
				Round: m.Round, Decision: m.Decision, Open: m.ClosedAt == nil,
			})
		}
		return jsonText(out), nil
	})

	type meetCloseIn struct {
		MeetingID string `json:"meeting_id"`
		Decision  string `json:"decision" jsonschema:"what was settled and the reason, including the option that was rejected"`
	}
	addTool(r, "meeting_close", func(ctx context.Context, in meetCloseIn) (*mcp.CallToolResult, error) {
		if strings.TrimSpace(in.Decision) == "" {
			return text("state the decision; a meeting closed without one wasted everyone's turn"), nil
		}
		m, err := s.Store.GetMeeting(ctx, in.MeetingID)
		if err != nil {
			return nil, err
		}
		if m.ClosedAt != nil {
			return text("meeting %s was already closed and decided: %s", m.ID, m.Decision), nil
		}
		closed, err := s.Store.CloseMeeting(ctx, in.MeetingID, in.Decision)
		if err != nil {
			return nil, err
		}
		// A decision that lives only in a transcript is one nobody will find.
		emb, _ := s.Brain.Embed(ctx, closed.Title+"\n"+in.Decision)
		_, _ = s.Store.CreateMemory(ctx, store.Memory{
			Scope: "global", Kind: "decision",
			Title:       "Meeting decision: " + closed.Title,
			Body:        in.Decision,
			Tags:        []string{"meeting", closed.Kind},
			SourceAgent: &sess.AgentID, SourceTask: nilIfBlank(sess.TaskID),
			Confidence: 0.8,
		}, emb)
		s.Bus.Publish("meeting:"+closed.ID, "decided", closed)
		s.Bus.Publish("meetings", "closed", closed)
		return text("meeting %s closed; the decision was recorded as a memory so it survives "+
			"the transcript", closed.ID), nil
	})

	type handoffIn struct {
		TaskID   string `json:"task_id,omitempty" jsonschema:"defaults to the task you are working on"`
		RoleSlug string `json:"role_slug" jsonschema:"role that should take it over"`
		Note     string `json:"note" jsonschema:"what you did and what they need to do next"`
		Column   string `json:"column,omitempty" jsonschema:"column to move it to, default ready"`
	}
	addTool(r, "handoff", func(ctx context.Context, in handoffIn) (*mcp.CallToolResult, error) {
		role, err := s.Store.GetRoleBySlug(ctx, in.RoleSlug)
		if err != nil {
			return nil, fmt.Errorf("unknown role %q; call role_list for the valid slugs", in.RoleSlug)
		}
		if strings.TrimSpace(in.Note) == "" {
			return text("write a note; a handoff without one makes the next role start from nothing"), nil
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
	addTool(r, "ask_role", func(ctx context.Context, in askIn) (*mcp.CallToolResult, error) {
		return s.askRole(ctx, sess, in.RoleSlug, in.Question)
	})

	type escalateIn struct {
		TaskID   string   `json:"task_id,omitempty" jsonschema:"defaults to the task you are working on"`
		Question string   `json:"question" jsonschema:"one specific question the Founder can answer with a decision"`
		Tried    string   `json:"tried" jsonschema:"which roles you already asked and what they said; required so the Founder is not asked something the company already knows"`
		Options  []string `json:"options,omitempty" jsonschema:"the choices you see, so the Founder can pick rather than compose an answer"`
	}
	addTool(r, "escalate_founder", func(ctx context.Context, in escalateIn) (*mcp.CallToolResult, error) {
		if strings.TrimSpace(in.Question) == "" {
			return text("state one specific question"), nil
		}
		if strings.TrimSpace(in.Tried) == "" {
			return text("say what you already tried. The Founder is the slowest path in the " +
				"company; ask_role the relevant role first and escalate only if that failed."), nil
		}
		id := orStr(in.TaskID, sess.TaskID)
		if id == "" {
			return text("escalation attaches to a task and this run has none"), nil
		}
		data := store.JSONMap{"question": in.Question, "tried": in.Tried, "from": sess.AgentName, "role": sess.RoleSlug}
		if len(in.Options) > 0 {
			data["options"] = in.Options
		}
		if err := s.Store.AddTaskEvent(ctx, id, actor, "escalated", data); err != nil {
			return nil, err
		}

		// Moving to blocked is what makes the wait visible on the board. The
		// column is not guaranteed to exist on a custom board, so its absence
		// must not lose the escalation itself.
		moved := false
		if board, err := s.Store.GetBoard(ctx, sess.BoardID); err == nil && hasColumn(board.Columns, "blocked") {
			col := "blocked"
			if t, err := s.Store.UpdateTask(ctx, id, store.TaskPatch{ColumnKey: &col}, actor); err == nil {
				moved = true
				s.publishBoard(sess.BoardID, "task_moved", t)
			}
		}
		s.Bus.Publish("founder", "question", data)
		s.publishBoard(sess.BoardID, "task_updated", map[string]any{"id": id})

		msg := "question sent to the Founder"
		if moved {
			msg += " and the task moved to blocked"
		}
		return text("%s. Do not wait on an answer inside this run — finish what you can, "+
			"report it with task_update, and the answer will be on the task when work resumes.", msg), nil
	})
}
