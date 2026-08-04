package store

import (
	"regexp"
	"strings"
	"testing"

	"iofi/internal/toolcat"
)

var (
	allowBlockRe = regexp.MustCompile(`(?s)"allow"\s*:\s*\[(.*?)\]`)
	quotedRe     = regexp.MustCompile(`"([a-z_]+)"`)
)

// TestSeededPoliciesNameRealTools reads the migrations rather than a running
// database, so a typo in a seeded allow list fails on a laptop with no
// Postgres. A role permitted to call a tool that does not exist looks correct
// in the UI and does nothing at run time.
func TestSeededPoliciesNameRealTools(t *testing.T) {
	entries, err := migrationFS.ReadDir("migrations")
	if err != nil {
		t.Fatal(err)
	}
	checked := 0
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".sql") {
			continue
		}
		body, err := migrationFS.ReadFile("migrations/" + e.Name())
		if err != nil {
			t.Fatal(err)
		}
		for _, block := range allowBlockRe.FindAllStringSubmatch(string(body), -1) {
			for _, m := range quotedRe.FindAllStringSubmatch(block[1], -1) {
				checked++
				if !toolcat.Known(m[1]) {
					t.Errorf("%s: tool_policy allows %q, which is not in the catalog",
						e.Name(), m[1])
				}
			}
		}
	}
	if checked == 0 {
		t.Fatal("found no tool_policy allow lists in the migrations; the regex stopped matching")
	}
}
