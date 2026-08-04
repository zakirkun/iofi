package store

import (
	"context"
	"fmt"
	"strings"
)

const docCols = `id, scope, scope_ref, title, filename, mime, bytes, sha256, source_path,
	extractor, state, error, content, tags, uploaded_by, created_at, updated_at`

// docListCols omits content. A list of ten manuals would otherwise ship every
// word of all ten to render a table of names.
const docListCols = `id, scope, scope_ref, title, filename, mime, bytes, sha256, source_path,
	extractor, state, error, '' AS content, tags, uploaded_by, created_at, updated_at,
	(SELECT count(*)::int FROM knowledge_chunks c WHERE c.doc_id = knowledge_docs.id) AS chunks`

// CreateDoc records an upload before its text exists. Re-uploading the same
// bytes into the same scope replaces the row rather than adding a duplicate,
// because search returning the same paragraph twice reads as corroboration.
func (s *Store) CreateDoc(ctx context.Context, d KnowledgeDoc) (*KnowledgeDoc, error) {
	return selectOne[KnowledgeDoc](ctx, s, `
		INSERT INTO knowledge_docs (scope, scope_ref, title, filename, mime, bytes, sha256,
			source_path, uploaded_by, tags, state)
		VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,'pending')
		ON CONFLICT (scope, scope_ref, sha256) WHERE sha256 <> '' DO UPDATE SET
			title=EXCLUDED.title, filename=EXCLUDED.filename, mime=EXCLUDED.mime,
			bytes=EXCLUDED.bytes, source_path=EXCLUDED.source_path,
			uploaded_by=EXCLUDED.uploaded_by, tags=EXCLUDED.tags,
			state='pending', error='', updated_at=now()
		RETURNING `+docCols,
		orDefault(d.Scope, "global"), d.ScopeRef, d.Title, d.Filename, d.MIME, d.Bytes,
		d.SHA256, d.SourcePath, orDefault(d.UploadedBy, "founder"), nonNil(d.Tags))
}

func (s *Store) GetDoc(ctx context.Context, id string) (*KnowledgeDoc, error) {
	return selectOne[KnowledgeDoc](ctx, s, `SELECT `+docCols+` FROM knowledge_docs WHERE id=$1`, id)
}

