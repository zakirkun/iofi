// Package meeting runs agent discussions as a turn-based state machine.
// Participants speak through the coordinator model, not through their coding
// CLI: a meeting is talking, not building.
package meeting

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"iofi/internal/brain"
	"iofi/internal/bus"
	"iofi/internal/store"
)

type Engine struct {
	Store *store.Store
	Brain *brain.Client
	Bus   *bus.Bus
}

func New(st *store.Store, br *brain.Client, b *bus.Bus) *Engine {
	return &Engine{Store: st, Brain: br, Bus: b}
}

// Run drives the meeting to a decision. Rounds are capped: a discussion that
// has not converged by max_rounds is decided on what was said, not left open.
func (e *Engine) Run(ctx context.Context, meetingID string) error {
	m, err := e.Store.GetMeeting(ctx, meetingID)
	if err != nil {
		return err
	}
	if m.ClosedAt != nil {
		return fmt.Errorf("meeting already closed")
	}

	briefs, err := e.briefs(ctx, m.Participants)
	if err != nil {
		return err
	}
	if len(briefs) == 0 {
		return fmt.Errorf("meeting has no participants")
	}

	for round := m.Round; round < m.MaxRounds; round++ {
		m.Round = round + 1
		_ = e.Store.SetMeetingState(ctx, m.ID, "round", m.Round)
		e.publish(m.ID, "state", map[string]any{"state": "round", "round": m.Round})

		transcript, err := e.transcript(ctx, m.ID)
		if err != nil {
			return err
		}
		move, err := e.Brain.Facilitate(ctx, m, transcript, briefs)
		if err != nil {
			return err
		}

		switch move.Action {
		case "ask":
			b, ok := findBrief(briefs, move.Speaker)
			if !ok {
				b = briefs[m.Round%len(briefs)] // facilitator named someone unknown; keep the meeting moving
			}
			if err := e.speak(ctx, m, b, move.Question, transcript); err != nil {
				return err
			}
		case "decide":
			return e.decide(ctx, m, move)
		case "close":
			_, err := e.Store.CloseMeeting(ctx, m.ID, move.Decision)
			e.publish(m.ID, "closed", map[string]any{"decision": move.Decision})
			return err
		default:
			return fmt.Errorf("facilitator returned unknown action %q", move.Action)
		}
	}

	// Round limit reached: force a decision on the transcript so far.
	transcript, _ := e.transcript(ctx, m.ID)
	m.Round = m.MaxRounds
	move, err := e.Brain.Facilitate(ctx, m, transcript+
		"\n\n[FACILITATOR NOTE: round limit reached, you must decide now]", briefs)
	if err != nil {
		return err
	}
	return e.decide(ctx, m, move)
}

// speak has one participant answer, in character, using their role prompt and
// the memories relevant to the agenda.
func (e *Engine) speak(ctx context.Context, m *store.Meeting, b brain.AgentBrief, question, transcript string) error {
	role, err := e.Store.GetRoleBySlug(ctx, b.RoleSlug)
	if err != nil {
		return err
	}
	emb, _ := e.Brain.Embed(ctx, m.Agenda+"\n"+question)
	mems, _ := e.Store.SearchMemories(ctx, store.MemoryQuery{
		Text: m.Agenda, Embedding: emb,
		Scopes: []string{"global", "role:" + role.ID}, Limit: 6,
	})
	var known strings.Builder
	for _, mm := range mems {
		fmt.Fprintf(&known, "- [%s] %s: %s\n", mm.Kind, mm.Title, mm.Body)
	}

	content, _, err := e.Brain.Chat(ctx, role.ModelPref, []brain.Message{
		{Role: "system", Content: role.SystemPrompt + `

You are in a meeting. Answer the question put to you in at most six sentences.
State your position and the reason for it. Disagree when you disagree — a
meeting where everyone agrees produced no information. Do not restate what
others already said.`},
		{Role: "user", Content: fmt.Sprintf(
			"MEETING: %s\nAGENDA:\n%s\n\nWHAT YOU KNOW:\n%s\nTRANSCRIPT SO FAR:\n%s\n\nQUESTION TO YOU: %s",
			m.Title, m.Agenda, known.String(), transcript, question)},
	}, false)
	if err != nil {
		return err
	}

	msg, err := e.Store.PostMessage(ctx, store.MeetingMessage{
		MeetingID: m.ID, AgentID: &b.ID, Speaker: b.Name, Round: m.Round, Content: content,
	})
	if err != nil {
		return err
	}
	e.publish(m.ID, "message", msg)
	return nil
}

