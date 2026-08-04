package runner

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"text/template"

	"iofi/internal/store"
)

// Event is the normalized stream every coding tool collapses into, so the UI
// and the runs table never learn a tool-specific output shape.
type Event struct {
	Kind      string         `json:"kind"` // text | tool_call | tool_result | usage | error | done
	Text      string         `json:"text,omitempty"`
	Name      string         `json:"name,omitempty"`
	Payload   map[string]any `json:"payload,omitempty"`
	TokensIn  int64          `json:"tokens_in,omitempty"`
	TokensOut int64          `json:"tokens_out,omitempty"`
	CostUSD   float64        `json:"cost_usd,omitempty"`
	ExitCode  int            `json:"exit_code,omitempty"`
}

type RunRequest struct {
	RunID     string
	Workspace string
	Prompt    string
	Model     string
	MCPURL    string // iofi's MCP endpoint for this run
	MCPToken  string // run-scoped bearer, ties tool calls back to this agent
	Env       map[string]string
}

type Adapter interface {
	Kind() string
	Start(ctx context.Context, req RunRequest) (<-chan Event, string, error) // events, rendered command, error
}

// ---------------------------------------------------------------- generic spawn

// SpawnAdapter runs a CLI described entirely by an agent_tools row. A new
// coding tool is a database row, not new Go code.
type SpawnAdapter struct {
	Tool store.AgentTool
}

func NewAdapter(t store.AgentTool) (Adapter, error) {
	switch t.Transport {
	case "", "spawn":
		return &SpawnAdapter{Tool: t}, nil
	case "mcp_client":
		return &MCPClientAdapter{Tool: t}, nil
	default:
		return nil, fmt.Errorf("tool %q: unknown transport %q", t.Kind, t.Transport)
	}
}

func (a *SpawnAdapter) Kind() string { return a.Tool.Kind }

// Inline prompts above this length are handed over as a file instead — Windows
// caps a command line near 32k and the brief can exceed it.
const maxInlinePrompt = 16000

func (a *SpawnAdapter) Start(ctx context.Context, req RunRequest) (<-chan Event, string, error) {
	iofiDir := filepath.Join(req.Workspace, ".iofi")
	if err := os.MkdirAll(iofiDir, 0o755); err != nil {
		return nil, "", err
	}
	briefPath := filepath.Join(iofiDir, "brief.md")
	if err := os.WriteFile(briefPath, []byte(req.Prompt), 0o644); err != nil {
		return nil, "", err
	}

	mcpConfigPath, err := a.writeMCPConfig(req, iofiDir)
	if err != nil {
		return nil, "", err
	}

	inline := req.Prompt
	if len(inline) > maxInlinePrompt {
		inline = "Your task brief is in .iofi/brief.md relative to your working " +
			"directory. Read it first, then carry it out."
	}

	vars := map[string]string{
		"Prompt":     inline,
		"PromptFile": briefPath,
		"Workspace":  req.Workspace,
		"MCPConfig":  mcpConfigPath,
		"Model":      req.Model,
		"RunID":      req.RunID,
	}

	args := make([]string, 0, len(a.Tool.Args))
	for _, raw := range a.Tool.Args {
		v, err := render(raw, vars)
		if err != nil {
			return nil, "", err
		}
		args = append(args, v)
	}

	cmd := exec.CommandContext(ctx, a.Tool.Command, args...)
	cmd.Dir = req.Workspace
	env := os.Environ()
	for k, raw := range a.Tool.Env {
		s, _ := raw.(string)
		v, err := render(s, vars)
		if err != nil {
			return nil, "", err
		}
		env = append(env, k+"="+v)
	}
	for k, v := range req.Env {
		env = append(env, k+"="+v)
	}
	env = append(env, "IOFI_RUN_ID="+req.RunID, "IOFI_MCP_URL="+req.MCPURL, "IOFI_MCP_TOKEN="+req.MCPToken)
	cmd.Env = env

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, "", err
	}
	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	rendered := a.Tool.Command + " " + strings.Join(args, " ")
	if err := cmd.Start(); err != nil {
		return nil, rendered, err
	}

	out := make(chan Event, 64)
	go func() {
		defer close(out)
		parse(a.Tool.ParseMode, stdout, out)
		err := cmd.Wait()
		code := cmd.ProcessState.ExitCode()
		if err != nil && code != 0 {
			msg := strings.TrimSpace(stderr.String())
			if msg == "" {
				msg = err.Error()
			}
			out <- Event{Kind: "error", Text: truncate(msg, 4000)}
		}
		out <- Event{Kind: "done", ExitCode: code}
	}()
	return out, rendered, nil
}

