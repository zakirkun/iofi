package runner

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/modelcontextprotocol/go-sdk/mcp"

	"iofi/internal/store"
)

// MCPClientAdapter drives a coding tool that exposes a whole agent turn as an
// MCP tool of its own (`codex mcp-server`). iofi is the client here, the
// mirror image of SpawnAdapter.
//
// The tool row supplies the process to start (command/args/env) and, in
// mcp_target, the name of the remote tool to call. Blank picks the first tool
// the server advertises, which is what single-tool servers expose anyway.
type MCPClientAdapter struct {
	Tool store.AgentTool
}

func (a *MCPClientAdapter) Kind() string { return a.Tool.Kind }

func (a *MCPClientAdapter) Start(ctx context.Context, req RunRequest) (<-chan Event, string, error) {
	vars := map[string]string{
		"Prompt": req.Prompt, "Workspace": req.Workspace,
		"Model": req.Model, "RunID": req.RunID,
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
	cmd.Env = os.Environ()
	for k, raw := range a.Tool.Env {
		s, _ := raw.(string)
		v, err := render(s, vars)
		if err != nil {
			return nil, "", err
		}
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	for k, v := range req.Env {
		cmd.Env = append(cmd.Env, k+"="+v)
	}
	cmd.Env = append(cmd.Env, "IOFI_RUN_ID="+req.RunID, "IOFI_MCP_URL="+req.MCPURL,
		"IOFI_MCP_TOKEN="+req.MCPToken)

	rendered := a.Tool.Command + " " + strings.Join(args, " ")
	client := mcp.NewClient(&mcp.Implementation{Name: "iofi", Version: "0.1.0"}, nil)
	sess, err := client.Connect(ctx, &mcp.CommandTransport{Command: cmd}, nil)
	if err != nil {
		return nil, rendered, fmt.Errorf("connect to %s: %w", a.Tool.Kind, err)
	}

	toolName, err := a.resolveToolName(ctx, sess)
	if err != nil {
		sess.Close()
		return nil, rendered, err
	}

	out := make(chan Event, 16)
	go func() {
		defer close(out)
		defer sess.Close()

		res, err := sess.CallTool(ctx, &mcp.CallToolParams{
			Name: toolName,
			Arguments: map[string]any{
				"prompt": req.Prompt,
				"cwd":    req.Workspace,
			},
		})
		if err != nil {
			out <- Event{Kind: "error", Text: truncate(err.Error(), 4000)}
			out <- Event{Kind: "done", ExitCode: 1}
			return
		}
		for _, c := range res.Content {
			if t, ok := c.(*mcp.TextContent); ok {
				out <- Event{Kind: "text", Text: t.Text}
			}
		}
		// Some servers return their token accounting in structured output.
		if in, o, ok := usageFrom(res.StructuredContent); ok {
			out <- Event{Kind: "usage", TokensIn: in, TokensOut: o}
		}
		if res.IsError {
			out <- Event{Kind: "error", Text: "the tool reported an error result"}
			out <- Event{Kind: "done", ExitCode: 1}
			return
		}
		out <- Event{Kind: "done"}
	}()
	return out, rendered + " (mcp: " + toolName + ")", nil
}

func (a *MCPClientAdapter) resolveToolName(ctx context.Context, sess *mcp.ClientSession) (string, error) {
	if a.Tool.MCPTarget != "" && !strings.ContainsAny(a.Tool.MCPTarget, `/\`) {
		return a.Tool.MCPTarget, nil
	}
	list, err := sess.ListTools(ctx, nil)
	if err != nil {
		return "", err
	}
	if len(list.Tools) == 0 {
		return "", fmt.Errorf("%s exposes no MCP tools", a.Tool.Kind)
	}
	return list.Tools[0].Name, nil
}

func usageFrom(structured any) (in, out int64, ok bool) {
	if structured == nil {
		return 0, 0, false
	}
	raw, err := json.Marshal(structured)
	if err != nil {
		return 0, 0, false
	}
	var probe struct {
		Usage struct {
			InputTokens  int64 `json:"input_tokens"`
			OutputTokens int64 `json:"output_tokens"`
			PromptTokens int64 `json:"prompt_tokens"`
			Completion   int64 `json:"completion_tokens"`
		} `json:"usage"`
	}
	if json.Unmarshal(raw, &probe) != nil {
		return 0, 0, false
	}
	in = probe.Usage.InputTokens + probe.Usage.PromptTokens
	out = probe.Usage.OutputTokens + probe.Usage.Completion
	return in, out, in > 0 || out > 0
}
