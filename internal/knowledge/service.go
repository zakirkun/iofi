package knowledge

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strings"

	"iofi/internal/brain"
	"iofi/internal/bus"
	"iofi/internal/docling"
	"iofi/internal/store"
)

// MaxUploadBytes bounds one document. The limit exists because extraction holds
// the whole file in memory and a founder dragging in a 500MB scan should be
// told so rather than watching the process die.
const MaxUploadBytes = 64 << 20

type Service struct {
	Store   *store.Store
	Brain   *brain.Client
	Docling *docling.Client
	Bus     *bus.Bus
	DataDir string // originals land in <DataDir>/knowledge
}

type Upload struct {
	Filename string
	MIME     string
	Data     []byte
	Title    string
	Scope    string // global | role | project
	ScopeRef string // role id or board id
	Tags     []string
	By       string
}

// Ingest stores the file and returns immediately with the document in `pending`.
//
// Extraction is not done inline: docling on a scanned PDF runs for minutes,
// well past any sane HTTP timeout, and an upload that appears to hang is
// indistinguishable from one that failed. The caller gets a row to watch and
// the work continues behind it.
func (s *Service) Ingest(ctx context.Context, up Upload) (*store.KnowledgeDoc, error) {
	if len(up.Data) == 0 {
		return nil, fmt.Errorf("%s is empty", up.Filename)
	}
	if len(up.Data) > MaxUploadBytes {
		return nil, fmt.Errorf("%s is %d MB; the limit is %d MB",
			up.Filename, len(up.Data)>>20, MaxUploadBytes>>20)
	}
	ext := strings.ToLower(filepath.Ext(up.Filename))
	if !docling.Supported(ext, s.Docling.Enabled()) {
		if s.Docling.Enabled() {
			return nil, fmt.Errorf("%s files are not supported", orExt(ext))
		}
		return nil, fmt.Errorf("%s files need document conversion, which is not configured. "+
			"Set IOFI_DOCLING_URL and start the docling service, or upload markdown or plain text",
			orExt(ext))
	}
	switch up.Scope {
	case "", "global":
		up.Scope, up.ScopeRef = "global", ""
	case "role", "project":
		if up.ScopeRef == "" {
			return nil, fmt.Errorf("scope %q needs the id of the role or board it belongs to", up.Scope)
		}
	default:
		return nil, fmt.Errorf("unknown scope %q; use global, role or project", up.Scope)
	}

	sum := sha256.Sum256(up.Data)
	doc, err := s.Store.CreateDoc(ctx, store.KnowledgeDoc{
		Scope: up.Scope, ScopeRef: up.ScopeRef,
		Title:    orStr(up.Title, strings.TrimSuffix(filepath.Base(up.Filename), ext)),
		Filename: filepath.Base(up.Filename), MIME: up.MIME,
		Bytes: int64(len(up.Data)), SHA256: hex.EncodeToString(sum[:]),
		Tags: up.Tags, UploadedBy: orStr(up.By, "founder"),
	})
	if err != nil {
		return nil, err
	}

	// The original is kept so a document can be re-extracted after docling is
	// configured or upgraded, without asking the founder to upload it again.
	path := filepath.Join(s.DataDir, "knowledge", doc.ID+ext)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return nil, err
	}
	if err := os.WriteFile(path, up.Data, 0o600); err != nil {
		return nil, err
	}
	if _, err := s.Store.Pool.Exec(ctx,
		`UPDATE knowledge_docs SET source_path=$2 WHERE id=$1`, doc.ID, path); err != nil {
		return nil, err
	}
	doc.SourcePath = path

	s.Bus.Publish("knowledge", "uploaded", doc)
	go s.process(context.WithoutCancel(ctx), doc.ID, up.Filename, up.Data)
	return doc, nil
}

// Reprocess re-runs extraction from the stored original. Used after docling is
// configured, so documents that failed for want of a converter recover without
// a second upload.
func (s *Service) Reprocess(ctx context.Context, docID string) error {
	doc, err := s.Store.GetDoc(ctx, docID)
	if err != nil {
		return err
	}
	if doc.SourcePath == "" {
		return fmt.Errorf("document %s has no stored original to re-read", docID)
	}
	data, err := os.ReadFile(doc.SourcePath)
	if err != nil {
		return fmt.Errorf("re-reading %s: %w", doc.Filename, err)
	}
	go s.process(context.WithoutCancel(ctx), doc.ID, doc.Filename, data)
	return nil
}

// process extracts, chunks and embeds. Every failure is written to the document
// row: a founder who uploaded a contract must be able to see that it never
// became searchable, and a log line on a server they do not read is not that.
func (s *Service) process(ctx context.Context, docID, filename string, data []byte) {
	fail := func(err error) {
		slog.Warn("knowledge ingest failed", "doc", docID, "file", filename, "err", err)
		if e := s.Store.SetDocFailed(ctx, docID, err.Error()); e != nil {
			slog.Error("could not record the ingest failure", "doc", docID, "err", e)
		}
		s.Bus.Publish("knowledge", "failed", map[string]string{"id": docID, "error": err.Error()})
	}

	res, err := s.Docling.Extract(ctx, filename, data)
	if err != nil {
		fail(err)
		return
	}
	chunks := Split(res.Markdown)
	if len(chunks) == 0 {
		fail(fmt.Errorf("%s produced no readable text", filename))
		return
	}

	texts := make([]string, len(chunks))
	rows := make([]store.KnowledgeChunk, len(chunks))
	for i, c := range chunks {
		// The heading is embedded with the body: a passage's section title is
		// often the only place the searched-for term appears.
		texts[i] = strings.TrimSpace(c.Heading + "\n" + c.Content)
		rows[i] = store.KnowledgeChunk{Idx: c.Idx, Heading: c.Heading, Content: c.Content}
	}

	// A failed embedding is not a failed ingest. The document is still worth
	// storing: SearchChunks falls back to keyword matching for rows with no
	// vector, which is worse than similarity but far better than nothing.
	vecs, err := s.Brain.EmbedBatch(ctx, texts)
	if err != nil {
		slog.Warn("embedding the document failed; it will be searchable by keyword only",
			"doc", docID, "err", err)
	}
	if err := s.Store.ReplaceChunks(ctx, docID, rows, vecs); err != nil {
		fail(err)
		return
	}
	doc, err := s.Store.SetDocExtracted(ctx, docID, res.Markdown, res.Extractor)
	if err != nil {
		fail(err)
		return
	}
	slog.Info("knowledge document ready", "doc", docID, "title", doc.Title,
		"chunks", len(rows), "extractor", res.Extractor)
	s.Bus.Publish("knowledge", "ready", doc)
}

// ScopesFor is the set an agent of this role, on this board, may read. Used by
// both the MCP tools and the prompt builder so the two never disagree about
// what a role can see.
func ScopesFor(roleID, boardID string) []string {
	scopes := []string{"global"}
	if roleID != "" {
		scopes = append(scopes, "role:"+roleID)
	}
	if boardID != "" {
		scopes = append(scopes, "project:"+boardID)
	}
	return scopes
}

func orStr(v, def string) string {
	if strings.TrimSpace(v) == "" {
		return def
	}
	return v
}

func orExt(ext string) string {
	if ext == "" {
		return "extensionless"
	}
	return ext
}
