package store

import (
	"context"
	"fmt"
	"strconv"
	"strings"
)

const memCols = `id, scope, scope_ref, kind, title, body, tags, source_agent, source_task,
	confidence, created_at, expires_at`

// VecLiteral renders an embedding in pgvector's text form. Casting a text
// parameter with ::vector avoids pulling in a pgvector driver package.
func VecLiteral(v []float32) string {
	if len(v) == 0 {
		return ""
	}
	var b strings.Builder
	b.WriteByte('[')
	for i, f := range v {
		if i > 0 {
			b.WriteByte(',')
		}
		b.WriteString(strconv.FormatFloat(float64(f), 'f', -1, 32))
	}
	b.WriteByte(']')
	return b.String()
}

func (s *Store) CreateMemory(ctx context.Context, m Memory, embedding []float32) (*Memory, error) {
	vec := VecLiteral(embedding)
	return selectOne[Memory](ctx, s, `
		INSERT INTO memories (scope, scope_ref, kind, title, body, tags, source_agent,
			source_task, confidence, embedding, expires_at)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9, NULLIF($10,'')::vector, $11)
		RETURNING `+memCols,
		orDefault(m.Scope, "global"), m.ScopeRef, orDefault(m.Kind, "fact"),
		m.Title, m.Body, nonNil(m.Tags), m.SourceAgent, m.SourceTask,
		orDefaultFloat(m.Confidence, 0.5), vec, m.ExpiresAt)
}

type MemoryQuery struct {
	Text      string // ignored when Embedding is set
	Embedding []float32
	Scopes    []string // "global", "role:<id>", "agent:<id>", "project:<id>"
	Kind      string
	Tags      []string
	Limit     int
}

// SearchMemories ranks by cosine similarity when an embedding is supplied,
// otherwise falls back to trigram-free ILIKE. The fallback exists so memory
// works before an embedding provider is configured.
func (s *Store) SearchMemories(ctx context.Context, q MemoryQuery) ([]Memory, error) {
	var where []string
	var args []any
	add := func(clause string, v any) {
		args = append(args, v)
		where = append(where, fmt.Sprintf(clause, len(args)))
	}
	where = append(where, "(expires_at IS NULL OR expires_at > now())")
	if len(q.Scopes) > 0 {
		var ors []string
		for _, sc := range q.Scopes {
			scope, ref, _ := strings.Cut(sc, ":")
			args = append(args, scope)
			iScope := len(args)
			if ref == "" {
				ors = append(ors, fmt.Sprintf("(scope=$%d)", iScope))
				continue
			}
			args = append(args, ref)
			ors = append(ors, fmt.Sprintf("(scope=$%d AND scope_ref=$%d)", iScope, len(args)))
		}
		where = append(where, "("+strings.Join(ors, " OR ")+")")
	}
	if q.Kind != "" {
		add("kind=$%d", q.Kind)
	}
	if len(q.Tags) > 0 {
		add("tags && $%d", q.Tags)
	}

	limit := q.Limit
	if limit <= 0 {
		limit = 10
	}

	scoreExpr, order := "confidence::float8 AS score", "confidence DESC, created_at DESC"
	if len(q.Embedding) > 0 {
		args = append(args, VecLiteral(q.Embedding))
		i := len(args)
		// Similarity weighted by confidence: a near-match nobody trusts should
		// not outrank a slightly worse match that was recorded as settled.
		scoreExpr = fmt.Sprintf("(1 - (embedding <=> $%d::vector)) * confidence AS score", i)
		order = fmt.Sprintf("(1 - (embedding <=> $%d::vector)) * confidence DESC", i)
		where = append(where, "embedding IS NOT NULL")
	} else if q.Text != "" {
		args = append(args, q.Text)
		i := len(args)
		where = append(where, fmt.Sprintf(
			"(title ILIKE '%%'||$%d||'%%' OR body ILIKE '%%'||$%d||'%%')", i, i))
	}

	args = append(args, limit)
	sql := `SELECT ` + memCols + `, ` + scoreExpr + ` FROM memories WHERE ` +
		strings.Join(where, " AND ") +
		` ORDER BY ` + order + fmt.Sprintf(` LIMIT $%d`, len(args))
	return selectAll[Memory](ctx, s, sql, args...)
}

func (s *Store) GetMemory(ctx context.Context, id string) (*Memory, error) {
	return selectOne[Memory](ctx, s, `SELECT `+memCols+` FROM memories WHERE id=$1`, id)
}

type MemoryPatch struct {
	Title      *string
	Body       *string
	Kind       *string
	Confidence *float32
}

// UpdateMemory rewrites a memory in place and re-embeds it when the text
// changed. Leaving the old embedding behind would keep the memory findable by
// the wording it no longer contains.
func (s *Store) UpdateMemory(ctx context.Context, id string, p MemoryPatch, embedding []float32) (*Memory, error) {
	var sets []string
	args := []any{id}
	set := func(col string, v any) {
		args = append(args, v)
		sets = append(sets, fmt.Sprintf("%s=$%d", col, len(args)))
	}
	if p.Title != nil {
		set("title", *p.Title)
	}
	if p.Body != nil {
		set("body", *p.Body)
	}
	if p.Kind != nil {
		set("kind", *p.Kind)
	}
	if p.Confidence != nil {
		set("confidence", *p.Confidence)
	}
	if len(embedding) > 0 {
		args = append(args, VecLiteral(embedding))
		sets = append(sets, fmt.Sprintf("embedding=$%d::vector", len(args)))
	}
	if len(sets) == 0 {
		return s.GetMemory(ctx, id)
	}
	return selectOne[Memory](ctx, s,
		`UPDATE memories SET `+strings.Join(sets, ", ")+` WHERE id=$1 RETURNING `+memCols, args...)
}

// ExpireMemory retires a memory without deleting it. Search already filters on
// expires_at, so one timestamp removes it from every recall path while the row
// stays available to anyone auditing what the company used to believe.
func (s *Store) ExpireMemory(ctx context.Context, id string) (*Memory, error) {
	return selectOne[Memory](ctx, s,
		`UPDATE memories SET expires_at=now() WHERE id=$1 RETURNING `+memCols, id)
}

func (s *Store) ListMemories(ctx context.Context, limit int) ([]Memory, error) {
	if limit <= 0 {
		limit = 100
	}
	return selectAll[Memory](ctx, s,
		`SELECT `+memCols+` FROM memories ORDER BY created_at DESC LIMIT $1`, limit)
}

func (s *Store) DeleteMemory(ctx context.Context, id string) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM memories WHERE id=$1`, id)
	return err
}

func orDefaultFloat(v, def float32) float32 {
	if v == 0 {
		return def
	}
	return v
}
