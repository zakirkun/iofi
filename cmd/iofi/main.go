package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"syscall"
	"time"

	"iofi/internal/api"
	"iofi/internal/brain"
	"iofi/internal/bus"
	"iofi/internal/config"
	"iofi/internal/github"
	"iofi/internal/mcpsrv"
	"iofi/internal/meeting"
	"iofi/internal/runner"
	"iofi/internal/session"
	"iofi/internal/skills"
	"iofi/internal/store"
	"iofi/web"
)

func main() {
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo})))

	if err := run(); err != nil {
		slog.Error("fatal", "err", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	cfg.WarnInsecure()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	st, err := store.Open(ctx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer st.Close()

	if err := st.Migrate(ctx); err != nil {
		return err
	}
	if err := st.CheckEmbedDim(ctx, cfg.EmbedDim); err != nil {
		return err
	}
	if err := st.SeedPrices(ctx); err != nil {
		return err
	}
	// Nothing can resume a run whose process died with the last iofi.
	if n, err := st.RecoverStaleRuns(ctx); err != nil {
		return err
	} else if n > 0 {
		slog.Warn("failed runs left in flight by a previous process", "runs", n)
	}
	slog.Info("database ready")

	if loaded, errs := skills.LoadDir(ctx, st, "skills"); loaded > 0 || len(errs) > 0 {
		for _, e := range errs {
			slog.Warn("skill", "problem", e)
		}
		slog.Info("skills loaded", "count", loaded)
	}

	b := bus.New()
	br := brain.New(cfg.LiteLLMBaseURL, cfg.LiteLLMKey, cfg.BrainModel, cfg.EmbedModel)
	git := runner.NewGit(cfg.DataDir)
	reg := session.NewRegistry()

	mcpURL := strings.TrimRight(cfg.PublicURL, "/") + "/mcp"
	run := runner.New(st, br, git, b, reg, mcpURL, cfg.MaxConcurrentRuns)
	run.LiteLLMURL, run.LiteLLMKey = cfg.LiteLLMBaseURL, cfg.LiteLLMKey

	mcp := mcpsrv.New(st, br, b, reg)
	mcp.Git = git
	mcp.Meetings = meeting.New(st, br, b)
	// Revoking a run token must also drop the MCP server cached behind it.
	reg.OnRevoke = mcp.Forget

	var sync *github.Sync
	if cfg.GitHubToken != "" {
		sync = github.NewSync(st, github.NewClient(cfg.GitHubToken), b, git, cfg.GitHubWebhookSecret)
		mcp.PR = sync
		go sync.Watch(ctx) // mirror board changes outward as they happen
		slog.Info("github sync enabled")
	} else {
		slog.Warn("IOFI_GITHUB_TOKEN unset: github sync and pr_open disabled")
	}

	go reg.Janitor(ctx, 5*time.Minute)
	go st.Janitor(ctx, time.Hour, time.Duration(cfg.EventRetentionDays)*24*time.Hour)

	srv := &api.Server{
		Store: st, Brain: br, Bus: b, Runner: run,
		Meeting: mcp.Meetings, MCP: mcp, GitHub: sync, WebFS: web.FS(),
		AuthToken: cfg.AuthToken, RateLimitPerMin: cfg.RateLimitPerMin,
		LiteLLMURL: cfg.LiteLLMBaseURL,
	}

	httpSrv := &http.Server{
		Addr:              cfg.Addr,
		Handler:           srv.Routes(),
		ReadHeaderTimeout: 10 * time.Second,
		// No WriteTimeout: SSE streams and long agent runs must not be cut off.
	}

	go func() {
		slog.Info("iofi listening", "addr", cfg.Addr, "mcp", mcpURL, "data", cfg.DataDir)
		if err := httpSrv.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("http server", "err", err)
			stop()
		}
	}()

	<-ctx.Done()
	slog.Info("shutting down")
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	return httpSrv.Shutdown(shutdownCtx)
}
