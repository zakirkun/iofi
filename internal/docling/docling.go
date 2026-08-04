// Package docling converts uploaded documents to markdown by calling a
// docling-serve instance over HTTP.
//
// Docling is a Python project, so it runs as its own container next to iofi
// rather than inside this process — the same arrangement as LiteLLM. When it is
// not configured, Extract still handles the formats that are already text, so
// uploading a markdown specification works on a laptop with nothing else
// running.
package docling

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"path/filepath"
	"strings"
	"time"
	"unicode/utf8"
)

// Client talks to docling-serve. A zero BaseURL disables it.
type Client struct {
	BaseURL string
	HTTP    *http.Client
	OCR     bool // run OCR on scanned pages; slower and needs the OCR models present
}

// New builds a client. The timeout is generous because converting a long
// scanned PDF is genuinely slow, and a document half-extracted is worse than
// one that took a minute.
func New(baseURL string, ocr bool) *Client {
	return &Client{
		BaseURL: strings.TrimRight(baseURL, "/"),
		HTTP:    &http.Client{Timeout: 10 * time.Minute},
		OCR:     ocr,
	}
}

func (c *Client) Enabled() bool { return c != nil && c.BaseURL != "" }

// plainText lists what this process can read without help. Everything else
// needs docling.
var plainText = map[string]bool{
	".md": true, ".markdown": true, ".txt": true, ".text": true,
	".csv": true, ".tsv": true, ".json": true, ".yaml": true, ".yml": true,
	".log": true, ".rst": true, ".adoc": true, ".asciidoc": true,
}

// Supported reports whether an extension can be handled at all, given whether
// docling is available. Rejecting an upload at the door beats accepting it and
// storing a document that permanently reads as failed.
func Supported(ext string, doclingUp bool) bool {
	ext = strings.ToLower(ext)
	if plainText[ext] {
		return true
	}
	if !doclingUp {
		return false
	}
	switch ext {
	case ".pdf", ".docx", ".pptx", ".xlsx", ".html", ".htm", ".xhtml",
		".png", ".jpg", ".jpeg", ".tiff", ".tif", ".bmp", ".webp":
		return true
	}
	return false
}

// Result is what came back, and by which route.
type Result struct {
	Markdown  string
	Extractor string // "docling" or "plain"
}

// Extract turns a document into markdown.
//
// A format this process understands is never sent to docling: a round trip to
// another container to read a text file is latency bought for nothing, and it
// makes uploading a runbook depend on a service that may not be running.
func (c *Client) Extract(ctx context.Context, filename string, data []byte) (Result, error) {
	ext := strings.ToLower(filepath.Ext(filename))
	if plainText[ext] {
		if !utf8.Valid(data) {
			return Result{}, fmt.Errorf("%s is not valid UTF-8 text", filename)
		}
		return Result{Markdown: string(data), Extractor: "plain"}, nil
	}
	if !c.Enabled() {
		return Result{}, fmt.Errorf(
			"cannot read %s: %s needs document conversion and IOFI_DOCLING_URL is not set. "+
				"Start the docling service or upload the content as markdown or plain text", filename, ext)
	}
	md, err := c.convert(ctx, filename, data)
	if err != nil {
		return Result{}, err
	}
	if strings.TrimSpace(md) == "" {
		return Result{}, fmt.Errorf("%s converted to an empty document; if it is a scan, "+
			"enable OCR with IOFI_DOCLING_OCR=1", filename)
	}
	return Result{Markdown: md, Extractor: "docling"}, nil
}

// convertResponse mirrors docling-serve's synchronous reply. Only md_content is
// requested, so the other content fields stay empty and are not modelled.
type convertResponse struct {
	Document struct {
		MDContent string `json:"md_content"`
	} `json:"document"`
	Status string `json:"status"` // success | partial_success | skipped | failure
	Errors []any  `json:"errors"`
}

func (c *Client) convert(ctx context.Context, filename string, data []byte) (string, error) {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)

	fields := [][2]string{
		{"to_formats", "md"},
		{"image_export_mode", "placeholder"}, // embedded images would bloat the row for no gain
		{"table_mode", "accurate"},           // tables in a specification are usually the point
		{"abort_on_error", "false"},
		{"do_ocr", boolStr(c.OCR)},
	}
	for _, f := range fields {
		if err := w.WriteField(f[0], f[1]); err != nil {
			return "", err
		}
	}
	part, err := w.CreateFormFile("files", filepath.Base(filename))
	if err != nil {
		return "", err
	}
	if _, err := part.Write(data); err != nil {
		return "", err
	}
	if err := w.Close(); err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost,
		c.BaseURL+"/v1/convert/file", &body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Accept", "application/json")

	resp, err := c.HTTP.Do(req)
	if err != nil {
		return "", fmt.Errorf("docling at %s: %w", c.BaseURL, err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 64<<20))
	if resp.StatusCode >= 300 {
		return "", fmt.Errorf("docling %s: %s", resp.Status, truncate(strings.TrimSpace(string(raw)), 400))
	}

	var out convertResponse
	if err := json.Unmarshal(raw, &out); err != nil {
		return "", fmt.Errorf("docling returned something that is not a conversion result: %w", err)
	}
	// partial_success still carries usable text; failure does not, and returning
	// it would store a fragment as if it were the whole document.
	if out.Status == "failure" {
		return "", fmt.Errorf("docling could not convert %s: %v", filename, out.Errors)
	}
	return out.Document.MDContent, nil
}

// Health reports whether the service answers, for the /api/health panel.
func (c *Client) Health(ctx context.Context) error {
	if !c.Enabled() {
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.BaseURL+"/health", nil)
	if err != nil {
		return err
	}
	client := &http.Client{Timeout: 5 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	io.Copy(io.Discard, io.LimitReader(resp.Body, 1<<10))
	if resp.StatusCode >= 300 {
		return fmt.Errorf("docling health: %s", resp.Status)
	}
	return nil
}

func boolStr(b bool) string {
	if b {
		return "true"
	}
	return "false"
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n] + "…"
}
