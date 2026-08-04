package store

import (
	"strconv"
	"testing"
	"time"
)

// TestGrantedPoliciesReachTheNewTools guards the promises the role prompts
// make. Three of them told agents to do things their policy forbade before
// migration 0004, and nothing failed loudly when it happened — the tool simply
// was not there and the agent worked around it.
func TestGrantedPoliciesReachTheNewTools(t *testing.T) {
	s, ctx := open(t)

	roles, err := s.ListRoles(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, want := range []string{"knowledge_search", "knowledge_get", "knowledge_list"} {
		for _, r := range roles {
			if !r.ToolPolicy.Permits(want) {
				t.Errorf("role %q cannot call %q; uploaded reference material is the "+
					"Founder's own instruction and no role should be blind to it", r.Slug, want)
			}
		}
	}

	mustPermit := map[string][]string{
		"tech-leader":     {"task_depend"}, // its prompt says "set depends_on"
		"sprint-manager":  {"task_assign"}, // its prompt says "you may reassign"
		"finance":         {"cost_report"}, // its prompt says to read run cost
		"qa":              {"repo_diff"},   // it verifies work it did not write
		"lead-programmer": {"task_get"},    // it could claim a task but not read it
	}
	for slug, tools := range mustPermit {
		role, err := s.GetRoleBySlug(ctx, slug)
		if err != nil {
			t.Fatalf("seeded role %q missing: %v", slug, err)
		}
		for _, tool := range tools {
			if !role.ToolPolicy.Permits(tool) {
				t.Errorf("role %q is instructed to use %q but its policy denies it", slug, tool)
			}
		}
	}
}

// TestKnowledgeDocLifecycle exercises the upload path's storage half: dedupe,
// chunk replacement, scope filtering and keyword fallback search.
func TestKnowledgeDocLifecycle(t *testing.T) {
	s, ctx := open(t)

	role, err := s.GetRoleBySlug(ctx, "qa")
	if err != nil {
		t.Fatal(err)
	}
	sha := "sha-" + t.Name()
	// The test database is shared and not reset between runs, so every assertion
	// keys off a token no other row can contain. Counting rows globally would
	// make this test pass only on an empty database, which is the one place it
	// is least likely to run.
	tag := "zqx" + strconv.FormatInt(time.Now().UnixNano(), 36)

	doc, err := s.CreateDoc(ctx, KnowledgeDoc{
		Scope: "global", Title: "Payment Terms", Filename: "terms.md", SHA256: sha,
	})
	if err != nil {
		t.Fatalf("CreateDoc: %v", err)
	}
	t.Cleanup(func() { _ = s.DeleteDoc(ctx, doc.ID) })

	// The same bytes in the same scope must replace the row. A second copy makes
	// search return the same paragraph twice, which reads as corroboration.
	again, err := s.CreateDoc(ctx, KnowledgeDoc{
		Scope: "global", Title: "Payment Terms v2", Filename: "terms.md", SHA256: sha,
	})
	if err != nil {
		t.Fatalf("CreateDoc (repeat): %v", err)
	}
	if again.ID != doc.ID {
		t.Fatalf("re-uploading the same bytes created a second document: %s then %s", doc.ID, again.ID)
	}
	if again.Title != "Payment Terms v2" {
		t.Errorf("the replacement kept the old title %q", again.Title)
	}

	if err := s.ReplaceChunks(ctx, doc.ID, []KnowledgeChunk{
		{Idx: 0, Heading: "Late Fees", Content: "A " + tag + " fee of 2% applies per month."},
		{Idx: 1, Heading: "Delivery", Content: "Invoices are due within 30 days."},
	}, nil); err != nil {
		t.Fatalf("ReplaceChunks: %v", err)
	}

	// A document still extracting must not be searchable: half a contract
	// answered as if it were the whole one is worse than no answer.
	if hits, err := s.SearchChunks(ctx, KnowledgeQuery{
		Text: tag, Scopes: []string{"global"},
	}); err != nil {
		t.Fatal(err)
	} else if len(hits) != 0 {
		t.Errorf("a pending document returned %d search hits", len(hits))
	}

	if _, err := s.SetDocExtracted(ctx, doc.ID, "# Payment Terms", "plain"); err != nil {
		t.Fatalf("SetDocExtracted: %v", err)
	}

	hits, err := s.SearchChunks(ctx, KnowledgeQuery{
		Text: tag, Scopes: []string{"global"}, Limit: 5,
	})
	if err != nil {
		t.Fatalf("SearchChunks: %v", err)
	}
	if len(hits) != 1 {
		t.Fatalf("keyword search matched %d chunks, want the one late-fee passage", len(hits))
	}
	if hits[0].DocTitle != "Payment Terms v2" || hits[0].Heading != "Late Fees" {
		t.Errorf("a hit must carry its document and section so it can be cited: %+v", hits[0])
	}

	// Scope isolation: a role-scoped document must not reach another role.
	roleDoc, err := s.CreateDoc(ctx, KnowledgeDoc{
		Scope: "role", ScopeRef: role.ID, Title: "QA Handbook",
		Filename: "qa.md", SHA256: sha + "-role",
	})
	if err != nil {
		t.Fatalf("CreateDoc (role scope): %v", err)
	}
	t.Cleanup(func() { _ = s.DeleteDoc(ctx, roleDoc.ID) })
	if err := s.ReplaceChunks(ctx, roleDoc.ID, []KnowledgeChunk{
		{Idx: 0, Heading: "Regression", Content: "Run the " + tag + " suite before signing off."},
	}, nil); err != nil {
		t.Fatal(err)
	}
	if _, err := s.SetDocExtracted(ctx, roleDoc.ID, "# QA Handbook", "plain"); err != nil {
		t.Fatal(err)
	}

	mine, err := s.SearchChunks(ctx, KnowledgeQuery{
		Text: tag, Scopes: []string{"role:" + role.ID},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(mine) != 1 || mine[0].DocID != roleDoc.ID {
		t.Errorf("QA cannot reach its own handbook: %+v", mine)
	}
	theirs, err := s.SearchChunks(ctx, KnowledgeQuery{
		Text: tag, Scopes: []string{"role:00000000-0000-0000-0000-000000000000"},
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(theirs) != 0 {
		t.Errorf("a role-scoped document leaked to another role: %d hits", len(theirs))
	}

	// ListDocs reports the chunk count without shipping the content.
	docs, err := s.ListDocs(ctx, []string{"role:" + role.ID})
	if err != nil {
		t.Fatal(err)
	}
	var listed *KnowledgeDoc
	for i := range docs {
		if docs[i].ID == roleDoc.ID {
			listed = &docs[i]
		}
	}
	if listed == nil {
		t.Fatalf("ListDocs did not return the role-scoped document: %+v", docs)
	}
	if listed.Chunks != 1 {
		t.Errorf("ListDocs reported %d chunks, want 1", listed.Chunks)
	}
	if listed.Content != "" {
		t.Error("ListDocs returned document content; a list of manuals would ship every word")
	}

	// Deleting the document must take its chunks with it, or search keeps
	// returning passages from a document nobody can open.
	if err := s.DeleteDoc(ctx, roleDoc.ID); err != nil {
		t.Fatal(err)
	}
	if chunks, err := s.ListChunks(ctx, roleDoc.ID); err != nil {
		t.Fatal(err)
	} else if len(chunks) != 0 {
		t.Errorf("%d orphaned chunks survived their document", len(chunks))
	}
}

// TestWouldCycleRejectsOnlyRealCycles: a false positive blocks legitimate
// ordering, a false negative deadlocks every task in the loop for good.
func TestWouldCycleRejectsOnlyRealCycles(t *testing.T) {
	s, ctx := open(t)
	board, err := s.GetBoardBySlug(ctx, "main")
	if err != nil {
		t.Fatal(err)
	}

	mk := func(title string) *Task {
		task, err := s.CreateTask(ctx, Task{BoardID: board.ID, Title: title}, "test")
		if err != nil {
			t.Fatal(err)
		}
		t.Cleanup(func() { _ = s.DeleteTask(ctx, task.ID) })
		return task
	}
	a, b, c := mk("cycle A"), mk("cycle B"), mk("cycle C")

	// B waits on A, C waits on B.
	if _, err := s.UpdateTask(ctx, b.ID, TaskPatch{DependsOn: &[]string{a.ID}}, "test"); err != nil {
		t.Fatal(err)
	}
	if _, err := s.UpdateTask(ctx, c.ID, TaskPatch{DependsOn: &[]string{b.ID}}, "test"); err != nil {
		t.Fatal(err)
	}

	if cyclic, err := s.WouldCycle(ctx, a.ID, []string{c.ID}); err != nil {
		t.Fatal(err)
	} else if !cyclic {
		t.Error("A waiting on C closes the loop A<-B<-C and was not detected")
	}
	if cyclic, err := s.WouldCycle(ctx, c.ID, []string{a.ID}); err != nil {
		t.Fatal(err)
	} else if cyclic {
		t.Error("C already depends on A transitively; adding it directly is not a cycle")
	}
	if cyclic, err := s.WouldCycle(ctx, a.ID, nil); err != nil {
		t.Fatal(err)
	} else if cyclic {
		t.Error("clearing every dependency reported a cycle")
	}
}
