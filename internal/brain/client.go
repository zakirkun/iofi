// Package brain wraps LiteLLM. It is the coordinator's reasoning, not the
// agents' — coding agents talk to their own providers through their CLI.
package brain

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type Client struct {
	BaseURL    string
	APIKey     string
	Model      string
	EmbedModel string
	HTTP       *http.Client
}

func New(baseURL, apiKey, model, embedModel string) *Client {
	return &Client{
		BaseURL:    strings.TrimRight(baseURL, "/"),
		APIKey:     apiKey,
		Model:      model,
		EmbedModel: embedModel,
		HTTP:       &http.Client{Timeout: 120 * time.Second},
	}
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model          string          `json:"model"`
	Messages       []Message       `json:"messages"`
	Temperature    float32         `json:"temperature,omitempty"`
	ResponseFormat *responseFormat `json:"response_format,omitempty"`
}

type responseFormat struct {
	Type string `json:"type"`
}

type chatResponse struct {
	Choices []struct {
		Message Message `json:"message"`
	} `json:"choices"`
	Usage struct {
		PromptTokens     int `json:"prompt_tokens"`
		CompletionTokens int `json:"completion_tokens"`
	} `json:"usage"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

type Usage struct {
	In  int
	Out int
}

func (c *Client) Chat(ctx context.Context, model string, msgs []Message, jsonOut bool) (string, Usage, error) {
	req := chatRequest{Model: pick(model, c.Model), Messages: msgs, Temperature: 0.2}
	if jsonOut {
		req.ResponseFormat = &responseFormat{Type: "json_object"}
	}
	var out chatResponse
	if err := c.post(ctx, "/v1/chat/completions", req, &out); err != nil {
		return "", Usage{}, err
	}
	if out.Error != nil {
		return "", Usage{}, fmt.Errorf("litellm: %s", out.Error.Message)
	}
	if len(out.Choices) == 0 {
		return "", Usage{}, fmt.Errorf("litellm: empty response")
	}
	return out.Choices[0].Message.Content,
		Usage{In: out.Usage.PromptTokens, Out: out.Usage.CompletionTokens}, nil
}

// ChatJSON parses the model's JSON reply into dst. Models wrap JSON in fences
// often enough that stripping them here is cheaper than retrying the call.
func (c *Client) ChatJSON(ctx context.Context, model string, msgs []Message, dst any) error {
	raw, _, err := c.Chat(ctx, model, msgs, true)
	if err != nil {
		return err
	}
	return json.Unmarshal([]byte(stripFence(raw)), dst)
}

type embedRequest struct {
	Model string   `json:"model"`
	Input []string `json:"input"`
}

type embedResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Embed returns nil (no error) when no embedding model is configured, so
// memory still works in keyword-fallback mode.
func (c *Client) Embed(ctx context.Context, text string) ([]float32, error) {
	if c.EmbedModel == "" || strings.TrimSpace(text) == "" {
		return nil, nil
	}
	var out embedResponse
	if err := c.post(ctx, "/v1/embeddings", embedRequest{Model: c.EmbedModel, Input: []string{text}}, &out); err != nil {
		return nil, err
	}
	if out.Error != nil {
		return nil, fmt.Errorf("litellm embed: %s", out.Error.Message)
	}
	if len(out.Data) == 0 {
		return nil, nil
	}
	return out.Data[0].Embedding, nil
}

func (c *Client) post(ctx context.Context, path string, body, dst any) error {
	buf, err := json.Marshal(body)
	if err != nil {
		return err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.BaseURL+path, bytes.NewReader(buf))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	if c.APIKey != "" {
		req.Header.Set("Authorization", "Bearer "+c.APIKey)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 8<<20))
	if err != nil {
		return err
	}
	if resp.StatusCode >= 300 {
		return fmt.Errorf("litellm %s: %s: %s", path, resp.Status, truncate(string(raw), 400))
	}
	return json.Unmarshal(raw, dst)
}

func stripFence(s string) string {
	s = strings.TrimSpace(s)
	if !strings.HasPrefix(s, "```") {
		return s
	}
	if i := strings.Index(s, "\n"); i >= 0 {
		s = s[i+1:]
	}
	return strings.TrimSpace(strings.TrimSuffix(strings.TrimSpace(s), "```"))
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "..."
}

func pick(v, def string) string {
	if v != "" {
		return v
	}
	return def
}
