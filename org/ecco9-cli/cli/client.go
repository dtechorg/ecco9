// Package cli provides the ecco9 command-line client SDK and the stdlib
// flag-based command dispatch for the `ecco9` binary. It keeps the Ollama
// CLI shape (generate/chat/serve/tags) while adding Deep Tree Echo and
// orchestration commands that talk to the ecco9-gateway REST endpoints.
package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"
)

// Client is a small HTTP SDK for the ecco9 gateway.
type Client struct {
	base *url.URL
	http *http.Client
	// APIKey, when set, is sent as the X-Ecco9-Key header.
	APIKey string
}

// NewClient returns a Client for the given gateway base URL.
func NewClient(rawurl string) (*Client, error) {
	if !strings.Contains(rawurl, "://") {
		rawurl = "http://" + rawurl
	}
	u, err := url.Parse(strings.TrimRight(rawurl, "/"))
	if err != nil {
		return nil, fmt.Errorf("parse gateway URL: %w", err)
	}
	return &Client{base: u, http: &http.Client{Timeout: 120 * time.Second}}, nil
}

// ClientFromEnvironment builds a Client from ECCO9_HOST / ECCO9_GATEWAY_ADDR,
// defaulting to http://127.0.0.1:8080. The API key comes from ECCO9_API_KEY.
func ClientFromEnvironment() (*Client, error) {
	host := os.Getenv("ECCO9_HOST")
	if host == "" {
		if addr := os.Getenv("ECCO9_GATEWAY_ADDR"); addr != "" {
			host = strings.Replace(addr, ":", "127.0.0.1:", 1)
		} else {
			host = "http://127.0.0.1:8080"
		}
	}
	c, err := NewClient(host)
	if err != nil {
		return nil, err
	}
	c.APIKey = os.Getenv("ECCO9_API_KEY")
	return c, nil
}

// BaseURL returns the gateway base URL.
func (c *Client) BaseURL() *url.URL { return c.base }