// ListDocs filters by the scope strings an agent may read, in the same
// "global" / "role:<id>" form SearchMemories takes. An empty list means every
// scope, which is what the founder's own view wants.
func (s *Store) ListDocs(ctx context.Context, scopes []string) ([]KnowledgeDoc, error) {
	sql := `SELECT ` + docListCols + ` FROM knowledge_docs`
	var args []any
	if len(scopes) > 0 {
		var ors []string
		for _, sc := range scopes {
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
		sql += " WHERE " + strings.Join(ors, " OR ")
	}
	return selectAll[KnowledgeDoc](ctx, s, sql+` ORDER BY created_at DESC`, args...)
}

// SetDocExtracted stores the text and marks the document usable.
func (s *Store) SetDocExtracted(ctx context.Context, id, content, extractor string) (*KnowledgeDoc, error) {
	return selectOne[KnowledgeDoc](ctx, s, `
		UPDATE knowledge_docs SET content=$2, extractor=$3, state='ready', error='',
			updated_at=now() WHERE id=$1 RETURNING `+docCols, id, content, extractor)
}

// SetDocFailed records why a document is unusable. The row is kept so the
// founder sees a failed upload instead of one that silently vanished.
func (s *Store) SetDocFailed(ctx context.Context, id, reason string) error {
	_, err := s.Pool.Exec(ctx,
		`UPDATE knowledge_docs SET state='failed', error=$2, updated_at=now() WHERE id=$1`,
		id, reason)
	return err
}

func (s *Store) DeleteDoc(ctx context.Context, id string) error {
	_, err := s.Pool.Exec(ctx, `DELETE FROM knowledge_docs WHERE id=$1`, id)
	return err
}

// ---------------------------------------------------------------- chunks

// ReplaceChunks swaps a document's passages atomically. Re-ingesting by
// deleting first and inserting after would leave the document unsearchable for
// the length of the embedding calls if anything failed in between.
func (s *Store) ReplaceChunks(ctx context.Context, docID string, chunks []KnowledgeChunk, embeddings [][]float32) error {
	tx, err := s.Pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	if _, err := tx.Exec(ctx, `DELETE FROM knowledge_chunks WHERE doc_id=$1`, docID); err != nil {
		return err
	}
	for i, c := range chunks {
		var vec string
		if i < len(embeddings) {
			vec = VecLiteral(embeddings[i])
		}
		if _, err := tx.Exec(ctx, `
			INSERT INTO knowledge_chunks (doc_id, idx, heading, content, embedding)
			VALUES ($1,$2,$3,$4, NULLIF($5,'')::vector)`,
			docID, c.Idx, c.Heading, c.Content, vec); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *Store) ListChunks(ctx context.Context, docID string) ([]KnowledgeChunk, error) {
	return selectAll[KnowledgeChunk](ctx, s,
		`SELECT id, doc_id, idx, heading, content FROM knowledge_chunks
		 WHERE doc_id=$1 ORDER BY idx`, docID)
}

type KnowledgeQuery struct {
	Text      string // used when Embedding is empty
	Embedding []float32
	Scopes    []string // "global", "role:<id>", "project:<id>"
	DocID     string   // restrict to one document
	Limit     int
}

// SearchChunks ranks passages by cosine similarity, falling back to ILIKE so
// the feature works before an embedding provider is configured — the same
// arrangement SearchMemories uses.
//
// Unlike memories there is no confidence weighting: an uploaded document is
// what the Founder said, and the system has no basis for trusting one paragraph
// of it less than another.
func (s *Store) SearchChunks(ctx context.Context, q KnowledgeQuery) ([]KnowledgeChunk, error) {
	where := []string{"d.state = 'ready'"}
	var args []any

	if len(q.Scopes) > 0 {
		var ors []string
		for _, sc := range q.Scopes {
			scope, ref, _ := strings.Cut(sc, ":")
			args = append(args, scope)
			iScope := len(args)
			if ref == "" {
				ors = append(ors, fmt.Sprintf("(d.scope=$%d)", iScope))
				continue
			}
			args = append(args, ref)
			ors = append(ors, fmt.Sprintf("(d.scope=$%d AND d.scope_ref=$%d)", iScope, len(args)))
		}
		where = append(where, "("+strings.Join(ors, " OR ")+")")
	}
	if q.DocID != "" {
		args = append(args, q.DocID)
		where = append(where, fmt.Sprintf("c.doc_id=$%d", len(args)))
	}

	scoreExpr, order := "0::float8 AS score", "c.doc_id, c.idx"
	if len(q.Embedding) > 0 {
		args = append(args, VecLiteral(q.Embedding))
		i := len(args)
		scoreExpr = fmt.Sprintf("(1 - (c.embedding <=> $%d::vector))::float8 AS score", i)
		order = fmt.Sprintf("c.embedding <=> $%d::vector", i)
		where = append(where, "c.embedding IS NOT NULL")
	} else if q.Text != "" {
		args = append(args, q.Text)
		i := len(args)
		where = append(where, fmt.Sprintf(
			"(c.content ILIKE '%%'||$%d||'%%' OR c.heading ILIKE '%%'||$%d||'%%')", i, i))
	}

	limit := q.Limit
	if limit <= 0 {
		limit = 6
	}
	args = append(args, limit)
	return selectAll[KnowledgeChunk](ctx, s, `
		SELECT c.id, c.doc_id, c.idx, c.heading, c.content, `+scoreExpr+`,
		       d.title AS doc_title, d.scope AS doc_scope
		FROM knowledge_chunks c
		JOIN knowledge_docs d ON d.id = c.doc_id
		WHERE `+strings.Join(where, " AND ")+`
		ORDER BY `+order+fmt.Sprintf(` LIMIT $%d`, len(args)), args...)
}
