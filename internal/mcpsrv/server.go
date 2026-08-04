// Package mcpsrv exposes iofi's control plane to coding agents over MCP.
// This is how an agent runs the company: it moves its own kanban cards, writes
// what it learned, and talks to other roles.
package mcpsrv

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"sync"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"iofi/internal/brain"
	"iofi/internal/bus"
	"iofi/internal/meeting"
	"iofi/internal/runner"
	"iofi/internal/session"
	"iofi/internal/store"
)

// PRService is the slice of GitHub the agent tools need. Kept as an interface
// so mcpsrv does not depend on the whole github package.
type PRService interface {
	OpenPR(ctx context.Context, taskID, title, body string) (url string, err error)
	PRStatus(ctx context.Context, taskID string) (string, error)
}

type Service struct {
	Store    *store.Store
	Brain    *brain.Client
	Bus      *bus.Bus
	Reg      *session.Registry
	Git      *runner.Git
	Meetings *meeting.Engine
	PR       PRService

	mu      sync.Mutex
	servers map[string]*mcp.Server // one server per run token, tools filtered by role
}

func New(st *store.Store, br *brain.Client, b *bus.Bus, reg *session.Registry) *Service {
	return &Service{Store: st, Brain: br, Bus: b, Reg: reg, servers: map[string]*mcp.Server{}}
}

// Handler serves the streamable HTTP MCP endpoint. The bearer token identifies
// the run, which identifies the agent, which decides the tool set.
func (s *Service) Handler() http.Handler {
	inner := mcp.NewStreamableHTTPHandler(func(r *http.Request) *mcp.Server {
		sess, ok := s.Reg.Get(bearer(r))
		if !ok {
			return nil
		}
		return s.serverFor(sess)
	}, nil)

	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if _, ok := s.Reg.Get(bearer(r)); !ok {
			http.Error(w, "unknown or expired run token", http.StatusUnauthorized)
			return
		}
		inner.ServeHTTP(w, r)
	})
}

func (s *Service) serverFor(sess *session.Session) *mcp.Server {
	s.mu.Lock()
	defer s.mu.Unlock()
	if srv, ok := s.servers[sess.Token]; ok {
		return srv
	}
	srv := mcp.NewServer(&mcp.Implementation{Name: "iofi", Version: "0.1.0"}, nil)
	s.register(srv, sess)
	s.servers[sess.Token] = srv
	return srv
}

// Forget drops the cached server when a run ends.
func (s *Service) Forget(token string) {
	s.mu.Lock()
	delete(s.servers, token)
	s.mu.Unlock()
}

func bearer(r *http.Request) string {
	h := r.Header.Get("Authorization")
	if t, ok := strings.CutPrefix(h, "Bearer "); ok {
		return strings.TrimSpace(t)
	}
	if t := r.URL.Query().Get("token"); t != "" {
		return t
	}
	return ""
}

// text builds the plain-text tool result agents read.
func text(format string, args ...any) *mcp.CallToolResult {
	return &mcp.CallToolResult{
		Content: []mcp.Content{&mcp.TextContent{Text: fmt.Sprintf(format, args...)}},
	}
}

func jsonText(v any) *mcp.CallToolResult {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return text("could not encode result: %v", err)
	}
	return text("%s", string(b))
}
