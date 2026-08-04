package brain

import (
	"context"
	"fmt"
	"strings"

	"iofi/internal/store"
)

// Triage is the coordinator's reading of a raw request: which role owns it,
// how big it is, and what "done" means.
type Triage struct {
	RoleSlug   string   `json:"role_slug"`
	Reason     string   `json:"reason"`
	Priority   int      `json:"priority"`
	Estimate   float64  `json:"estimate_hours"`
	Acceptance string   `json:"acceptance"`
	Labels     []string `json:"labels"`
	Subtasks   []struct {
		Title      string `json:"title"`
		Spec       string `json:"spec"`
		RoleSlug   string `json:"role_slug"`
		Acceptance string `json:"acceptance"`
	} `json:"subtasks"`
}

const triageSystem = `You route work inside an AI-run software company.
Given a request and the available roles, decide which single role owns it.
Return JSON only, matching this shape:
{"role_slug":"","reason":"","priority":1,"estimate_hours":0,"acceptance":"",
 "labels":[],"subtasks":[{"title":"","spec":"","role_slug":"","acceptance":""}]}
Rules:
- role_slug must be one of the listed slugs. Never invent one.
- priority: 1 highest, 5 lowest.
- acceptance states observable behaviour a QA agent can check without asking.
- Emit subtasks only when the work genuinely spans more than one role or more
  than one focused session. Otherwise return an empty list.`

func (c *Client) Triage(ctx context.Context, title, spec string, roles []store.Role) (*Triage, error) {
	var t Triage
	err := c.ChatJSON(ctx, "", []Message{
		{Role: "system", Content: triageSystem},
		{Role: "user", Content: fmt.Sprintf("ROLES:\n%s\n\nREQUEST TITLE: %s\n\nREQUEST BODY:\n%s",
			roleCatalog(roles), title, spec)},
	}, &t)
	if err != nil {
		return nil, err
	}
	if t.Priority < 1 || t.Priority > 5 {
		t.Priority = 3
	}
	if !slugExists(t.RoleSlug, roles) {
		return nil, fmt.Errorf("triage returned unknown role %q", t.RoleSlug)
	}
	return &t, nil
}

// SelectSkills narrows a role's skill library to the ones this task needs, so
// the agent prompt stays short. Returns slugs.
func (c *Client) SelectSkills(ctx context.Context, task string, skills []store.Skill, max int) ([]string, error) {
	if len(skills) <= max {
		out := make([]string, 0, len(skills))
		for _, s := range skills {
			out = append(out, s.Slug)
		}
		return out, nil
	}
	var res struct {
		Slugs []string `json:"slugs"`
	}
	catalog := make([]string, 0, len(skills))
	for _, s := range skills {
		desc, _ := s.Frontmatter["description"].(string)
		catalog = append(catalog, fmt.Sprintf("- %s: %s", s.Slug, desc))
	}
	err := c.ChatJSON(ctx, "", []Message{
		{Role: "system", Content: fmt.Sprintf(
			`Pick at most %d skills that this task actually needs. Return {"slugs":[]} only.`, max)},
		{Role: "user", Content: "SKILLS:\n" + strings.Join(catalog, "\n") + "\n\nTASK:\n" + task},
	}, &res)
	if err != nil {
		return nil, err
	}
	return res.Slugs, nil
}

// FacilitatorMove is the meeting engine's next step. Exactly one of Speaker,
// Decision or Close is meaningful.
type FacilitatorMove struct {
	Action   string `json:"action"` // ask | decide | close
	Speaker  string `json:"speaker_agent_id"`
	Question string `json:"question"`
	Decision string `json:"decision"`
	Tasks    []struct {
		Title      string `json:"title"`
		Spec       string `json:"spec"`
		RoleSlug   string `json:"role_slug"`
		Acceptance string `json:"acceptance"`
	} `json:"tasks"`
	Memories []struct {
		Title string `json:"title"`
		Body  string `json:"body"`
		Kind  string `json:"kind"`
	} `json:"memories"`
}

const facilitatorSystem = `You facilitate a meeting between AI agents in a software company.
Your only output is JSON:
{"action":"ask|decide|close","speaker_agent_id":"","question":"","decision":"",
 "tasks":[{"title":"","spec":"","role_slug":"","acceptance":""}],
 "memories":[{"title":"","body":"","kind":"decision"}]}
Rules:
- "ask": pick the ONE participant whose input is missing and ask them a specific
  question. Never ask someone who already answered that exact point.
- "decide": the discussion has converged or the round limit is near. State the
  decision, the tasks it produces, and the memories worth keeping.
- "close": nothing further is needed.
- Do not let agents restate agreement. Agreement is a reason to decide, not to
  run another round.`

func (c *Client) Facilitate(ctx context.Context, m *store.Meeting, transcript string, participants []AgentBrief) (*FacilitatorMove, error) {
	var mv FacilitatorMove
	err := c.ChatJSON(ctx, "", []Message{
		{Role: "system", Content: facilitatorSystem},
		{Role: "user", Content: fmt.Sprintf(
			"MEETING: %s (%s)\nAGENDA:\n%s\n\nROUND %d of %d\n\nPARTICIPANTS:\n%s\n\nTRANSCRIPT:\n%s",
			m.Title, m.Kind, m.Agenda, m.Round, m.MaxRounds, briefList(participants), transcript)},
	}, &mv)
	if err != nil {
		return nil, err
	}
	return &mv, nil
}

type AgentBrief struct {
	ID       string
	Name     string
	RoleSlug string
	RoleName string
}

func briefList(a []AgentBrief) string {
	var b strings.Builder
	for _, x := range a {
		fmt.Fprintf(&b, "- %s (id=%s, role=%s)\n", x.Name, x.ID, x.RoleName)
	}
	return b.String()
}

func roleCatalog(roles []store.Role) string {
	var b strings.Builder
	for _, r := range roles {
		fmt.Fprintf(&b, "- %s: %s — %s\n", r.Slug, r.Name, r.Description)
	}
	return b.String()
}

func slugExists(slug string, roles []store.Role) bool {
	for _, r := range roles {
		if r.Slug == slug {
			return true
		}
	}
	return false
}
