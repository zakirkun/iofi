// Package knowledge turns uploaded documents into passages agents can search.
package knowledge

import (
	"strings"
)

const (
	// TargetChunk is how much text one passage aims to carry. Large enough that
	// a paragraph keeps the sentences around it, small enough that eight of them
	// fit in a brief alongside the task and the memories.
	TargetChunk = 1200
	// MaxChunk is the hard ceiling. A single paragraph longer than this is split
	// rather than stored whole, or one wall of text would crowd out every other
	// result it ranks against.
	MaxChunk = 2200
	// Overlap repeats the tail of the previous passage at the head of the next,
	// so a sentence that answers the query is not cut in half by a boundary and
	// missed by both chunks.
	Overlap = 150
)

// Chunk is one retrievable passage with the heading it sat under.
type Chunk struct {
	Idx     int
	Heading string
	Content string
}

// Split breaks extracted markdown into passages.
//
// The split follows the document's own structure: headings start a new passage
// and are carried on every chunk beneath them, because a paragraph retrieved
// without its section title is frequently unusable — "must not exceed 30 days"
// means nothing when the reader cannot see which clause it belongs to.
func Split(md string) []Chunk {
	var out []Chunk
	heading := ""
	var buf strings.Builder

	flush := func() {
		text := strings.TrimSpace(buf.String())
		buf.Reset()
		if text == "" {
			return
		}
		out = append(out, Chunk{Idx: len(out), Heading: heading, Content: text})
	}

	// carryOver seeds the next passage with the tail of the one just emitted.
	carryOver := func() {
		if len(out) == 0 {
			return
		}
		prev := out[len(out)-1].Content
		if len(prev) <= Overlap {
			return
		}
		tail := prev[len(prev)-Overlap:]
		// Start the overlap at a word boundary; half a word helps no one.
		if i := strings.IndexAny(tail, " \n"); i >= 0 && i < len(tail)-1 {
			tail = tail[i+1:]
		}
		buf.WriteString(strings.TrimSpace(tail))
		buf.WriteString("\n\n")
	}

	for _, block := range blocks(md) {
		if h, ok := headingOf(block); ok {
			flush()
			heading = h
			// The heading itself opens the passage so the text under it reads in
			// context when a chunk is pasted into a prompt.
			buf.WriteString(block)
			buf.WriteString("\n\n")
			continue
		}
		for _, piece := range hardSplit(block, MaxChunk) {
			if buf.Len() > 0 && buf.Len()+len(piece) > TargetChunk {
				flush()
				carryOver()
			}
			buf.WriteString(piece)
			buf.WriteString("\n\n")
		}
	}
	flush()
	return out
}

// blocks splits on blank lines, keeping fenced code together — a fence broken
// across two passages leaves both of them unreadable.
func blocks(md string) []string {
	lines := strings.Split(strings.ReplaceAll(md, "\r\n", "\n"), "\n")
	var out []string
	var cur []string
	inFence := false

	emit := func() {
		if text := strings.TrimSpace(strings.Join(cur, "\n")); text != "" {
			out = append(out, text)
		}
		cur = cur[:0]
	}
	for _, line := range lines {
		if strings.HasPrefix(strings.TrimSpace(line), "```") {
			inFence = !inFence
			cur = append(cur, line)
			if !inFence {
				emit()
			}
			continue
		}
		if !inFence && strings.TrimSpace(line) == "" {
			emit()
			continue
		}
		if !inFence {
			if _, isHeading := headingOf(line); isHeading && len(cur) > 0 {
				emit()
			}
		}
		cur = append(cur, line)
	}
	emit()
	return out
}

func headingOf(block string) (string, bool) {
	line := strings.TrimSpace(block)
	if !strings.HasPrefix(line, "#") {
		return "", false
	}
	// Only the first line matters; a block starting with a heading was already
	// split so the heading stands alone.
	line, _, _ = strings.Cut(line, "\n")
	title := strings.TrimSpace(strings.TrimLeft(line, "#"))
	if title == "" {
		return "", false
	}
	return title, true
}

// hardSplit breaks a block that exceeds max on the last sentence or word
// boundary that fits, so an oversized table or paragraph still yields readable
// pieces instead of being cut mid-word.
func hardSplit(block string, max int) []string {
	if len(block) <= max {
		return []string{block}
	}
	var out []string
	for len(block) > max {
		cut := max
		if i := strings.LastIndexAny(block[:max], ".!?\n"); i > max/2 {
			cut = i + 1
		} else if i := strings.LastIndex(block[:max], " "); i > max/2 {
			cut = i
		}
		out = append(out, strings.TrimSpace(block[:cut]))
		block = strings.TrimSpace(block[cut:])
	}
	if block != "" {
		out = append(out, block)
	}
	return out
}
