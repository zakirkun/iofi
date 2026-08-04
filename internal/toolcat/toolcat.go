// Package toolcat is the single list of MCP tool names iofi exposes to agents.
//
// The same names appear in three places that used to drift apart: the tool
// registration in mcpsrv, the tool_policy allow-lists on roles, and the
// tools_required frontmatter of every skill. Nothing checked that they agreed,
// so a role could permit a tool that no longer existed and a skill could
// instruct an agent to call one that was never registered.
//
// This package has no dependencies on purpose: mcpsrv, skills and api all
// import it, and it must never import them back.
package toolcat

import "sort"

type Category string

const (
	Board     Category = "board"
	Knowledge Category = "knowledge"
	Org       Category = "org"
	Collab    Category = "collab"
	Repo      Category = "repo"
	Ops       Category = "ops"
)

// Spec describes one tool to the three consumers. Description is what the
// model reads when deciding whether to call it, so it states the behaviour, not
// the signature.
type Spec struct {
	Name        string   `json:"name"`
	Category    Category `json:"category"`
	Description string   `json:"description"`

	// Mutates marks tools that change company state. The role-policy editor
	// warns before granting these; read-only tools are safe to hand out freely.
	Mutates bool `json:"mutates"`
}

// Catalog is the authority. Adding a tool means adding a row here first —
// mcpsrv.register refuses to register a name that is not in this list, and the
// skill loader refuses to load a skill that requires one.
var Catalog = []Spec{
	// ------------------------------------------------------------ board
	{Name: "task_list", Category: Board, Description: "List tasks on your board. " +
		"Each entry carries its assignee and how long it has sat, so stalled work is " +
		"visible without opening every task."},
	{Name: "task_get", Category: Board, Description: "Read the full spec, acceptance " +
		"criteria and event history of a task."},
	{Name: "task_search", Category: Board, Description: "Find tasks by text across " +
		"title, spec and acceptance criteria, including closed ones. Use it to check " +
		"whether the work you are about to create already exists."},
	{Name: "task_claim", Category: Board, Mutates: true, Description: "Claim an " +
		"unassigned task. Fails if another agent claimed it first."},
	{Name: "task_create", Category: Board, Mutates: true, Description: "Create a task " +
		"on the board for a role to pick up."},
	{Name: "task_split", Category: Board, Mutates: true, Description: "Break a task " +
		"into child tasks, each owned by a role."},
	{Name: "task_update", Category: Board, Mutates: true, Description: "Report progress " +
		"on a task. Call this before you finish."},
	{Name: "task_assign", Category: Board, Mutates: true, Description: "Assign a task to " +
		"a named agent, or clear its assignee. Use agent_roster first to see who has " +
		"capacity."},
	{Name: "task_depend", Category: Board, Mutates: true, Description: "Declare that a " +
		"task cannot start until other tasks are done, or remove such a link. The board " +
		"refuses to dispatch work whose prerequisites are still open."},
	{Name: "task_comment", Category: Board, Mutates: true, Description: "Leave a note on " +
		"a task without changing any of its fields. Use it for findings that are not " +
		"progress, such as a question you are waiting on."},
	{Name: "task_label", Category: Board, Mutates: true, Description: "Add or remove " +
		"labels on a task."},
	{Name: "kanban_move", Category: Board, Mutates: true, Description: "Move a task to " +
		"another column."},
	{Name: "board_get", Category: Board, Description: "Read your board's columns, their " +
		"WIP limits and how full each one is. Check this before moving work into a " +
		"column that may be capped."},

	// ------------------------------------------------------------ knowledge
	{Name: "memory_write", Category: Knowledge, Mutates: true, Description: "Record " +
		"something other agents need to know. Use it for decisions, conventions and pitfalls."},
	{Name: "memory_search", Category: Knowledge, Description: "Search what the company " +
		"already knows before deciding or writing new code."},
	{Name: "memory_update", Category: Knowledge, Mutates: true, Description: "Correct a " +
		"memory that turned out to be wrong or has been superseded. Prefer this over " +
		"writing a second memory that contradicts the first."},
	{Name: "memory_forget", Category: Knowledge, Mutates: true, Description: "Retire a " +
		"memory that no longer holds. It stops appearing in search immediately; say why " +
		"in the reason so the record of the change survives."},
	{Name: "knowledge_search", Category: Knowledge, Description: "Search the reference " +
		"documents the Founder uploaded — specifications, standards, contracts, runbooks. " +
		"These are authoritative in a way agent-written memories are not: where the two " +
		"disagree, the document wins."},
	{Name: "knowledge_get", Category: Knowledge, Description: "Read an uploaded reference " +
		"document, or a range of its sections, when a search result was not enough."},
	{Name: "knowledge_list", Category: Knowledge, Description: "List the reference " +
		"documents available to you: the company-wide ones and any uploaded for your role " +
		"or this project."},

	// ------------------------------------------------------------ org
	{Name: "role_list", Category: Org, Description: "List every role in the company with " +
		"its slug and what it owns. Call this before handoff or ask_role rather than " +
		"guessing a slug."},
	{Name: "agent_roster", Category: Org, Description: "Who exists, which role they hold, " +
		"and whether they have capacity right now. Needed before assigning work to a person " +
		"rather than a role."},
	{Name: "skill_list", Category: Org, Description: "List the procedures your role has " +
		"available. Only the most relevant few were included in your brief."},
	{Name: "skill_get", Category: Org, Description: "Read one procedure in full by its slug."},

	// ------------------------------------------------------------ collab
	{Name: "meeting_open", Category: Collab, Mutates: true, Description: "Open a " +
		"discussion with other roles and let the facilitator run it."},
	{Name: "meeting_post", Category: Collab, Mutates: true, Description: "Say something " +
		"in a meeting."},
	{Name: "meeting_read", Category: Collab, Description: "Read a meeting transcript."},
	{Name: "meeting_list", Category: Collab, Description: "List meetings you are part of. " +
		"Check for an open one on your topic before opening a second."},
	{Name: "meeting_close", Category: Collab, Mutates: true, Description: "Close a meeting " +
		"with the decision it reached. A meeting nobody closes stays open forever and its " +
		"decision is never recorded."},
	{Name: "handoff", Category: Collab, Mutates: true, Description: "Pass a task to " +
		"another role with a note."},
	{Name: "ask_role", Category: Collab, Description: "Ask another role a question and get " +
		"an answer now. Use it instead of guessing."},
	{Name: "escalate_founder", Category: Collab, Mutates: true, Description: "Put a " +
		"question to the Founder when no role can answer it — a business call, a budget, " +
		"an external dependency. The task moves to blocked and waits. Use it sparingly: " +
		"the Founder is the slowest path in the company."},

	// ------------------------------------------------------------ repo
	{Name: "repo_status", Category: Repo, Description: "Show git status of your workspace."},
	{Name: "repo_diff", Category: Repo, Description: "Show the diff of your workspace " +
		"against the base branch. Reviewers and QA need this to check work they did not write."},
	{Name: "repo_log", Category: Repo, Description: "List the commits on your branch that " +
		"are not on the base branch."},
	{Name: "pr_open", Category: Repo, Mutates: true, Description: "Push your branch and " +
		"open a pull request for this task."},
	{Name: "pr_status", Category: Repo, Description: "Check the state of the pull request " +
		"opened for this task: open, merged or closed."},

	// ------------------------------------------------------------ ops
	{Name: "cost_report", Category: Ops, Description: "Token and cost totals per agent " +
		"over a window. Figures from tools that do not report money are priced from the " +
		"configured rate table and are estimates."},
	{Name: "run_list", Category: Ops, Description: "Recent agent runs with their state, " +
		"cost and failure reason. Use it to see what the company has actually been doing."},
	{Name: "run_tail", Category: Ops, Description: "Read the last events of a run. Use it " +
		"to find out why a run failed instead of dispatching the same task again."},
}

var byName = func() map[string]Spec {
	m := make(map[string]Spec, len(Catalog))
	for _, s := range Catalog {
		if _, dup := m[s.Name]; dup {
			panic("toolcat: duplicate tool name " + s.Name)
		}
		m[s.Name] = s
	}
	return m
}()

// Lookup returns the spec for a tool name.
func Lookup(name string) (Spec, bool) {
	s, ok := byName[name]
	return s, ok
}

// Known reports whether a name is a real tool. The skill loader uses it to
// reject a procedure that tells an agent to call something that does not exist.
func Known(name string) bool {
	_, ok := byName[name]
	return ok
}

// Names returns every tool name, sorted.
func Names() []string {
	out := make([]string, 0, len(Catalog))
	for _, s := range Catalog {
		out = append(out, s.Name)
	}
	sort.Strings(out)
	return out
}

// ByCategory groups the catalog for the role-policy editor in the web UI.
func ByCategory() map[Category][]Spec {
	out := map[Category][]Spec{}
	for _, s := range Catalog {
		out[s.Category] = append(out[s.Category], s)
	}
	return out
}
