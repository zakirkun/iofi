package runner

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"iofi/internal/store"
)

const maxSkillsInPrompt = 4

// composePrompt builds the brief handed to a coding agent:
// role persona -> selected skills -> shared memory -> the task itself.
func (r *Runner) composePrompt(ctx context.Context, role *store.Role, agent *store.Agent, task *store.Task) (string, error) {
	var b strings.Builder

	b.WriteString(role.SystemPrompt)
	b.WriteString("\n\n## Who you are\n")
	fmt.Fprintf(&b, "You are %q, the %s of this company. The Founder oversees the board.\n",
		agent.Name, role.Name)

	if skills, err := r.selectSkills(ctx, role, task); err != nil {
		slog.Warn("skill selection failed, using none", "role", role.Slug, "err", err)
	} else if len(skills) > 0 {
		b.WriteString("\n## Skills\n")
		b.WriteString("Follow these procedures where they apply.\n")
		for _, s := range skills {
			fmt.Fprintf(&b, "\n### Skill: %s\n%s\n", s.Slug, s.Body)
		}
	}

	if mem := r.recallMemories(ctx, role, task); mem != "" {
		b.WriteString("\n## What the company already knows\n")
		b.WriteString("Prior decisions and conventions. Contradicting one without saying why is a defect.\n\n")
		b.WriteString(mem)
	}

	b.WriteString("\n## Your task\n")
	fmt.Fprintf(&b, "Task #%d — %s\n\n", task.Seq, task.Title)
	if task.Spec != "" {
		b.WriteString(task.Spec + "\n")
	}
	if task.Acceptance != "" {
		b.WriteString("\n### Acceptance criteria\n" + task.Acceptance + "\n")
	}

	b.WriteString(`
## How to report

You are connected to iofi over MCP as the "iofi" server. Use those tools rather
than describing what you would do:

- task_update  — post progress and your final summary on this task
- kanban_move  — move this task when its state genuinely changes
- memory_write — record a decision, convention or pitfall other agents need
- memory_search— check what is already known before you invent something
- ask_role     — ask another role when the brief is ambiguous; do not guess
- meeting_post — contribute to an open discussion

Finish by calling task_update with a summary of what you changed and how you
verified it. If you could not complete the task, say so there and explain what
blocked you — an unreported failure is worse than a reported one.
`)
	fmt.Fprintf(&b, "\nTask id: %s\nBoard id: %s\n", task.ID, task.BoardID)
	return b.String(), nil
}

func (r *Runner) selectSkills(ctx context.Context, role *store.Role, task *store.Task) ([]store.Skill, error) {
	all, err := r.Store.SkillsForRole(ctx, role.ID)
	if err != nil || len(all) == 0 {
		return nil, err
	}
	if len(all) <= maxSkillsInPrompt {
		return all, nil
	}
	slugs, err := r.Brain.SelectSkills(ctx, task.Title+"\n"+task.Spec, all, maxSkillsInPrompt)
	if err != nil {
		return all[:maxSkillsInPrompt], nil // priority order is a fine fallback
	}
	keep := map[string]bool{}
	for _, s := range slugs {
		keep[s] = true
	}
	var out []store.Skill
	for _, s := range all {
		if keep[s.Slug] {
			out = append(out, s)
		}
	}
	return out, nil
}

func (r *Runner) recallMemories(ctx context.Context, role *store.Role, task *store.Task) string {
	query := task.Title + "\n" + task.Spec
	emb, err := r.Brain.Embed(ctx, query)
	if err != nil {
		slog.Warn("embedding failed, falling back to keyword recall", "err", err)
	}
	mems, err := r.Store.SearchMemories(ctx, store.MemoryQuery{
		Text:      task.Title,
		Embedding: emb,
		Scopes:    []string{"global", "role:" + role.ID},
		Limit:     8,
	})
	if err != nil || len(mems) == 0 {
		return ""
	}
	var b strings.Builder
	for _, m := range mems {
		fmt.Fprintf(&b, "- [%s] %s: %s\n", m.Kind, m.Title, oneLine(m.Body, 400))
	}
	return b.String()
}

func oneLine(s string, max int) string {
	s = strings.Join(strings.Fields(s), " ")
	if len(s) > max {
		s = s[:max] + "..."
	}
	return s
}
