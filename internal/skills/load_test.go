package skills

import (
	"context"
	"os"
	"testing"

	"iofi/internal/store"
)

// TestLoadDirWiresEverySkillCleanly is the check that closes the loop between
// the three places tool names live. Parse already rejects a skill naming a tool
// the catalog does not have; this one catches the other half — a skill whose
// procedure calls a tool the role it applies to is not permitted to use. That
// failure is invisible until an agent is halfway through a run and the tool is
// simply absent, with nothing explaining why.
func TestLoadDirWiresEverySkillCleanly(t *testing.T) {
	dsn := os.Getenv("IOFI_TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("IOFI_TEST_DATABASE_URL not set")
	}
	ctx := context.Background()
	st, err := store.Open(ctx, dsn)
	if err != nil {
		t.Fatalf("open: %v", err)
	}
	defer st.Close()
	if err := st.Migrate(ctx); err != nil {
		t.Fatalf("migrate: %v", err)
	}

	loaded, errs := LoadDir(ctx, st, "../../skills")
	for _, e := range errs {
		t.Errorf("skill: %v", e)
	}
	if loaded == 0 {
		t.Fatal("no skills loaded")
	}

	// Every role must end up with procedures. A role with none falls back to its
	// system prompt alone, which is the state this whole mechanism exists to fix.
	roles, err := st.ListRoles(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, role := range roles {
		attached, err := st.SkillsForRole(ctx, role.ID)
		if err != nil {
			t.Fatal(err)
		}
		if len(attached) == 0 {
			t.Errorf("role %q has no skills attached", role.Slug)
		}
	}
}
