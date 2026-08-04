package skills

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"iofi/internal/toolcat"
)

const good = `---
slug: demo
version: 2
applies_to: [qa, cto]
priority: 5
---
## Purpose
do a thing

## When To Use
when asked

## Procedure
1. step

## Constraints
none

## Output Contract
text

## Examples
example
`

func TestParseAcceptsCompleteSkill(t *testing.T) {
	sk, err := Parse("demo.md", []byte(good))
	if err != nil {
		t.Fatalf("expected the skill to parse: %v", err)
	}
	if sk.Frontmatter.Slug != "demo" || sk.Frontmatter.Version != 2 {
		t.Fatalf("frontmatter not read: %+v", sk.Frontmatter)
	}
	if len(sk.Frontmatter.AppliesTo) != 2 {
		t.Fatalf("applies_to not read: %+v", sk.Frontmatter.AppliesTo)
	}
	if sk.Sections["Procedure"] != "1. step" {
		t.Fatalf("section body wrong: %q", sk.Sections["Procedure"])
	}
	for _, want := range RequiredSections {
		if _, ok := sk.Sections[want]; !ok {
			t.Fatalf("missing parsed section %q", want)
		}
	}
}

func TestParseRejectsUnknownTool(t *testing.T) {
	withTool := strings.Replace(good, "priority: 5",
		"priority: 5\ntools_required: [memory_search, task_teleport]", 1)
	_, err := Parse("demo.md", []byte(withTool))
	if err == nil {
		t.Fatal("expected a skill requiring a nonexistent tool to be rejected")
	}
	if !strings.Contains(err.Error(), "task_teleport") {
		t.Fatalf("the error should name the offending tool, got: %v", err)
	}
}

// TestShippedSkillsAreLoadable parses the real skill files. It is the check
// that keeps the procedures handed to agents in step with the tools that
// actually exist: rename a tool without updating a skill and this fails here
// rather than halfway through a run.
func TestShippedSkillsAreLoadable(t *testing.T) {
	root := filepath.Join("..", "..", "skills")
	entries, err := os.ReadDir(root)
	if err != nil {
		t.Skipf("no skills directory to check: %v", err)
	}
	seen := map[string]string{}
	files := 0
	for _, e := range entries {
		if e.IsDir() || !strings.EqualFold(filepath.Ext(e.Name()), ".md") {
			continue
		}
		files++
		raw, err := os.ReadFile(filepath.Join(root, e.Name()))
		if err != nil {
			t.Fatal(err)
		}
		sk, err := Parse(e.Name(), raw)
		if err != nil {
			t.Errorf("%s: %v", e.Name(), err)
			continue
		}
		if prev, dup := seen[sk.Frontmatter.Slug]; dup {
			t.Errorf("%s and %s share the slug %q; the second silently overwrites the first",
				prev, e.Name(), sk.Frontmatter.Slug)
		}
		seen[sk.Frontmatter.Slug] = e.Name()

		if len(sk.Frontmatter.AppliesTo) == 0 {
			t.Errorf("%s: applies_to is empty, so no role will ever receive this skill", e.Name())
		}
		if sk.Frontmatter.Description == "" {
			t.Errorf("%s: no description, so skill selection has nothing to rank it on", e.Name())
		}
		// Parse already rejects unknown tools; this asserts the list is not
		// simply absent, which would let a skill call anything unchecked.
		for _, tool := range sk.Frontmatter.ToolsRequired {
			if !toolcat.Known(tool) {
				t.Errorf("%s: requires unknown tool %q", e.Name(), tool)
			}
		}
	}
	if files == 0 {
		t.Fatal("the skills directory has no .md files")
	}
}

func TestParseRejectsMissingSection(t *testing.T) {
	// Drop the Examples heading: the loader must refuse it.
	partial := good[:len(good)-len("## Examples\nexample\n")]
	if _, err := Parse("demo.md", []byte(partial)); err == nil {
		t.Fatal("expected a skill missing Examples to be rejected")
	}
}

func TestParseRejectsMissingFrontmatter(t *testing.T) {
	if _, err := Parse("demo.md", []byte("## Purpose\nx\n")); err == nil {
		t.Fatal("expected a skill without frontmatter to be rejected")
	}
}

func TestParseDefaultsSlugToFilename(t *testing.T) {
	body := "---\nversion: 1\n---\n" + good[len("---\nslug: demo\nversion: 2\napplies_to: [qa, cto]\npriority: 5\n---\n"):]
	sk, err := Parse("api-design.md", []byte(body))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if sk.Frontmatter.Slug != "api-design" {
		t.Fatalf("slug should fall back to the filename, got %q", sk.Frontmatter.Slug)
	}
	if sk.Frontmatter.Priority != 100 {
		t.Fatalf("priority should default to 100, got %d", sk.Frontmatter.Priority)
	}
}
