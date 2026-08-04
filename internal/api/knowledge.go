package api

import (
	"fmt"
	"io"
	"net/http"
	"strings"

	"github.com/go-chi/chi/v5"

	"iofi/internal/knowledge"
	"iofi/internal/store"
)

// listDocs answers the founder's view, which is every scope. Agents never
// reach this endpoint — they read the knowledge base through MCP, where the
// scope filter is applied from their own role.
func (s *Server) listDocs(w http.ResponseWriter, r *http.Request) {
	var scopes []string
	if scope := r.URL.Query().Get("scope"); scope != "" {
		if ref := r.URL.Query().Get("scope_ref"); ref != "" {
			scope += ":" + ref
		}
		scopes = []string{scope}
	}
	reply(w)(nil2empty(s.Store.ListDocs(r.Context(), scopes)))
}

func (s *Server) getDoc(w http.ResponseWriter, r *http.Request) {
	reply(w)(nil2empty(s.Store.GetDoc(r.Context(), chi.URLParam(r, "id"))))
}

func (s *Server) docChunks(w http.ResponseWriter, r *http.Request) {
	reply(w)(nil2empty(s.Store.ListChunks(r.Context(), chi.URLParam(r, "id"))))
}

// uploadDoc accepts one or more files in a multipart form and returns as soon
// as they are stored. Extraction runs behind the response, so the client must
// watch the document's `state` — or the `knowledge` SSE topic — rather than
// treating a 202 as "ready to search".
func (s *Server) uploadDoc(w http.ResponseWriter, r *http.Request) {
	if s.Knowledge == nil {
		fail(w, http.StatusServiceUnavailable, fmt.Errorf("knowledge base is not configured"))
		return
	}
	// The form is parsed with a small memory budget; anything larger spills to
	// a temp file rather than being held twice in RAM.
	if err := r.ParseMultipartForm(8 << 20); err != nil {
		fail(w, http.StatusBadRequest, fmt.Errorf("expected a multipart upload: %w", err))
		return
	}
	defer r.MultipartForm.RemoveAll()

	files := r.MultipartForm.File["files"]
	if len(files) == 0 {
		files = r.MultipartForm.File["file"] // single-file clients
	}
	if len(files) == 0 {
		fail(w, http.StatusBadRequest, fmt.Errorf(`no files; send them as the "files" field`))
		return
	}

	scope := r.FormValue("scope")
	scopeRef := r.FormValue("scope_ref")
	title := r.FormValue("title")
	var tags []string
	if raw := r.FormValue("tags"); raw != "" {
		for _, t := range strings.Split(raw, ",") {
			if t = strings.TrimSpace(t); t != "" {
				tags = append(tags, t)
			}
		}
	}

	// One bad file in a batch must not discard the good ones, so failures are
	// reported per file and the accepted documents still start extracting.
	accepted := make([]*store.KnowledgeDoc, 0, len(files))
	rejected := map[string]string{}
	for _, fh := range files {
		f, err := fh.Open()
		if err != nil {
			rejected[fh.Filename] = err.Error()
			continue
		}
		data, err := io.ReadAll(io.LimitReader(f, knowledge.MaxUploadBytes+1))
		f.Close()
		if err != nil {
			rejected[fh.Filename] = err.Error()
			continue
		}
		docTitle := title
		if len(files) > 1 {
			docTitle = "" // a shared title across a batch would name them all the same
		}
		doc, err := s.Knowledge.Ingest(r.Context(), knowledge.Upload{
			Filename: fh.Filename,
			MIME:     fh.Header.Get("Content-Type"),
			Data:     data,
			Title:    docTitle,
			Scope:    scope, ScopeRef: scopeRef, Tags: tags, By: "founder",
		})
		if err != nil {
			rejected[fh.Filename] = err.Error()
			continue
		}
		accepted = append(accepted, doc)
	}

	code := http.StatusAccepted
	if len(accepted) == 0 {
		code = http.StatusBadRequest
	}
	writeJSON(w, code, map[string]any{
		"accepted": accepted,
		"rejected": rejected,
		"note":     "extraction runs in the background; poll the document state or watch the knowledge event topic",
	})
}

func (s *Server) reprocessDoc(w http.ResponseWriter, r *http.Request) {
	if s.Knowledge == nil {
		fail(w, http.StatusServiceUnavailable, fmt.Errorf("knowledge base is not configured"))
		return
	}
	if err := s.Knowledge.Reprocess(r.Context(), chi.URLParam(r, "id")); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	writeJSON(w, http.StatusAccepted, map[string]string{"state": "pending"})
}

func (s *Server) deleteDoc(w http.ResponseWriter, r *http.Request) {
	if err := s.Store.DeleteDoc(r.Context(), chi.URLParam(r, "id")); err != nil {
		fail(w, http.StatusInternalServerError, err)
		return
	}
	s.Bus.Publish("knowledge", "deleted", map[string]string{"id": chi.URLParam(r, "id")})
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) searchKnowledge(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Query  string   `json:"query"`
		Scopes []string `json:"scopes"`
		DocID  string   `json:"doc_id"`
		Limit  int      `json:"limit"`
	}
	if err := decode(r, &body); err != nil {
		fail(w, http.StatusBadRequest, err)
		return
	}
	ctx := r.Context()
	emb, _ := s.Brain.Embed(ctx, body.Query)
	reply(w)(nil2empty(s.Store.SearchChunks(ctx, store.KnowledgeQuery{
		Text: body.Query, Embedding: emb, Scopes: body.Scopes,
		DocID: body.DocID, Limit: body.Limit,
	})))
}