func (c *Client) do(ctx context.Context, method, path string, reqData, respData any) error {
	var body io.Reader
	if reqData != nil {
		b, err := json.Marshal(reqData)
		if err != nil {
			return fmt.Errorf("marshal request: %w", err)
		}
		body = bytes.NewReader(b)
	}
	req, err := http.NewRequestWithContext(ctx, method, c.base.String()+path, body)
	if err != nil {
		return fmt.Errorf("build request: %w", err)
	}
	if reqData != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	if c.APIKey != "" {
		req.Header.Set("X-Ecco9-Key", c.APIKey)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return fmt.Errorf("%s %s: %w", method, path, err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return fmt.Errorf("read response: %w", err)
	}
	if resp.StatusCode >= 400 {
		return fmt.Errorf("%s %s: %s: %s", method, path, resp.Status, strings.TrimSpace(string(raw)))
	}
	if respData != nil && len(raw) > 0 {
		if err := json.Unmarshal(raw, respData); err != nil {
			return fmt.Errorf("decode response: %w", err)
		}
	}
	return nil
}

// GenerateRequest mirrors the Ollama /api/generate request shape.
type GenerateRequest struct {
	Model   string         `json:"model,omitempty"`
	Prompt  string         `json:"prompt"`
	Stream  *bool          `json:"stream,omitempty"`
	Options map[string]any `json:"options,omitempty"`
}

// GenerateResponse mirrors the Ollama /api/generate response shape.
type GenerateResponse struct {
	Model    string `json:"model"`
	Response string `json:"response"`
	Done     bool   `json:"done"`
}

// ChatRequest mirrors the Ollama /api/chat request shape.
type ChatRequest struct {
	Model    string         `json:"model,omitempty"`
	Messages []Message      `json:"messages"`
	Stream   *bool          `json:"stream,omitempty"`
	Options  map[string]any `json:"options,omitempty"`
}

// Message is a single chat turn.
type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatResponse mirrors the Ollama /api/chat response shape.
type ChatResponse struct {
	Model   string  `json:"model"`
	Message Message `json:"message"`
	Done    bool    `json:"done"`
}

// TagsResponse mirrors the Ollama /api/tags response shape.
type TagsResponse struct {
	Models []struct {
		Name       string `json:"name"`
		ModifiedAt string `json:"modified_at,omitempty"`
		Size       int64  `json:"size,omitempty"`
	} `json:"models"`
}

// Generate calls POST /api/generate.
func (c *Client) Generate(ctx context.Context, req *GenerateRequest) (*GenerateResponse, error) {
	var out GenerateResponse
	if err := c.do(ctx, http.MethodPost, "/api/generate", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Chat calls POST /api/chat.
func (c *Client) Chat(ctx context.Context, req *ChatRequest) (*ChatResponse, error) {
	var out ChatResponse
	if err := c.do(ctx, http.MethodPost, "/api/chat", req, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Tags calls GET /api/tags.
func (c *Client) Tags(ctx context.Context) (*TagsResponse, error) {
	var out TagsResponse
	if err := c.do(ctx, http.MethodGet, "/api/tags", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// Version calls GET /api/version and returns the version string.
func (c *Client) Version(ctx context.Context) (string, error) {
	var out struct {
		Version string `json:"version"`
	}
	if err := c.do(ctx, http.MethodGet, "/api/version", nil, &out); err != nil {
		return "", err
	}
	return out.Version, nil
}

// EchoStatus is the aggregate Deep Tree Echo status document.
type EchoStatus struct {
	Status            string          `json:"status"`
	IdentityCoherence float64         `json:"identity_coherence"`
	ServicesUp        int             `json:"services_up"`
	ServicesTotal     int             `json:"services_total"`
	Services          []EchoSvcHealth `json:"services"`
}

// EchoSvcHealth is one backend's entry in the aggregate status.
type EchoSvcHealth struct {
	Service           string  `json:"service"`
	URL               string  `json:"url"`
	Reachable         bool    `json:"reachable"`
	IdentityCoherence float64 `json:"identity_coherence"`
	CognitiveLoad     float64 `json:"cognitive_load"`
	Error             string  `json:"error,omitempty"`
}

// EchoStatus calls GET /api/echo/status.
func (c *Client) EchoStatus(ctx context.Context) (*EchoStatus, error) {
	var out EchoStatus
	if err := c.do(ctx, http.MethodGet, "/api/echo/status", nil, &out); err != nil {
		return nil, err
	}
	return &out, nil
}

// EchoThink calls POST /api/echo/think with a cognitive prompt.
func (c *Client) EchoThink(ctx context.Context, prompt string) (map[string]any, error) {
	var out map[string]any
	err := c.do(ctx, http.MethodPost, "/api/echo/think", map[string]any{"prompt": prompt}, &out)
	return out, err
}

// EchoFeel calls POST /api/echo/feel with an emotion name and intensity.
func (c *Client) EchoFeel(ctx context.Context, emotion string, intensity float64) (map[string]any, error) {
	var out map[string]any
	err := c.do(ctx, http.MethodPost, "/api/echo/feel",
		map[string]any{"emotion": emotion, "intensity": intensity}, &out)
	return out, err
}

// EchoRemember calls POST /api/echo/remember with a key/value memory.
func (c *Client) EchoRemember(ctx context.Context, key, value string) (map[string]any, error) {
	var out map[string]any
	err := c.do(ctx, http.MethodPost, "/api/echo/remember",
		map[string]any{"key": key, "value": value}, &out)
	return out, err
}

// Agent is a cognitive agent registered with the orchestrator.
type Agent struct {
	ID           string   `json:"id"`
	Name         string   `json:"name"`
	Type         string   `json:"type"`
	Capabilities []string `json:"capabilities,omitempty"`
}

// ListAgents calls GET /api/orchestration/agents on the gateway (proxied to
// the orchestrator when that route is mounted, else to the orchestrator
// backend directly via /agents).
func (c *Client) ListAgents(ctx context.Context) ([]Agent, error) {
	var out struct {
		Agents []Agent `json:"agents"`
	}
	if err := c.do(ctx, http.MethodGet, "/agents", nil, &out); err != nil {
		return nil, err
	}
	return out.Agents, nil
}

// OrchestrateRequest submits a task to the orchestration engine.
type OrchestrateRequest struct {
	Type  string `json:"type"`
	Input string `json:"input"`
}

// Orchestrate calls POST /tasks on the orchestration backend and returns the
// created task document.
func (c *Client) Orchestrate(ctx context.Context, req *OrchestrateRequest) (map[string]any, error) {
	var out map[string]any
	err := c.do(ctx, http.MethodPost, "/tasks", req, &out)
	return out, err
}
