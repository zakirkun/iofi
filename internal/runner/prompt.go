package runner

import (
	"context"
	"fmt"
	"log/slog"
	"strings"

	"iofi/internal/store"
)

// maxSkillsInPrompt bounds how much procedure enters a brief. Roles now carry
// eight to fourteen skills each, so this is a selection budget rather than a
// cap that rarely binds: the coordinator picks the ones that fit the task and
// the rest stay reachable through skill_list and skill_get.
const maxSkillsInPrompt = 5

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

	// Reference material comes before memory on purpose: an uploaded
	// specification outranks anything an agent concluded on its own, and the
	// order it is read in is the order it is weighted.
	if ref := r.recallKnowledge(ctx, role, task); ref != "" {
		b.WriteString("\n## Reference material the Founder uploaded\n")
		b.WriteString("These passages come from documents the company treats as authoritative. " +
			"Where one contradicts a memory or your own judgement, it wins. " +
			"Call knowledge_search for more of a document and knowledge_get to read it in full.\n\n")
		b.WriteString(ref)
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

You are connected to iofi over MCP as the "iofi" server. Your full tool list
arrives with that connection; these are the ones the company expects you to use
rather than describing what you would do:

- task_update  — post progress and your final summary on this task
- kanban_move  — move this task when its state genuinely changes
- memory_search— check what is already known before you invent something
- memory_write — record a decision, convention or pitfall other agents need
- task_search  — check whether the work already exists before creating more
- role_list    — the exact role slugs; ask_role and handoff reject a guessed one
- ask_role     — ask another role when the brief is ambiguous; do not guess

Never guess an identifier. Role slugs come from role_list, agent names from
agent_roster, column keys from board_get. A guessed one is rejected and the
attempt is wasted.

If the answer is a business call no role owns — a budget, a priority between two
things the Founder wants, an external dependency — use escalate_founder once,
then keep working on what does not depend on it. Do not stall the run waiting.

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

// maxKnowledgeChars caps the uploaded material put in a brief. Reference
// documents are the largest thing that can enter a prompt, and a 300-page
// standard would push the task itself out of the model's attention.
const maxKnowledgeChars = 6000

// recallKnowledge pulls the passages of uploaded documents that bear on this
// task, scoped to what the role is allowed to read.
func (r *Runner) recallKnowledge(ctx context.Context, role *store.Role, task *store.Task) string {
	query := task.Title + "\n" + task.Spec
	emb, err := r.Brain.Embed(ctx, query)
	if err != nil {
		slog.Warn("embedding failed, falling back to keyword recall of documents", "err", err)
	}
	hits, err := r.Store.SearchChunks(ctx, store.KnowledgeQuery{
		Text:      task.Title,
		Embedding: emb,
		Scopes:    []string{"global", "role:" + role.ID, "project:" + task.BoardID},
		Limit:     6,
	})
	if err != nil || len(hits) == 0 {
		return ""
	}
	var b strings.Builder
	for _, h := range hits {
		passage := h.Content
		if b.Len()+len(passage) > maxKnowledgeChars {
			// Cutting a passage mid-sentence is better than dropping the rest
			// silently, as long as the agent is told where to get the whole thing.
			room := maxKnowledgeChars - b.Len()
			if room < 200 {
				fmt.Fprintf(&b, "\n(more in the uploaded documents — call knowledge_search)\n")
				break
			}
			passage = passage[:room] + "… (truncated; knowledge_get doc_id=" + h.DocID + ")"
		}
		fmt.Fprintf(&b, "### %s", h.DocTitle)
		if h.Heading != "" {
			fmt.Fprintf(&b, " › %s", h.Heading)
		}
		fmt.Fprintf(&b, "\n(doc %s)\n%s\n\n", h.DocID, passage)
	}
	return b.String()
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
