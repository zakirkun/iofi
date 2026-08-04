package config

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

type Config struct {
	Addr        string // HTTP listen address
	DatabaseURL string
	DataDir     string // repos + worktrees live here

	LiteLLMBaseURL string
	LiteLLMKey     string
	BrainModel     string // model used for coordination decisions
	EmbedModel     string
	EmbedDim       int // must match the vector(N) column in migration 0001

	GitHubToken         string
	GitHubWebhookSecret string

	// Public base URL agents use to reach the MCP endpoint.
	PublicURL string

	// AuthToken guards every /api route. Generated at boot when unset, because
	// an unauthenticated API can hire agents and read the whole memory.
	AuthToken       string
	AuthGenerated   bool
	RateLimitPerMin int

	MaxConcurrentRuns  int
	EventRetentionDays int
}

func Load() (Config, error) {
	c := Config{
		Addr:               env("IOFI_ADDR", ":8080"),
		DatabaseURL:        env("IOFI_DATABASE_URL", "postgres://iofi:iofi@localhost:5432/iofi?sslmode=disable"),
		DataDir:            env("IOFI_DATA_DIR", "data"),
		LiteLLMBaseURL:     env("IOFI_LITELLM_URL", "http://localhost:4000"),
		LiteLLMKey:         os.Getenv("IOFI_LITELLM_KEY"),
		BrainModel:         env("IOFI_BRAIN_MODEL", "gpt-4o-mini"),
		EmbedModel:         env("IOFI_EMBED_MODEL", "text-embedding-3-small"),
		EmbedDim:           envInt("IOFI_EMBED_DIM", 1536),
		GitHubToken:        os.Getenv("IOFI_GITHUB_TOKEN"),
		PublicURL:          env("IOFI_PUBLIC_URL", "http://localhost:8080"),
		AuthToken:          os.Getenv("IOFI_AUTH_TOKEN"),
		RateLimitPerMin:    envInt("IOFI_RATE_LIMIT_PER_MIN", 600),
		MaxConcurrentRuns:  envInt("IOFI_MAX_RUNS", 4),
		EventRetentionDays: envInt("IOFI_EVENT_RETENTION_DAYS", 30),
	}
	c.GitHubWebhookSecret = os.Getenv("IOFI_GITHUB_WEBHOOK_SECRET")

	if c.AuthToken == "" {
		c.AuthToken = randomToken()
		c.AuthGenerated = true
	}

	abs, err := filepath.Abs(c.DataDir)
	if err != nil {
		return c, fmt.Errorf("data dir: %w", err)
	}
	c.DataDir = abs
	return c, nil
}

// WarnInsecure reports configuration that is safe on a laptop and dangerous
// anywhere else. Agents run unsandboxed on this host by design.
func (c Config) WarnInsecure() {
	if c.AuthGenerated {
		slog.Warn("IOFI_AUTH_TOKEN was not set; generated one for this run — "+
			"set it in the environment to keep sessions across restarts",
			"token", c.AuthToken)
	}
	host, _, _ := strings.Cut(strings.TrimPrefix(c.Addr, "tcp://"), ":")
	if host != "" && host != "localhost" && host != "127.0.0.1" {
		slog.Warn("listening on a non-loopback address: agents execute with full "+
			"filesystem access on this host and there is no sandbox",
			"addr", c.Addr)
	}
	if c.GitHubToken != "" && c.GitHubWebhookSecret == "" {
		slog.Warn("IOFI_GITHUB_WEBHOOK_SECRET is unset: webhook deliveries are " +
			"rejected unless the repository row carries its own secret")
	}
}

func randomToken() string {
	b := make([]byte, 24)
	if _, err := rand.Read(b); err != nil {
		panic(err)
	}
	return base64.RawURLEncoding.EncodeToString(b)
}

func env(k, def string) string {
	if v := os.Getenv(k); v != "" {
		return v
	}
	return def
}

func envInt(k string, def int) int {
	if v, err := strconv.Atoi(os.Getenv(k)); err == nil && v > 0 {
		return v
	}
	return def
}
