// Package github keeps the kanban and a GitHub repository in step, in both
// directions, without the two ends echoing each other forever.
package github

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

const apiBase = "https://api.github.com"

type Client struct {
	Token string
	HTTP  *http.Client
}

func NewClient(token string) *Client {
	return &Client{Token: token, HTTP: &http.Client{Timeout: 30 * time.Second}}
}

func (c *Client) Enabled() bool { return c != nil && c.Token != "" }

type Issue struct {
	Number  int      `json:"number"`
	NodeID  string   `json:"node_id"`
	Title   string   `json:"title"`
	Body    string   `json:"body"`
	State   string   `json:"state"`
	HTMLURL string   `json:"html_url"`
	Labels  []Label  `json:"labels"`
	PR      *PRRef   `json:"pull_request,omitempty"`
	Assign  []string `json:"-"`
}

type Label struct {
	Name string `json:"name"`
}

type PRRef struct {
	URL string `json:"url"`
}

type PullRequest struct {
	Number  int    `json:"number"`
	NodeID  string `json:"node_id"`
	HTMLURL string `json:"html_url"`
	State   string `json:"state"`
	Merged  bool   `json:"merged"`
	Head    struct {
		Ref string `json:"ref"`
	} `json:"head"`
}

func (c *Client) CreateIssue(ctx context.Context, repo, title, body string, labels []string) (*Issue, error) {
	var out Issue
	err := c.do(ctx, http.MethodPost, fmt.Sprintf("/repos/%s/issues", repo), map[string]any{
		"title": title, "body": body, "labels": labels,
	}, &out)
	return &out, err
}

func (c *Client) UpdateIssue(ctx context.Context, repo string, number int, patch map[string]any) (*Issue, error) {
	var out Issue
	err := c.do(ctx, http.MethodPatch, fmt.Sprintf("/repos/%s/issues/%d", repo, number), patch, &out)
	return &out, err
}

func (c *Client) Comment(ctx context.Context, repo string, number int, body string) error {
	return c.do(ctx, http.MethodPost,
		fmt.Sprintf("/repos/%s/issues/%d/comments", repo, number),
		map[string]any{"body": body}, nil)
}

func (c *Client) CreatePR(ctx context.Context, repo, title, body, head, base string) (*PullRequest, error) {
	var out PullRequest
	err := c.do(ctx, http.MethodPost, fmt.Sprintf("/repos/%s/pulls", repo), map[string]any{
		"title": title, "body": body, "head": head, "base": base,
	}, &out)
	return &out, err
}

func (c *Client) GetPR(ctx context.Context, repo string, number int) (*PullRequest, error) {
	var out PullRequest
	err := c.do(ctx, http.MethodGet, fmt.Sprintf("/repos/%s/pulls/%d", repo, number), nil, &out)
	return &out, err
}

func (c *Client) do(ctx context.Context, method, path string, body, dst any) error {
	if !c.Enabled() {
		return fmt.Errorf("github token not configured")
	}
	var rdr io.Reader
	if body != nil {
		buf, err := json.Marshal(body)
		if err != nil {
			return err
		}
		rdr = bytes.NewReader(buf)
	}
	req, err := http.NewRequestWithContext(ctx, method, apiBase+path, rdr)
	if err != nil {
		return err
	}
	req.Header.Set("Authorization", "Bearer "+c.Token)
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("X-GitHub-Api-Version", "2022-11-28")
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode >= 300 {
		return fmt.Errorf("github %s %s: %s: %s", method, path, resp.Status,
			strings.TrimSpace(string(raw)))
	}
	if dst == nil {
		return nil
	}
	return json.Unmarshal(raw, dst)
}
