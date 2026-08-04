package api

import (
	"io/fs"
	"net/http"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"

	"iofi/internal/brain"
	"iofi/internal/bus"
	"iofi/internal/github"
	"iofi/internal/mcpsrv"
	"iofi/internal/meeting"
	"iofi/internal/runner"
	"iofi/internal/store"
)

type Server struct {
	Store   *store.Store
	Brain   *brain.Client
	Bus     *bus.Bus
	Runner  *runner.Runner
	Meeting *meeting.Engine
	MCP     *mcpsrv.Service
	GitHub  *github.Sync
	WebFS   fs.FS // built frontend, embedded

	AuthToken       string
	RateLimitPerMin int
	LiteLLMURL      string
}

func (s *Server) Routes() http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID, middleware.Recoverer)
	r.Use(middleware.Timeout(60 * time.Second))

	// Agents reach the control plane here.
	r.Mount("/mcp", s.MCP.Handler())

	if s.GitHub != nil {
		r.Post("/webhooks/github", s.GitHub.Webhook)
	}

	r.Route("/api", func(r chi.Router) {
		r.Use(rateLimit(s.RateLimitPerMin))
		r.Use(requireToken(s.AuthToken))

		r.Get("/health", s.health)
		r.Get("/dashboard", s.dashboard)
		r.Get("/events", s.events) // SSE

		r.Route("/roles", func(r chi.Router) {
			r.Get("/", s.listRoles)
			r.Post("/", s.upsertRole)
			r.Get("/{id}", s.getRole)
			r.Put("/{id}", s.upsertRole)
			r.Delete("/{id}", s.deleteRole)
			r.Get("/{id}/skills", s.roleSkills)
			r.Post("/{id}/skills", s.attachSkill)
			r.Delete("/{id}/skills/{skillID}", s.detachSkill)
		})

		r.Route("/skills", func(r chi.Router) {
			r.Get("/", s.listSkills)
			r.Post("/", s.upsertSkill)
			r.Post("/reload", s.reloadSkills)
			r.Delete("/{id}", s.deleteSkill)
		})

		r.Route("/tools", func(r chi.Router) {
			r.Get("/", s.listTools)
			r.Put("/{id}", s.updateTool)
		})

		r.Route("/agents", func(r chi.Router) {
			r.Get("/", s.listAgents)
			r.Post("/", s.createAgent)
			r.Put("/{id}", s.updateAgent)
			r.Delete("/{id}", s.deleteAgent)
		})

		r.Route("/boards", func(r chi.Router) {
			r.Get("/", s.listBoards)
			r.Post("/", s.createBoard)
			r.Put("/{id}", s.updateBoard)
			r.Delete("/{id}", s.deleteBoard)
			r.Get("/{id}/tasks", s.boardTasks)
			r.Get("/{id}/stalled", s.stalledTasks)
		})

		r.Route("/tasks", func(r chi.Router) {
			r.Get("/", s.listTasks)
			r.Post("/", s.createTask)
			r.Post("/intake", s.intake) // founder request -> triaged task tree
			r.Get("/{id}", s.getTask)
			r.Patch("/{id}", s.patchTask)
			r.Delete("/{id}", s.deleteTask)
			r.Post("/{id}/dispatch", s.dispatchTask)
			r.Post("/{id}/push", s.pushTask)
		})

		r.Route("/memories", func(r chi.Router) {
			r.Get("/", s.listMemories)
			r.Post("/", s.createMemory)
			r.Post("/search", s.searchMemories)
			r.Delete("/{id}", s.deleteMemory)
		})

		r.Route("/meetings", func(r chi.Router) {
			r.Get("/", s.listMeetings)
			r.Post("/", s.createMeeting)
			r.Get("/{id}", s.getMeeting)
			r.Get("/{id}/messages", s.meetingMessages)
			r.Post("/{id}/messages", s.postMeetingMessage)
			r.Post("/{id}/run", s.runMeeting)
		})

		r.Route("/runs", func(r chi.Router) {
			r.Get("/", s.listRuns)
			r.Get("/{id}", s.getRun)
			r.Get("/{id}/events", s.runEvents)
			r.Post("/{id}/cancel", s.cancelRun)
		})

		r.Route("/repos", func(r chi.Router) {
			r.Get("/", s.listRepos)
			r.Post("/", s.upsertRepo)
			r.Delete("/{id}", s.deleteRepo)
		})

		r.Get("/workspaces", s.listWorkspaces)
		r.Post("/workspaces/{id}/release", s.releaseWorkspace)

		r.Get("/costs", s.costReport)
		r.Get("/settings/prices", s.getPrices)
		r.Put("/settings/prices", s.setPrices)
	})

	if s.WebFS != nil {
		r.NotFound(s.spa)
	}
	return r
}

// spa serves the embedded frontend, falling back to index.html so client-side
// routes survive a page reload.
func (s *Server) spa(w http.ResponseWriter, r *http.Request) {
	if strings.HasPrefix(r.URL.Path, "/api/") {
		http.NotFound(w, r)
		return
	}
	path := strings.TrimPrefix(r.URL.Path, "/")
	if path == "" {
		path = "index.html"
	}
	if _, err := fs.Stat(s.WebFS, path); err != nil {
		path = "index.html"
	}
	if path == "index.html" {
		w.Header().Set("Cache-Control", "no-store")
	}
	http.ServeFileFS(w, r, s.WebFS, path)
}
