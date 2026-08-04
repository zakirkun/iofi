package mcpsrv

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"iofi/internal/session"
	"iofi/internal/store"
)

// registerOrg covers who exists and what the company has been doing — the
// questions an agent must answer before it can hand work to anyone.
func (s *Service) registerOrg(r *registrar, sess *session.Session) {
	addTool(r, "role_list", func(ctx context.Context, _ struct{}) (*mcp.CallToolResult, error) {
		roles, err := s.Store.ListRoles(ctx)
		if err != nil {
			return nil, err
		}
		type roleView struct {
			Slug        string `json:"slug"`
			Name        string `json:"name"`
			Description string `json:"description"`
			Yours       bool   `json:"yours,omitempty"`
		}
		out := make([]roleView, 0, len(roles))
		for _, role := range roles {
			out = append(out, roleView{
				Slug: role.Slug, Name: role.Name, Description: role.Description,
				Yours: role.ID == sess.RoleID,
			})
		}
		return jsonText(out), nil
	})

	addTool(r, "agent_roster", func(ctx context.Context, _ struct{}) (*mcp.CallToolResult, error) {
		lines, err := s.Store.Roster(ctx)
		if err != nil {
			return nil, err
		}
		// "Has capacity" is the only thing a caller actually decides on, and
		// deriving it from three columns is a step every caller would repeat.
		type view struct {
			store.RosterLine
			Free bool `json:"has_capacity"`
			You  bool `json:"you,omitempty"`
		}
		out := make([]view, 0, len(lines))
		for _, l := range lines {
			out = append(out, view{
				RosterLine: l,
				Free:       l.Status != "paused" && l.ActiveRun < l.Capacity,
				You:        l.AgentID == sess.AgentID,
			})
		}
		return jsonText(out), nil
	})

	addTool(r, "skill_list", func(ctx context.Context, _ struct{}) (*mcp.CallToolResult, error) {
		skills, err := s.Store.SkillsForRole(ctx, sess.RoleID)
		if err != nil {
			return nil, err
		}
		if len(skills) == 0 {
			return text("your role has no procedures attached"), nil
		}
		var b strings.Builder
		for _, sk := range skills {
			desc, _ := sk.Frontmatter["description"].(string)
			fmt.Fprintf(&b, "- %s: %s\n", sk.Slug, desc)
		}
		b.WriteString("\nCall skill_get with a slug to read one in full.")
		return text("%s", b.String()), nil
	})

	type skillGetIn struct {
		Slug string `json:"slug" jsonschema:"skill slug from skill_list"`
	}
	addTool(r, "skill_get", func(ctx context.Context, in skillGetIn) (*mcp.CallToolResult, error) {
		sk, err := s.Store.GetSkillBySlug(ctx, in.Slug)
		if err != nil {
			return nil, fmt.Errorf("no skill %q; call skill_list for the ones you have", in.Slug)
		}
		return text("# Skill: %s (v%d)\n\n%s", sk.Slug, sk.Version, sk.Body), nil
	})

	// ------------------------------------------------------------ ops

	type costIn struct {
		Days int `json:"days,omitempty" jsonschema:"window in days, default 7"`
	}
	addTool(r, "cost_report", func(ctx context.Context, in costIn) (*mcp.CallToolResult, error) {
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

	type runListIn struct {
		AgentName string `json:"agent_name,omitempty" jsonschema:"restrict to one agent, exact name from agent_roster"`
		TaskID    string `json:"task_id,omitempty" jsonschema:"restrict to one task"`
		State     string `json:"state,omitempty" jsonschema:"queued, running, done, failed or cancelled"`
		Limit     int    `json:"limit,omitempty" jsonschema:"default 20"`
	}
	addTool(r, "run_list", func(ctx context.Context, in runListIn) (*mcp.CallToolResult, error) {
		f := store.RunFilter{TaskID: in.TaskID, State: in.State, Limit: orInt(in.Limit, 20)}
		if in.AgentName != "" {
			a, err := s.Store.GetAgentByName(ctx, in.AgentName)
			if err != nil {
				return nil, fmt.Errorf("no agent named %q; call agent_roster for the exact names", in.AgentName)
			}
			f.AgentID = a.ID
		}
		runs, err := s.Store.ListRuns(ctx, f)
		if err != nil {
			return nil, err
		}
		if len(runs) == 0 {
			return text("no runs match"), nil
		}
		type runView struct {
			ID       string  `json:"id"`
			Agent    string  `json:"agent"`
			TaskID   string  `json:"task_id,omitempty"`
			State    string  `json:"state"`
			CostUSD  float64 `json:"cost_usd"`
			Tokens   int64   `json:"tokens"`
			Error    string  `json:"error,omitempty"`
			AgeHours int     `json:"age_hours"`
		}
		names := map[string]string{}
		out := make([]runView, 0, len(runs))
		for _, run := range runs {
			if _, ok := names[run.AgentID]; !ok {
				if a, err := s.Store.GetAgent(ctx, run.AgentID); err == nil {
					names[run.AgentID] = a.Name
				}
			}
			v := runView{
				ID: run.ID, Agent: names[run.AgentID], State: run.State,
				CostUSD: run.CostUSD, Tokens: run.TokensIn + run.TokensOut,
				Error: run.Error, AgeHours: int(time.Since(run.CreatedAt).Hours()),
			}
			if run.TaskID != nil {
				v.TaskID = *run.TaskID
			}
			out = append(out, v)
		}
		return jsonText(out), nil
	})

	type runTailIn struct {
		RunID string `json:"run_id" jsonschema:"run id from run_list"`
		Last  int    `json:"last,omitempty" jsonschema:"how many trailing events, default 30"`
	}
	addTool(r, "run_tail", func(ctx context.Context, in runTailIn) (*mcp.CallToolResult, error) {
		run, err := s.Store.GetRun(ctx, in.RunID)
		if err != nil {
			return nil, fmt.Errorf("no run with id %q", in.RunID)
		}
		events, err := s.Store.TailRunEvents(ctx, in.RunID, orInt(in.Last, 30))
		if err != nil {
			return nil, err
		}
		var b strings.Builder
		fmt.Fprintf(&b, "run %s: %s", run.ID, run.State)
		if run.Error != "" {
			fmt.Fprintf(&b, " — %s", run.Error)
		}
		b.WriteString("\n\n")
		for _, e := range events {
			fmt.Fprintf(&b, "[%d] %s: %s\n", e.Seq, e.Kind, oneLine(e.Payload))
		}
		if len(events) == 0 {
			b.WriteString("(no events recorded)")
		}
		return text("%s", b.String()), nil
	})
}

// oneLine flattens an event payload to something readable in a transcript. The
// raw JSON of a tool_result is often a whole file and would bury the run.
func oneLine(p store.JSONMap) string {
	for _, key := range []string{"text", "message", "error", "name", "summary"} {
		if v, ok := p[key]; ok {
			s := strings.Join(strings.Fields(fmt.Sprint(v)), " ")
			if len(s) > 300 {
				s = s[:300] + "…"
			}
			return s
		}
	}
	s := strings.Join(strings.Fields(fmt.Sprint(map[string]any(p))), " ")
	if len(s) > 300 {
		s = s[:300] + "…"
	}
	return s
}
