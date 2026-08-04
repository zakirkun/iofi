package store

import (
	"context"
	"os"
	"testing"
	"time"
)

// Every query here runs against a real PostgreSQL with pgvector. There is
// nothing useful to mock: the things that break are the SQL, the jsonb casts
// and the array types, and a mock reproduces none of them.
func open(t *testing.T) (*Store, context.Context) {
	t.Helper()
	dsn := os.Getenv("IOFI_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("IOFI_TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	s, err := Open(ctx, dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	t.Cleanup(s.Close)
	if err := s.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}
	return s, ctx
}

func TestMigrationsAreIdempotent(t *testing.T) {
	s, ctx := open(t)
	// A second Migrate must be a no-op; a re-run of 0002 would duplicate seeds.
	if err := s.Migrate(ctx); err != nil {
		t.Fatalf("second migrate: %v", err)
	}
	roles, err := s.ListRoles(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(roles) < 10 {
		t.Fatalf("expected the seeded roles, got %d", len(roles))
	}
	if _, err := s.GetRoleBySlug(ctx, "lead-programmer"); err != nil {
		t.Fatalf("seeded role missing: %v", err)
	}
	tools, err := s.ListTools(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if len(tools) < 4 {
		t.Fatalf("expected the seeded tools, got %d", len(tools))
	}
	// Every shipped tool must be runnable as seeded; a row that is enabled but
	// has no command fails at dispatch and looks like an iofi bug.
	for _, tool := range tools {
		if tool.Enabled && tool.Command == "" {
			t.Fatalf("tool %q is enabled with no command", tool.Kind)
		}
		if tool.Enabled && len(tool.Args) == 0 {
			t.Fatalf("tool %q is enabled with no args", tool.Kind)
		}
	}
}

func TestRolePolicyRoundTrip(t *testing.T) {
	s, ctx := open(t)
	r, err := s.UpsertRole(ctx, Role{
		Slug: "test-role", Name: "Test", SystemPrompt: "be useful",
		ToolPolicy: ToolPolicy{Allow: []string{"task_list"}, Deny: []string{"pr_open"}},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.DeleteRole(ctx, r.ID) })

	got, err := s.GetRole(ctx, r.ID)
	if err != nil {
		t.Fatal(err)
	}
	if !got.ToolPolicy.Permits("task_list") || got.ToolPolicy.Permits("pr_open") {
		t.Fatalf("jsonb policy did not survive the round trip: %+v", got.ToolPolicy)
	}
}

func TestTaskLifecycle(t *testing.T) {
	s, ctx := open(t)
	board := seedBoard(t, s, ctx)

	role, err := s.GetRoleBySlug(ctx, "qa")
	if err != nil {
		t.Fatal(err)
	}
	task, err := s.CreateTask(ctx, Task{
		BoardID: board.ID, Title: "check the thing", Spec: "spec",
		RoleRequired: &role.ID, Labels: []string{"a", "b"},
	}, "founder")
	if err != nil {
		t.Fatalf("create: %v", err)
	}
	if task.ColumnKey != "backlog" || task.Priority != 3 {
		t.Fatalf("defaults not applied: %+v", task)
	}
	if len(task.Labels) != 2 {
		t.Fatalf("text[] round trip lost data: %v", task.Labels)
	}

	col := "ready"
	moved, err := s.UpdateTask(ctx, task.ID, TaskPatch{ColumnKey: &col}, "founder")
	if err != nil {
		t.Fatalf("update: %v", err)
	}
	if moved.ColumnKey != "ready" {
		t.Fatalf("column not moved: %q", moved.ColumnKey)
	}

	events, err := s.ListTaskEvents(ctx, task.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(events) < 2 {
		t.Fatalf("expected create and move events, got %d", len(events))
	}

	open, err := s.ListTasks(ctx, TaskFilter{BoardID: board.ID, Open: true, RoleID: role.ID})
	if err != nil {
		t.Fatal(err)
	}
	if len(open) != 1 {
		t.Fatalf("filter by role returned %d", len(open))
	}
}

func TestWIPLimitBlocksMove(t *testing.T) {
	s, ctx := open(t)
	board, err := s.CreateBoard(ctx, Board{
		Slug: "wip-" + stamp(), Name: "wip",
		Columns: Columns{{Key: "backlog", Name: "Backlog"}, {Key: "doing", Name: "Doing", WIP: 1}},
	})
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = s.Pool.Exec(ctx, `DELETE FROM boards WHERE id=$1`, board.ID) })

	doing := "doing"
	for i := range 2 {
		task, err := s.CreateTask(ctx, Task{BoardID: board.ID, Title: "t"}, "founder")
		if err != nil {
			t.Fatal(err)
		}
		_, err = s.UpdateTask(ctx, task.ID, TaskPatch{ColumnKey: &doing}, "founder")
		if i == 0 && err != nil {
			t.Fatalf("first move should fit under the limit: %v", err)
		}
		if i == 1 {
			var wip ErrWIPExceeded
			if err == nil {
				t.Fatal("second move should have been refused by the WIP limit")
			}
			if !asWIP(err, &wip) {
				t.Fatalf("expected ErrWIPExceeded, got %T: %v", err, err)
			}
		}
	}
}

func TestDependenciesBlockDispatch(t *testing.T) {
	s, ctx := open(t)
	board := seedBoard(t, s, ctx)

	dep, err := s.CreateTask(ctx, Task{BoardID: board.ID, Title: "first"}, "founder")
	if err != nil {
		t.Fatal(err)
	}
	child, err := s.CreateTask(ctx, Task{
		BoardID: board.ID, Title: "second", DependsOn: []string{dep.ID},
	}, "founder")
	if err != nil {
		t.Fatalf("uuid[] insert: %v", err)
	}

	unmet, err := s.UnmetDependencies(ctx, child.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(unmet) != 1 {
		t.Fatalf("expected one open prerequisite, got %v", unmet)
	}

	closed := true
	if _, err := s.UpdateTask(ctx, dep.ID, TaskPatch{Close: &closed}, "founder"); err != nil {
		t.Fatal(err)
	}
	unmet, err = s.UnmetDependencies(ctx, child.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(unmet) != 0 {
		t.Fatalf("closing the prerequisite should clear the block, got %v", unmet)
	}
}

func TestClaimTaskIsExclusive(t *testing.T) {
	s, ctx := open(t)
	board := seedBoard(t, s, ctx)
	agent := seedAgent(t, s, ctx, "lead-programmer")

	task, err := s.CreateTask(ctx, Task{BoardID: board.ID, Title: "claim me"}, "founder")
	if err != nil {
		t.Fatal(err)
	}
	if _, err := s.ClaimTask(ctx, task.ID, agent.ID); err != nil {
		t.Fatalf("first claim: %v", err)
	}
	if _, err := s.ClaimTask(ctx, task.ID, agent.ID); err != ErrNotFound {
		t.Fatalf("a second claim must fail, got %v", err)
	}
}

func TestMemorySearchWithAndWithoutEmbedding(t *testing.T) {
	s, ctx := open(t)

	vec := make([]float32, 1536)
	for i := range vec {
		vec[i] = 0.01
	}
	m, err := s.CreateMemory(ctx, Memory{
		Kind: "convention", Title: "use the shared verifier " + stamp(),
		Body: "auth.VerifyToken is the only entry point", Tags: []string{"auth"},
		Confidence: 0.9,
	}, vec)
	if err != nil {
		t.Fatalf("vector insert: %v", err)
	}
	t.Cleanup(func() { _ = s.DeleteMemory(ctx, m.ID) })

	byVec, err := s.SearchMemories(ctx, MemoryQuery{Embedding: vec, Scopes: []string{"global"}, Limit: 5})
	if err != nil {
		t.Fatalf("vector search: %v", err)
	}
	if len(byVec) == 0 {
		t.Fatal("vector search returned nothing for its own embedding")
	}
	if byVec[0].Score <= 0 {
		t.Fatalf("score should be positive, got %v", byVec[0].Score)
	}

	byText, err := s.SearchMemories(ctx, MemoryQuery{Text: "shared verifier", Limit: 5})
	if err != nil {
		t.Fatalf("keyword fallback: %v", err)
	}
	if len(byText) == 0 {
		t.Fatal("keyword fallback found nothing")
	}

	byTag, err := s.SearchMemories(ctx, MemoryQuery{Tags: []string{"auth"}, Limit: 5})
	if err != nil {
		t.Fatalf("tag filter: %v", err)
	}
	if len(byTag) == 0 {
		t.Fatal("tag filter found nothing")
	}
}

func TestExpiredMemoriesAreHidden(t *testing.T) {
	s, ctx := open(t)
	past := time.Now().Add(-time.Hour)
	m, err := s.CreateMemory(ctx, Memory{
		Title: "stale " + stamp(), Body: "temporary", ExpiresAt: &past,
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = s.DeleteMemory(ctx, m.ID) })

	got, err := s.SearchMemories(ctx, MemoryQuery{Text: "temporary", Limit: 20})
	if err != nil {
		t.Fatal(err)
	}
	for _, x := range got {
		if x.ID == m.ID {
			t.Fatal("an expired memory was returned by search")
		}
	}
}

func TestRunAccountingAndRecovery(t *testing.T) {
	s, ctx := open(t)
	agent := seedAgent(t, s, ctx, "devops")

	run, err := s.CreateRun(ctx, Run{AgentID: agent.ID, Prompt: "do it"})
	if err != nil {
		t.Fatal(err)
	}
	if err := s.StartRun(ctx, run.ID, "claude -p"); err != nil {
		t.Fatal(err)
	}
	if err := s.AddRunUsage(ctx, run.ID, 100, 20, 0.5); err != nil {
		t.Fatal(err)
	}
	if err := s.AppendRunEvent(ctx, run.ID, 1, "text", JSONMap{"text": "hello"}); err != nil {
		t.Fatal(err)
	}
	evs, err := s.ListRunEvents(ctx, run.ID, 0)
	if err != nil || len(evs) != 1 {
		t.Fatalf("run events: %v %d", err, len(evs))
	}

	// A run left "running" must be reclaimed, or its agent stays busy forever.
	if _, err := s.RecoverStaleRuns(ctx); err != nil {
		t.Fatal(err)
	}
	got, err := s.GetRun(ctx, run.ID)
	if err != nil {
		t.Fatal(err)
	}
	if got.State != "failed" {
		t.Fatalf("stale run not reclaimed: %s", got.State)
	}
	if got.TokensIn != 100 || got.CostUSD != 0.5 {
		t.Fatalf("usage accounting wrong: %+v", got)
	}

	lines, err := s.CostReport(ctx, 24*time.Hour)
	if err != nil {
		t.Fatalf("cost report: %v", err)
	}
	if len(lines) == 0 {
		t.Fatal("cost report empty")
	}
}

func TestPickAgentRespectsCapacity(t *testing.T) {
	s, ctx := open(t)
	agent := seedAgent(t, s, ctx, "qa")

	role, _ := s.GetRoleBySlug(ctx, "qa")
	if _, err := s.PickAgentForRole(ctx, role.ID); err != nil {
		t.Fatalf("an idle agent should be pickable: %v", err)
	}
	// Fill its single capacity slot.
	if _, err := s.CreateRun(ctx, Run{AgentID: agent.ID}); err != nil {
		t.Fatal(err)
	}
	if _, err := s.PickAgentForRole(ctx, role.ID); err != ErrNotFound {
		t.Fatalf("a full agent must not be picked, got %v", err)
	}
}

func TestMeetingFlow(t *testing.T) {
	s, ctx := open(t)
	agent := seedAgent(t, s, ctx, "cto")

	m, err := s.CreateMeeting(ctx, Meeting{
		Title: "design " + stamp(), Agenda: "what do we build",
		Participants: []string{agent.ID},
	})
	if err != nil {
		t.Fatalf("uuid[] participants: %v", err)
	}
	if _, err := s.PostMessage(ctx, MeetingMessage{
		MeetingID: m.ID, AgentID: &agent.ID, Speaker: "test", Content: "my view",
	}); err != nil {
		t.Fatal(err)
	}
	msgs, err := s.ListMessages(ctx, m.ID, 0)
	if err != nil || len(msgs) != 1 {
		t.Fatalf("messages: %v %d", err, len(msgs))
	}
	if _, err := s.CloseMeeting(ctx, m.ID, "we build it"); err != nil {
		t.Fatal(err)
	}
}

func TestSettingsAndPrices(t *testing.T) {
	s, ctx := open(t)
	if err := s.SetSetting(ctx, "test_key", map[string]int{"a": 1}); err != nil {
		t.Fatal(err)
	}
	var back map[string]int
	if err := s.GetSetting(ctx, "test_key", &back); err != nil || back["a"] != 1 {
		t.Fatalf("settings round trip: %v %v", err, back)
	}
	if err := s.SeedPrices(ctx); err != nil {
		t.Fatal(err)
	}
	if p := s.Prices(ctx); len(p) == 0 {
		t.Fatal("prices were not seeded")
	}
}

func TestEmbedDimMatchesMigration(t *testing.T) {
	s, ctx := open(t)
	if err := s.CheckEmbedDim(ctx, 1536); err != nil {
		t.Fatalf("the shipped migration should be vector(1536): %v", err)
	}
	if err := s.CheckEmbedDim(ctx, 42); err == nil {
		t.Fatal("a mismatched dimension must be reported, not ignored")
	}
}

// ---------------------------------------------------------------- helpers

func stamp() string { return time.Now().Format("150405.000000") }

func seedBoard(t *testing.T, s *Store, ctx context.Context) *Board {
	t.Helper()
	b, err := s.CreateBoard(ctx, Board{
		Slug: "t-" + stamp(), Name: "test board", Columns: DefaultColumns(),
	})
	if err != nil {
		t.Fatalf("create board: %v", err)
	}
	t.Cleanup(func() { _, _ = s.Pool.Exec(ctx, `DELETE FROM boards WHERE id=$1`, b.ID) })
	return b
}

func seedAgent(t *testing.T, s *Store, ctx context.Context, roleSlug string) *Agent {
	t.Helper()
	role, err := s.GetRoleBySlug(ctx, roleSlug)
	if err != nil {
		t.Fatalf("role %s: %v", roleSlug, err)
	}
	tools, err := s.ListTools(ctx)
	if err != nil || len(tools) == 0 {
		t.Fatalf("tools: %v", err)
	}
	a, err := s.CreateAgent(ctx, Agent{
		Name: "test-" + stamp(), RoleID: role.ID, ToolID: tools[0].ID,
	})
	if err != nil {
		t.Fatalf("create agent: %v", err)
	}
	t.Cleanup(func() { _ = s.DeleteAgent(ctx, a.ID) })
	return a
}

func asWIP(err error, dst *ErrWIPExceeded) bool {
	e, ok := err.(ErrWIPExceeded)
	if ok {
		*dst = e
	}
	return ok
}
