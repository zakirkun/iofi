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
		// Index is the position of the input this vector belongs to. Providers
		// are allowed to answer out of order, and a batch matched positionally
		// would attach every vector to the wrong text with nothing to notice it.
		Index     int       `json:"index"`
		Embedding []float32 `json:"embedding"`
	} `json:"data"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// Embed returns nil (no error) when no embedding model is configured, so
// memory still works in keyword-fallback mode.
func (c *Client) Embed(ctx context.Context, text string) ([]float32, error) {
	vecs, err := c.EmbedBatch(ctx, []string{text})
	if err != nil || len(vecs) == 0 {
		return nil, err
	}
	return vecs[0], nil
}

// embedBatchSize bounds one request. A long document is hundreds of chunks, and
// sending them one at a time is hundreds of round trips for work the provider
// accepts in a single call.
const embedBatchSize = 96

// EmbedBatch embeds several texts at once, returning one vector per input in
// the same order. A nil entry means that input was empty or no model is
// configured; callers store it as a null embedding and fall back to keyword
// search for that row rather than failing the whole document.
func (c *Client) EmbedBatch(ctx context.Context, texts []string) ([][]float32, error) {
	out := make([][]float32, len(texts))
	if c.EmbedModel == "" {
		return out, nil
	}
	// Empty inputs are rejected by the provider, so they are held back and
	// their slots left nil rather than shifting every later vector by one.
	idx := make([]int, 0, len(texts))
	batch := make([]string, 0, len(texts))
	for i, t := range texts {
		if strings.TrimSpace(t) == "" {
			continue
		}
		idx = append(idx, i)
		batch = append(batch, t)
	}
	for start := 0; start < len(batch); start += embedBatchSize {
		end := min(start+embedBatchSize, len(batch))
		var resp embedResponse
		if err := c.post(ctx, "/v1/embeddings",
			embedRequest{Model: c.EmbedModel, Input: batch[start:end]}, &resp); err != nil {
			return out, err
		}
		if resp.Error != nil {
			return out, fmt.Errorf("litellm embed: %s", resp.Error.Message)
		}
		if len(resp.Data) != end-start {
			return out, fmt.Errorf("litellm embed: asked for %d vectors, got %d",
				end-start, len(resp.Data))
		}
		for i, d := range resp.Data {
			at := d.Index
			if at < 0 || at >= end-start {
				at = i // provider omitted index; positional order is all there is
			}
			out[idx[start+at]] = d.Embedding
		}
	}
	return out, nil
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
