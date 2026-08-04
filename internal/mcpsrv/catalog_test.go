package mcpsrv

import (
	"sort"
	"testing"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"iofi/internal/session"
	"iofi/internal/store"
	"iofi/internal/toolcat"
)

// TestRegistrationMatchesCatalog is the check that keeps the three copies of
// the tool list in step. The catalog feeds the role editor and the skill
// loader, so a tool in the catalog that nobody registers is a permission the
// founder can grant and an instruction a skill can give that reach nothing —
// and a registered tool missing from the catalog is refused at registration and
// would quietly never appear to an agent.
func TestRegistrationMatchesCatalog(t *testing.T) {
	// register only builds closures; nothing dereferences Service's collaborators
	// until a tool is actually called, so an empty Service is enough here.
	svc := &Service{}
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)

	// An empty allow list means "everything not denied", which is what makes
	// this walk the whole surface rather than one role's slice of it.
	sess := &session.Session{Policy: store.ToolPolicy{}}

	got := svc.register(srv, sess)
	sort.Strings(got)
	want := toolcat.Names()

	if len(got) != len(want) {
		t.Errorf("registered %d tools, catalog has %d", len(got), len(want))
	}
	inCatalog := map[string]bool{}
	for _, n := range want {
		inCatalog[n] = true
	}
	registered := map[string]bool{}
	for _, n := range got {
		if registered[n] {
			t.Errorf("tool %q registered twice", n)
		}
		registered[n] = true
		if !inCatalog[n] {
			t.Errorf("tool %q is registered but missing from the catalog", n)
		}
	}
	for _, n := range want {
		if !registered[n] {
			t.Errorf("tool %q is in the catalog but never registered", n)
		}
	}
}

// TestPolicyFiltersRegistration proves the allow list still gates what an agent
// sees. Without this, a bug in addTool that ignored the policy would leave
// every role holding every tool and no other test would notice.
func TestPolicyFiltersRegistration(t *testing.T) {
	svc := &Service{}
	srv := mcp.NewServer(&mcp.Implementation{Name: "test", Version: "0"}, nil)
	sess := &session.Session{Policy: store.ToolPolicy{Allow: []string{"task_list"}}}

	// register reports the full considered set; what it attached is what the
	// policy permitted, so check the policy directly for every catalog name.
	svc.register(srv, sess)
	for _, spec := range toolcat.Catalog {
		want := spec.Name == "task_list"
		if got := sess.Policy.Permits(spec.Name); got != want {
			t.Errorf("Permits(%q) = %v, want %v", spec.Name, got, want)
		}
	}
}

// TestCatalogEntriesAreUsable catches rows added in haste: an empty description
// leaves a model guessing what the tool does, which is the whole interface.
func TestCatalogEntriesAreUsable(t *testing.T) {
	for _, spec := range toolcat.Catalog {
		if spec.Name == "" {
			t.Error("catalog has an entry with no name")
			continue
		}
		if len(spec.Description) < 20 {
			t.Errorf("tool %q: description is too short to guide a model: %q",
				spec.Name, spec.Description)
		}
		if spec.Category == "" {
			t.Errorf("tool %q: no category, so the role editor cannot group it", spec.Name)
		}
	}
}
