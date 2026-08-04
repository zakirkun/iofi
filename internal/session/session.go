// Package session maps a run-scoped bearer token to the agent behind it, so
// every MCP tool call can be attributed and permission-checked.
package session

import (
	"context"
	"crypto/rand"
	"crypto/subtle"
	"encoding/base64"
	"sync"
	"time"

	"iofi/internal/store"
)

// DefaultTTL bounds a token's life even if the run that owns it never reports
// finishing. A token that outlives its run is a standing key to the company.
const DefaultTTL = 3 * time.Hour

type Session struct {
	Token     string
	RunID     string
	AgentID   string
	AgentName string
	RoleID    string
	RoleSlug  string
	TaskID    string
	BoardID   string
	Workspace string
	Policy    store.ToolPolicy
	CreatedAt time.Time
	ExpiresAt time.Time
}

type Registry struct {
	mu   sync.RWMutex
	byID map[string]*Session
	ttl  time.Duration

	// OnRevoke lets the MCP layer drop its per-token server cache.
	OnRevoke func(token string)
}

func NewRegistry() *Registry {
	return &Registry{byID: map[string]*Session{}, ttl: DefaultTTL}
}

func (r *Registry) Create(s Session) *Session {
	s.Token = newToken()
	s.CreatedAt = time.Now()
	s.ExpiresAt = s.CreatedAt.Add(r.ttl)
	r.mu.Lock()
	r.byID[s.Token] = &s
	r.mu.Unlock()
	return &s
}

// Get is constant-time in the comparison and rejects expired tokens.
func (r *Registry) Get(token string) (*Session, bool) {
	if token == "" {
		return nil, false
	}
	r.mu.RLock()
	s, ok := r.byID[token]
	r.mu.RUnlock()
	if !ok || subtle.ConstantTimeCompare([]byte(s.Token), []byte(token)) != 1 {
		return nil, false
	}
	if time.Now().After(s.ExpiresAt) {
		r.Revoke(token)
		return nil, false
	}
	return s, true
}

func (r *Registry) Revoke(token string) {
	r.mu.Lock()
	_, existed := r.byID[token]
	delete(r.byID, token)
	onRevoke := r.OnRevoke
	r.mu.Unlock()
	if existed && onRevoke != nil {
		onRevoke(token)
	}
}

func (r *Registry) Len() int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return len(r.byID)
}

// Janitor drops expired tokens. Without it, a crashed adapter leaks its session
// and the MCP server cache behind it for the process's lifetime.
func (r *Registry) Janitor(ctx context.Context, every time.Duration) {
	t := time.NewTicker(every)
	defer t.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case now := <-t.C:
			r.mu.RLock()
			var stale []string
			for tok, s := range r.byID {
				if now.After(s.ExpiresAt) {
					stale = append(stale, tok)
				}
			}
			r.mu.RUnlock()
			for _, tok := range stale {
				r.Revoke(tok)
			}
		}
	}
}

func newToken() string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		panic(err) // crypto/rand failing means the process cannot be trusted
	}
	return base64.RawURLEncoding.EncodeToString(b)
}
