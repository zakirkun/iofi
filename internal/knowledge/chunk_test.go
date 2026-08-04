package knowledge

import (
	"strings"
	"testing"
)

func TestSplitKeepsHeadingWithItsText(t *testing.T) {
	md := `# Payment Terms

Invoices are due within 30 days.

## Late Fees

A late fee of 2% applies per month.
`
	chunks := Split(md)
	if len(chunks) == 0 {
		t.Fatal("expected at least one chunk")
	}
	var lateFee *Chunk
	for i := range chunks {
		if strings.Contains(chunks[i].Content, "2%") {
			lateFee = &chunks[i]
		}
	}
	if lateFee == nil {
		t.Fatal("the late fee paragraph was lost")
	}
	// A clause retrieved without its section title is frequently unusable.
	if lateFee.Heading != "Late Fees" {
		t.Errorf("heading = %q, want %q", lateFee.Heading, "Late Fees")
	}
}

func TestSplitPacksToTargetAndOverlaps(t *testing.T) {
	para := strings.Repeat("word ", 120) // ~600 chars
	md := strings.Join([]string{para, para, para, para, para, para}, "\n\n")

	chunks := Split(md)
	if len(chunks) < 2 {
		t.Fatalf("expected the text to split, got %d chunk(s)", len(chunks))
	}
	for i, c := range chunks {
		if len(c.Content) > MaxChunk+Overlap {
			t.Errorf("chunk %d is %d chars, over the %d ceiling", i, len(c.Content), MaxChunk)
		}
	}
	// Overlap is what stops a sentence on a boundary from being missed by both
	// neighbours, so its absence is a silent retrieval bug.
	prevTail := chunks[0].Content[len(chunks[0].Content)-40:]
	if !strings.Contains(chunks[1].Content[:Overlap+40], strings.TrimSpace(prevTail[:20])) {
		t.Error("the second chunk does not repeat any of the first chunk's tail")
	}
}

func TestSplitKeepsCodeFencesWhole(t *testing.T) {
	md := "# Setup\n\nRun this:\n\n```sh\nmake build\nmake test\n```\n\nThen deploy.\n"
	chunks := Split(md)
	joined := ""
	for _, c := range chunks {
		joined += c.Content
	}
	if strings.Count(joined, "```")%2 != 0 {
		t.Error("a code fence was left unclosed by the split")
	}
	for _, c := range chunks {
		if strings.Contains(c.Content, "make build") && !strings.Contains(c.Content, "make test") {
			t.Error("the fence was broken across chunks; both halves are unreadable")
		}
	}
}

func TestSplitHardSplitsAnOversizedParagraph(t *testing.T) {
	// One paragraph with no blank lines, well over the ceiling.
	huge := strings.Repeat("This sentence is a filler sentence. ", 200) // ~7200 chars
	chunks := Split(huge)
	if len(chunks) < 2 {
		t.Fatalf("expected an oversized paragraph to be broken up, got %d chunk(s)", len(chunks))
	}
	for i, c := range chunks {
		if len(c.Content) > MaxChunk+Overlap {
			t.Errorf("chunk %d is %d chars, over the %d ceiling", i, len(c.Content), MaxChunk)
		}
		if strings.HasSuffix(c.Content, "sente") {
			t.Errorf("chunk %d was cut mid-word", i)
		}
	}
}

func TestSplitEmptyInput(t *testing.T) {
	if got := Split("   \n\n  "); len(got) != 0 {
		t.Errorf("blank input should yield no chunks, got %d", len(got))
	}
}

func TestSplitIndexesAreSequential(t *testing.T) {
	md := strings.Repeat("Paragraph text here.\n\n", 300)
	for i, c := range Split(md) {
		if c.Idx != i {
			t.Fatalf("chunk %d carries index %d; knowledge_get pages by this number", i, c.Idx)
		}
	}
}

func TestScopesForBuildsTheReadableSet(t *testing.T) {
	got := ScopesFor("role-1", "board-1")
	want := []string{"global", "role:role-1", "project:board-1"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
	// A run with no board must not produce a "project:" scope matching every
	// document whose scope_ref happens to be empty.
	if got := ScopesFor("", ""); len(got) != 1 || got[0] != "global" {
		t.Errorf("ScopesFor with no role or board = %v, want [global]", got)
	}
}
