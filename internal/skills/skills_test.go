package skills

import "testing"

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
