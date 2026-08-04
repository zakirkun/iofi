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
	"iofi/internal/toolcat"
)

// registrar carries what every tool registration needs: the server to attach
// to, the policy that decides whether it may be attached at all, and a record
// of what actually got attached. The record is what the catalog test compares
// against, so a tool that exists in the catalog but is never wired up fails the
// build instead of silently never appearing to an agent.
type registrar struct {
	srv    *mcp.Server
	policy store.ToolPolicy
	names  []string
}

// addTool wires one tool. The description comes from the catalog rather than
// the call site: an agent's only guide to a tool is its description, and two
// copies of it drift.
func addTool[In any](r *registrar, name string,
	h func(ctx context.Context, in In) (*mcp.CallToolResult, error)) {

	spec, ok := toolcat.Lookup(name)
	if !ok {
		// A programming error, not a runtime one — but panicking here would
		// take down the server on an agent's first connection rather than at
		// the developer's desk. The catalog test is what actually catches it.
		slog.Error("refusing to register a tool missing from the catalog", "tool", name)
		return
	}
	r.names = append(r.names, name)
	if !r.policy.Permits(name) {
		return
	}
	mcp.AddTool(r.srv, &mcp.Tool{Name: name, Description: spec.Description},
		func(ctx context.Context, _ *mcp.CallToolRequest, in In) (*mcp.CallToolResult, any, error) {
			res, err := h(ctx, in)
			if err != nil {
				return text("failed: %v", err), nil, nil // tool-level failure, not transport failure
			}
			return res, nil, nil
		})
}

// register attaches every tool this session's role is allowed to use and
// returns the full catalog-covered set it considered, permitted or not.
func (s *Service) register(srv *mcp.Server, sess *session.Session) []string {
	r := &registrar{srv: srv, policy: sess.Policy}
	s.registerBoard(r, sess)
	s.registerKnowledge(r, sess)
	s.registerOrg(r, sess)
	s.registerCollab(r, sess)
	s.registerRepo(r, sess)
	return r.names
}

// actorOf names the writer of every event a tool produces. Without it the task
// history reads as if the company acted on its own.
func actorOf(sess *session.Session) string { return "agent:" + sess.AgentID }

// askRole answers as the target role using the coordinator model plus that
// role's system prompt and memories. Spawning the other agent's CLI and waiting
// would deadlock two agents against one capacity slot.
func (s *Service) askRole(ctx context.Context, sess *session.Session, roleSlug, question string) (*mcp.CallToolResult, error) {
	role, err := s.Store.GetRoleBySlug(ctx, roleSlug)
	if err != nil {
		return nil, fmt.Errorf("unknown role %q; call role_list for the valid slugs", roleSlug)
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
		_ = s.Store.AddTaskEvent(ctx, sess.TaskID, actorOf(sess), "commented",
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
			return nil, fmt.Errorf("unknown role %q; call role_list for the valid slugs", roleSlug)
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
			return nil, fmt.Errorf("unknown role %q; call role_list for the valid slugs", slug)
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
	Closed    bool   `json:"closed,omitempty"`
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
			IdleHours: int(time.Since(t.UpdatedAt).Hours()), Closed: t.ClosedAt != nil,
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

// without removes every occurrence, used by the tools that take away a link or
// a label rather than adding one.
func without(list, drop []string) []string {
	gone := make(map[string]bool, len(drop))
	for _, d := range drop {
		gone[d] = true
	}
	out := make([]string, 0, len(list))
	for _, v := range list {
		if !gone[v] {
			out = append(out, v)
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
