package mcpsrv

import (
	"context"
	"fmt"
	"strings"
	"time"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"iofi/internal/knowledge"
	"iofi/internal/session"
	"iofi/internal/store"
)

func (s *Service) registerKnowledge(r *registrar, sess *session.Session) {
	type memWriteIn struct {
		Title         string   `json:"title" jsonschema:"one line naming the thing learned"`
		Body          string   `json:"body" jsonschema:"the knowledge itself, self-contained"`
		Kind          string   `json:"kind,omitempty" jsonschema:"fact, decision, convention, pitfall or artifact"`
		Scope         string   `json:"scope,omitempty" jsonschema:"global (default), role for your role only, or project for this board only"`
		Tags          []string `json:"tags,omitempty"`
		Confidence    float64  `json:"confidence,omitempty" jsonschema:"0 to 1, how settled this is; low confidence ranks lower in search. Default 0.5"`
		ExpiresInDays int      `json:"expires_in_days,omitempty" jsonschema:"drop this memory after N days; use it for anything temporary"`
	}
	addTool(r, "memory_write", func(ctx context.Context, in memWriteIn) (*mcp.CallToolResult, error) {
		scope, ref := "global", ""
		switch in.Scope {
		case "role":
			scope, ref = "role", sess.RoleID
		case "project":
			scope, ref = "project", sess.BoardID
		}
		m := store.Memory{
			Scope: scope, ScopeRef: ref, Kind: orStr(in.Kind, "fact"),
			Title: in.Title, Body: in.Body, Tags: in.Tags,
			SourceAgent: &sess.AgentID, SourceTask: nilIfBlank(sess.TaskID),
			Confidence: clamp01(in.Confidence),
		}
		if in.ExpiresInDays > 0 {
			exp := time.Now().AddDate(0, 0, in.ExpiresInDays)
			m.ExpiresAt = &exp
		}
		emb, _ := s.Brain.Embed(ctx, in.Title+"\n"+in.Body)
		saved, err := s.Store.CreateMemory(ctx, m, emb)
		if err != nil {
			return nil, err
		}
		s.Bus.Publish("memory", "created", saved)
		return text("stored memory %s", saved.ID), nil
	})

	type memSearchIn struct {
		Query string `json:"query" jsonschema:"what you want to know"`
		Kind  string `json:"kind,omitempty"`
		Limit int    `json:"limit,omitempty" jsonschema:"default 8"`
	}
	addTool(r, "memory_search", func(ctx context.Context, in memSearchIn) (*mcp.CallToolResult, error) {
		emb, _ := s.Brain.Embed(ctx, in.Query)
		mems, err := s.Store.SearchMemories(ctx, store.MemoryQuery{
			Text: in.Query, Embedding: emb, Kind: in.Kind,
			Scopes: []string{"global", "role:" + sess.RoleID, "project:" + sess.BoardID},
			Limit:  orInt(in.Limit, 8),
		})
		if err != nil {
			return nil, err
		}
		if len(mems) == 0 {
			return text("nothing recorded on that yet"), nil
		}
		var b strings.Builder
		for _, m := range mems {
			// The id is printed because correcting a memory needs it and an
			// agent that cannot name the wrong record writes a second one.
			fmt.Fprintf(&b, "## [%s] %s\nid: %s\n%s\n\n", m.Kind, m.Title, m.ID, m.Body)
		}
		return text("%s", b.String()), nil
	})

	type memUpdateIn struct {
		MemoryID   string  `json:"memory_id" jsonschema:"id from memory_search"`
		Title      string  `json:"title,omitempty" jsonschema:"replace the title"`
		Body       string  `json:"body,omitempty" jsonschema:"replace the body; state what is true now, not a diff"`
		Kind       string  `json:"kind,omitempty"`
		Confidence float64 `json:"confidence,omitempty" jsonschema:"0 to 1"`
		Reason     string  `json:"reason" jsonschema:"why the earlier version was wrong or is no longer current; required"`
	}
	addTool(r, "memory_update", func(ctx context.Context, in memUpdateIn) (*mcp.CallToolResult, error) {
		if strings.TrimSpace(in.Reason) == "" {
			return text("give a reason; a memory that changes without one cannot be audited"), nil
		}
		old, err := s.Store.GetMemory(ctx, in.MemoryID)
		if err != nil {
			return nil, fmt.Errorf("no memory with id %q", in.MemoryID)
		}
		p := store.MemoryPatch{}
		if in.Title != "" {
			p.Title = &in.Title
		}
		if in.Body != "" {
			p.Body = &in.Body
		}
		if in.Kind != "" {
			p.Kind = &in.Kind
		}
		if c := clamp01(in.Confidence); c > 0 {
			p.Confidence = &c
		}

		// Re-embed only when the text moved; embedding an unchanged memory
		// spends a model call to arrive at the vector already stored.
		var emb []float32
		if p.Title != nil || p.Body != nil {
			title, body := old.Title, old.Body
			if p.Title != nil {
				title = *p.Title
			}
			if p.Body != nil {
				body = *p.Body
			}
			emb, _ = s.Brain.Embed(ctx, title+"\n"+body)
		}
		updated, err := s.Store.UpdateMemory(ctx, in.MemoryID, p, emb)
		if err != nil {
			return nil, err
		}
		// The correction itself is knowledge: someone believed the old version.
		_, _ = s.Store.CreateMemory(ctx, store.Memory{
			Scope: "global", Kind: "fact",
			Title: "Corrected: " + old.Title,
			Body: fmt.Sprintf("Memory %s was revised by %s (%s). Reason: %s\n\nPrevious text:\n%s",
				old.ID, sess.AgentName, sess.RoleSlug, in.Reason, old.Body),
			SourceAgent: &sess.AgentID, SourceTask: nilIfBlank(sess.TaskID),
			Confidence: 0.9,
		}, nil)
		s.Bus.Publish("memory", "updated", updated)
		return text("memory %s revised", updated.ID), nil
	})

	type memForgetIn struct {
		MemoryID string `json:"memory_id" jsonschema:"id from memory_search"`
		Reason   string `json:"reason" jsonschema:"why it no longer holds; required"`
	}
	addTool(r, "memory_forget", func(ctx context.Context, in memForgetIn) (*mcp.CallToolResult, error) {
		if strings.TrimSpace(in.Reason) == "" {
			return text("give a reason; retiring knowledge without one loses why it was dropped"), nil
		}
		old, err := s.Store.GetMemory(ctx, in.MemoryID)
		if err != nil {
			return nil, fmt.Errorf("no memory with id %q", in.MemoryID)
		}
		if _, err := s.Store.ExpireMemory(ctx, in.MemoryID); err != nil {
			return nil, err
		}
		_, _ = s.Store.CreateMemory(ctx, store.Memory{
			Scope: "global", Kind: "fact",
			Title: "Retired: " + old.Title,
			Body: fmt.Sprintf("Memory %s was retired by %s (%s). Reason: %s\n\nIt said:\n%s",
				old.ID, sess.AgentName, sess.RoleSlug, in.Reason, old.Body),
			SourceAgent: &sess.AgentID, SourceTask: nilIfBlank(sess.TaskID),
			Confidence: 0.9,
		}, nil)
		s.Bus.Publish("memory", "updated", old)
		return text("memory %s retired; it will not appear in search again", in.MemoryID), nil
	})

	// ------------------------------------------------------------ uploaded documents

	scopes := knowledge.ScopesFor(sess.RoleID, sess.BoardID)

	type kbSearchIn struct {
		Query string `json:"query" jsonschema:"what you need the document to tell you"`
		DocID string `json:"doc_id,omitempty" jsonschema:"restrict to one document from knowledge_list"`
		Limit int    `json:"limit,omitempty" jsonschema:"default 6"`
	}
	addTool(r, "knowledge_search", func(ctx context.Context, in kbSearchIn) (*mcp.CallToolResult, error) {
		if strings.TrimSpace(in.Query) == "" {
			return text("give a query"), nil
		}
		emb, _ := s.Brain.Embed(ctx, in.Query)
		hits, err := s.Store.SearchChunks(ctx, store.KnowledgeQuery{
			Text: in.Query, Embedding: emb, Scopes: scopes,
			DocID: in.DocID, Limit: orInt(in.Limit, 6),
		})
		if err != nil {
			return nil, err
		}
		if len(hits) == 0 {
			return text("no uploaded document covers that. Call knowledge_list to see what " +
				"exists before concluding the company has nothing on it."), nil
		}
		var b strings.Builder
		for _, h := range hits {
			// The document title and section travel with every passage: an
			// agent quoting a requirement must be able to say where it came from.
			fmt.Fprintf(&b, "## %s", h.DocTitle)
			if h.Heading != "" {
				fmt.Fprintf(&b, " › %s", h.Heading)
			}
			fmt.Fprintf(&b, "\n(doc %s, section %d)\n%s\n\n", h.DocID, h.Idx, h.Content)
		}
		return text("%s", b.String()), nil
	})

	type kbGetIn struct {
		DocID string `json:"doc_id" jsonschema:"document id from knowledge_list or knowledge_search"`
		From  int    `json:"from,omitempty" jsonschema:"first section index, default 0"`
		Count int    `json:"count,omitempty" jsonschema:"how many sections, default 6; the whole document is usually too large to read at once"`
	}
	addTool(r, "knowledge_get", func(ctx context.Context, in kbGetIn) (*mcp.CallToolResult, error) {
		doc, err := s.Store.GetDoc(ctx, in.DocID)
		if err != nil {
			return nil, fmt.Errorf("no document with id %q", in.DocID)
		}
		if !readable(doc, scopes) {
			return text("that document was not uploaded for your role"), nil
		}
		if doc.State != "ready" {
			return text("document %q is %s%s", doc.Title, doc.State, reasonSuffix(doc.Error)), nil
		}
		chunks, err := s.Store.ListChunks(ctx, doc.ID)
		if err != nil {
			return nil, err
		}
		from, count := max(in.From, 0), orInt(in.Count, 6)
		if from >= len(chunks) {
			return text("%q has %d sections; section %d does not exist", doc.Title, len(chunks), from), nil
		}
		end := min(from+count, len(chunks))

		var b strings.Builder
		fmt.Fprintf(&b, "# %s\nsections %d-%d of %d\n\n", doc.Title, from, end-1, len(chunks))
		for _, c := range chunks[from:end] {
			if c.Heading != "" {
				fmt.Fprintf(&b, "## %s\n", c.Heading)
			}
			b.WriteString(c.Content)
			b.WriteString("\n\n")
		}
		if end < len(chunks) {
			fmt.Fprintf(&b, "…%d more sections. Call knowledge_get again with from=%d.",
				len(chunks)-end, end)
		}
		return text("%s", b.String()), nil
	})

	addTool(r, "knowledge_list", func(ctx context.Context, _ struct{}) (*mcp.CallToolResult, error) {
		docs, err := s.Store.ListDocs(ctx, scopes)
		if err != nil {
			return nil, err
		}
		if len(docs) == 0 {
			return text("no reference documents have been uploaded for you"), nil
		}
		type view struct {
			ID       string   `json:"id"`
			Title    string   `json:"title"`
			Scope    string   `json:"scope"`
			Sections int      `json:"sections"`
			Tags     []string `json:"tags,omitempty"`
			State    string   `json:"state,omitempty"`
			Problem  string   `json:"problem,omitempty"`
		}
		out := make([]view, 0, len(docs))
		for _, d := range docs {
			v := view{ID: d.ID, Title: d.Title, Scope: d.Scope, Sections: d.Chunks, Tags: d.Tags}
			// A document still extracting is listed rather than hidden: an agent
			// that cannot see it will assume the company has nothing on the topic.
			if d.State != "ready" {
				v.State, v.Problem = d.State, d.Error
			}
			out = append(out, v)
		}
		return jsonText(out), nil
	})
}

// readable enforces the same scope rule the search query applies, for the tools
// that take a document id directly and would otherwise bypass it.
func readable(d *store.KnowledgeDoc, scopes []string) bool {
	for _, sc := range scopes {
		scope, ref, _ := strings.Cut(sc, ":")
		if d.Scope == scope && (ref == "" || d.ScopeRef == ref) {
			return true
		}
	}
	return false
}

func reasonSuffix(err string) string {
	if err == "" {
		return ""
	}
	return ": " + err
}
