package runner

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"iofi/internal/store"
)

func TestRenderSubstitutesTemplateVars(t *testing.T) {
	vars := map[string]string{"Prompt": "do it", "Workspace": `C:\work\a`}
	got, err := render("{{.Workspace}}/.codex/config.toml", vars)
	if err != nil {
		t.Fatal(err)
	}
	if got != `C:\work\a/.codex/config.toml` {
		t.Fatalf("got %q", got)
	}
	// A literal with no template markers must come back untouched.
	if v, _ := render("--output-format", vars); v != "--output-format" {
		t.Fatalf("literal arg was rewritten: %q", v)
	}
}

func TestWriteMCPConfigShapePerTool(t *testing.T) {
	dir := t.TempDir()
	req := RunRequest{RunID: "r1", Workspace: dir, MCPURL: "http://x/mcp", MCPToken: "tok"}

	claude := &SpawnAdapter{Tool: store.AgentTool{Kind: "claude_code", MCPStyle: "flag", MCPTarget: "--mcp-config"}}
	path, err := claude.writeMCPConfig(req, filepath.Join(dir, ".iofi"))
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var cfg map[string]any
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatalf("claude config is not JSON: %v", err)
	}
	if _, ok := cfg["mcpServers"]; !ok {
		t.Fatalf("claude config missing mcpServers: %s", raw)
	}

	codex := &SpawnAdapter{Tool: store.AgentTool{
		Kind: "codex", MCPStyle: "config_file", MCPTarget: "{{.Workspace}}/.codex/config.toml",
	}}
	path, err = codex.writeMCPConfig(req, filepath.Join(dir, ".iofi"))
	if err != nil {
		t.Fatal(err)
	}
	raw, _ = os.ReadFile(path)
	if !strings.Contains(string(raw), "[mcp_servers.iofi]") {
		t.Fatalf("codex config should be TOML: %s", raw)
	}

	oc := &SpawnAdapter{Tool: store.AgentTool{
		Kind: "opencode", MCPStyle: "config_file", MCPTarget: "{{.Workspace}}/opencode.json",
	}}
	path, err = oc.writeMCPConfig(req, filepath.Join(dir, ".iofi"))
	if err != nil {
		t.Fatal(err)
	}
	raw, _ = os.ReadFile(path)
	if err := json.Unmarshal(raw, &cfg); err != nil {
		t.Fatal(err)
	}
	if _, ok := cfg["mcp"]; !ok {
		t.Fatalf("opencode config uses a different key: %s", raw)
	}

	// CommandCode reuses the mcpServers envelope but keys the entry on
	// "transport", and ignores a server that is not explicitly enabled.
	cc := &SpawnAdapter{Tool: store.AgentTool{
		Kind: "command_code", MCPStyle: "config_file", MCPTarget: "{{.Workspace}}/.mcp.json",
	}}
	path, err = cc.writeMCPConfig(req, filepath.Join(dir, ".iofi"))
	if err != nil {
		t.Fatal(err)
	}
	if filepath.Base(path) != ".mcp.json" {
		t.Fatalf("command-code reads .mcp.json from the project root, got %s", path)
	}
	raw, _ = os.ReadFile(path)
	var ccCfg struct {
		Servers map[string]struct {
			Transport string            `json:"transport"`
			Enabled   bool              `json:"enabled"`
			URL       string            `json:"url"`
			Headers   map[string]string `json:"headers"`
		} `json:"mcpServers"`
	}
	if err := json.Unmarshal(raw, &ccCfg); err != nil {
		t.Fatal(err)
	}
	entry, ok := ccCfg.Servers["iofi"]
	if !ok {
		t.Fatalf("command-code config missing the iofi server: %s", raw)
	}
	if entry.Transport != "http" || !entry.Enabled || entry.URL != req.MCPURL {
		t.Fatalf("command-code entry is wrong: %+v", entry)
	}
	if entry.Headers["Authorization"] != "Bearer "+req.MCPToken {
		t.Fatalf("run token not passed to command-code: %+v", entry.Headers)
	}
}

func TestEmitClaudeNormalizesStream(t *testing.T) {
	out := make(chan Event, 8)
	emitClaude(`{"type":"assistant","message":{"content":[{"type":"text","text":"hi"},`+
		`{"type":"tool_use","name":"task_update","input":{"summary":"x"}}],`+
		`"usage":{"input_tokens":10,"output_tokens":3}}}`, out)
	close(out)

	var kinds []string
	var in, o int64
	for e := range out {
		kinds = append(kinds, e.Kind)
		in, o = in+e.TokensIn, o+e.TokensOut
	}
	want := []string{"text", "tool_call", "usage"}
	if strings.Join(kinds, ",") != strings.Join(want, ",") {
		t.Fatalf("got %v want %v", kinds, want)
	}
	if in != 10 || o != 3 {
		t.Fatalf("usage lost: in=%d out=%d", in, o)
	}
}

func TestSafeNameStripsPathCharacters(t *testing.T) {
	if got := safeName("owner/repo:1"); got != "owner-repo-1" {
		t.Fatalf("got %q", got)
	}
}