// writeMCPConfig points the tool at iofi's MCP endpoint. Shape differs per
// tool, so it is keyed by kind with the widely-supported mcpServers form as the
// default.
func (a *SpawnAdapter) writeMCPConfig(req RunRequest, iofiDir string) (string, error) {
	if a.Tool.MCPStyle == "env" || req.MCPURL == "" {
		return "", nil
	}
	vars := map[string]string{"Workspace": req.Workspace, "RunID": req.RunID}
	target, err := render(a.Tool.MCPTarget, vars)
	if err != nil {
		return "", err
	}
	if a.Tool.MCPStyle == "flag" || target == "" || !strings.ContainsAny(target, `/\`) {
		target = filepath.Join(iofiDir, "mcp.json")
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return "", err
	}

	headers := map[string]string{"Authorization": "Bearer " + req.MCPToken}
	var body []byte
	switch {
	case strings.HasSuffix(target, ".toml"):
		body = []byte(fmt.Sprintf(
			"[mcp_servers.iofi]\nurl = %q\nbearer_token = %q\n", req.MCPURL, req.MCPToken))
	case a.Tool.Kind == "opencode":
		body, err = json.MarshalIndent(map[string]any{
			"$schema": "https://opencode.ai/config.json",
			"mcp": map[string]any{
				"iofi": map[string]any{
					"type": "remote", "url": req.MCPURL, "enabled": true, "headers": headers,
				},
			},
		}, "", "  ")
	case a.Tool.Kind == "command_code":
		// Same mcpServers envelope as Claude Code, but the entry is keyed on
		// "transport" rather than "type" and must be enabled explicitly.
		body, err = json.MarshalIndent(map[string]any{
			"mcpServers": map[string]any{
				"iofi": map[string]any{
					"transport": "http", "enabled": true,
					"url": req.MCPURL, "headers": headers,
				},
			},
		}, "", "  ")
	default:
		body, err = json.MarshalIndent(map[string]any{
			"mcpServers": map[string]any{
				"iofi": map[string]any{"type": "http", "url": req.MCPURL, "headers": headers},
			},
		}, "", "  ")
	}
	if err != nil {
		return "", err
	}
	return target, os.WriteFile(target, body, 0o600)
}

func render(tmpl string, vars map[string]string) (string, error) {
	if !strings.Contains(tmpl, "{{") {
		return tmpl, nil
	}
	t, err := template.New("x").Option("missingkey=zero").Parse(tmpl)
	if err != nil {
		return "", err
	}
	var b strings.Builder
	if err := t.Execute(&b, vars); err != nil {
		return "", err
	}
	return b.String(), nil
}

// ---------------------------------------------------------------- output parsing

func parse(mode string, r interface{ Read([]byte) (int, error) }, out chan<- Event) {
	sc := bufio.NewScanner(r)
	sc.Buffer(make([]byte, 0, 64*1024), 16*1024*1024)
	for sc.Scan() {
		line := sc.Text()
		if strings.TrimSpace(line) == "" {
			continue
		}
		switch mode {
		case "claude_stream_json":
			emitClaude(line, out)
		case "jsonl":
			emitJSONL(line, out)
		default:
			out <- Event{Kind: "text", Text: line}
		}
	}
}

// Claude Code --output-format stream-json: one JSON object per line.
func emitClaude(line string, out chan<- Event) {
	var m map[string]any
	if json.Unmarshal([]byte(line), &m) != nil {
		out <- Event{Kind: "text", Text: line}
		return
	}
	switch m["type"] {
	case "assistant", "user":
		msg, _ := m["message"].(map[string]any)
		for _, blk := range asList(msg["content"]) {
			b, _ := blk.(map[string]any)
			switch b["type"] {
			case "text":
				out <- Event{Kind: "text", Text: str(b["text"])}
			case "tool_use":
				out <- Event{Kind: "tool_call", Name: str(b["name"]), Payload: asMap(b["input"])}
			case "tool_result":
				out <- Event{Kind: "tool_result", Payload: asMap(b)}
			}
		}
		if u, ok := msg["usage"].(map[string]any); ok {
			out <- Event{Kind: "usage", TokensIn: num(u["input_tokens"]), TokensOut: num(u["output_tokens"])}
		}
	case "result":
		ev := Event{Kind: "usage", CostUSD: fnum(m["total_cost_usd"])}
		if u, ok := m["usage"].(map[string]any); ok {
			ev.TokensIn, ev.TokensOut = num(u["input_tokens"]), num(u["output_tokens"])
		}
		out <- ev
		if s := str(m["result"]); s != "" {
			out <- Event{Kind: "text", Text: s}
		}
	case "system":
		// session boot noise; skipped on purpose
	default:
		out <- Event{Kind: "text", Text: line}
	}
}

// Generic JSONL: best-effort field sniffing across tools that emit one JSON
// object per line without a shared schema.
func emitJSONL(line string, out chan<- Event) {
	var m map[string]any
	if json.Unmarshal([]byte(line), &m) != nil {
		out <- Event{Kind: "text", Text: line}
		return
	}
	for _, k := range []string{"text", "message", "content", "delta"} {
		if s := str(m[k]); s != "" {
			out <- Event{Kind: "text", Text: s}
			return
		}
	}
	if u, ok := m["usage"].(map[string]any); ok {
		out <- Event{Kind: "usage",
			TokensIn:  num(firstOf(u, "input_tokens", "prompt_tokens")),
			TokensOut: num(firstOf(u, "output_tokens", "completion_tokens"))}
		return
	}
	out <- Event{Kind: "text", Text: line}
}

func firstOf(m map[string]any, keys ...string) any {
	for _, k := range keys {
		if v, ok := m[k]; ok {
			return v
		}
	}
	return nil
}

func asList(v any) []any {
	l, _ := v.([]any)
	return l
}

func asMap(v any) map[string]any {
	m, _ := v.(map[string]any)
	return m
}

func str(v any) string {
	s, _ := v.(string)
	return s
}

func num(v any) int64 {
	f, _ := v.(float64)
	return int64(f)
}

func fnum(v any) float64 {
	f, _ := v.(float64)
	return f
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}