// decide records the outcome: a decision message, the tasks it produces, and
// the memories the company keeps.
func (e *Engine) decide(ctx context.Context, m *store.Meeting, move *brain.FacilitatorMove) error {
	if _, err := e.Store.PostMessage(ctx, store.MeetingMessage{
		MeetingID: m.ID, Speaker: "facilitator", Round: m.Round,
		Content: "DECISION: " + move.Decision,
	}); err != nil {
		return err
	}

	var created []string
	for _, t := range move.Tasks {
		if m.BoardID == nil {
			break
		}
		task := store.Task{
			BoardID: *m.BoardID, Title: t.Title, Spec: t.Spec,
			Acceptance: t.Acceptance, ColumnKey: "backlog",
		}
		if t.RoleSlug != "" {
			if role, err := e.Store.GetRoleBySlug(ctx, t.RoleSlug); err == nil {
				task.RoleRequired = &role.ID
			}
		}
		nt, err := e.Store.CreateTask(ctx, task, "meeting:"+m.ID)
		if err != nil {
			slog.Warn("meeting task creation failed", "meeting", m.ID, "title", t.Title, "err", err)
			continue
		}
		created = append(created, nt.ID)
		e.Bus.Publish("board:"+*m.BoardID, "task_created", nt)
	}

	for _, mem := range move.Memories {
		emb, _ := e.Brain.Embed(ctx, mem.Title+"\n"+mem.Body)
		if _, err := e.Store.CreateMemory(ctx, store.Memory{
			Scope: "global", Kind: orStr(mem.Kind, "decision"),
			Title: mem.Title, Body: mem.Body, Confidence: 0.8,
			Tags: []string{"meeting"},
		}, emb); err != nil {
			slog.Warn("meeting memory write failed", "meeting", m.ID, "err", err)
		}
	}

	if _, err := e.Store.CloseMeeting(ctx, m.ID, move.Decision); err != nil {
		return err
	}
	e.publish(m.ID, "decided", map[string]any{"decision": move.Decision, "tasks": created})
	e.Bus.Publish("meetings", "decided", map[string]any{"meeting_id": m.ID})
	return nil
}

func (e *Engine) transcript(ctx context.Context, meetingID string) (string, error) {
	msgs, err := e.Store.ListMessages(ctx, meetingID, 0)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	for _, m := range msgs {
		fmt.Fprintf(&b, "%s: %s\n\n", m.Speaker, m.Content)
	}
	if b.Len() == 0 {
		return "(nothing said yet)", nil
	}
	return b.String(), nil
}

func (e *Engine) briefs(ctx context.Context, agentIDs []string) ([]brain.AgentBrief, error) {
	var out []brain.AgentBrief
	for _, id := range agentIDs {
		a, err := e.Store.GetAgent(ctx, id)
		if err != nil {
			continue
		}
		r, err := e.Store.GetRole(ctx, a.RoleID)
		if err != nil {
			continue
		}
		out = append(out, brain.AgentBrief{ID: a.ID, Name: a.Name, RoleSlug: r.Slug, RoleName: r.Name})
	}
	return out, nil
}

func (e *Engine) publish(meetingID, kind string, data any) {
	e.Bus.Publish("meeting:"+meetingID, kind, data)
}

func findBrief(list []brain.AgentBrief, id string) (brain.AgentBrief, bool) {
	for _, b := range list {
		if b.ID == id {
			return b, true
		}
	}
	return brain.AgentBrief{}, false
}

func orStr(v, def string) string {
	if v == "" {
		return def
	}
	return v
}
