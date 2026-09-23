package cli

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// newTestClient returns a Client bound to a stub gateway running handler.
func newTestClient(t *testing.T, handler http.Handler) *Client {
	t.Helper()
	srv := httptest.NewServer(handler)
	t.Cleanup(srv.Close)
	c, err := NewClient(srv.URL)
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	return c
}

func TestNewClientAddsSchemeAndTrimsSlash(t *testing.T) {
	c, err := NewClient("example.com:8080/")
	if err != nil {
		t.Fatalf("NewClient: %v", err)
	}
	if got := c.BaseURL().String(); got != "http://example.com:8080" {
		t.Fatalf("base = %q, want http://example.com:8080", got)
	}
}

func TestClientFromEnvironmentDefaults(t *testing.T) {
	t.Setenv("ECCO9_HOST", "")
	t.Setenv("ECCO9_GATEWAY_ADDR", "")
	t.Setenv("ECCO9_API_KEY", "k")
	c, err := ClientFromEnvironment()
	if err != nil {
		t.Fatalf("ClientFromEnvironment: %v", err)
	}
	if got := c.BaseURL().String(); got != "http://127.0.0.1:8080" {
		t.Fatalf("default host = %q, want http://127.0.0.1:8080", got)
	}
	if c.APIKey != "k" {
		t.Fatalf("APIKey = %q, want k", c.APIKey)
	}
}

func TestClientSendsAPIKeyHeader(t *testing.T) {
	var gotKey string
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotKey = r.Header.Get("X-Ecco9-Key")
		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"version":"0.1.0"}`))
	}))
	c.APIKey = "secret"
	if _, err := c.Version(context.Background()); err != nil {
		t.Fatalf("Version: %v", err)
	}
	if gotKey != "secret" {
		t.Fatalf("X-Ecco9-Key = %q, want secret", gotKey)
	}
}

func TestGenerateAndChatRoundTrip(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/generate":
			var req GenerateRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			_ = json.NewEncoder(w).Encode(GenerateResponse{
				Model: req.Model, Response: "echo: " + req.Prompt, Done: true,
			})
		case "/api/chat":
			var req ChatRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			last := req.Messages[len(req.Messages)-1]
			_ = json.NewEncoder(w).Encode(ChatResponse{
				Model:   req.Model,
				Message: Message{Role: "assistant", Content: "echo: " + last.Content},
				Done:    true,
			})
		default:
			http.NotFound(w, r)
		}
	}))

	g, err := c.Generate(context.Background(), &GenerateRequest{Model: "m", Prompt: "hi"})
	if err != nil {
		t.Fatalf("Generate: %v", err)
	}
	if g.Response != "echo: hi" || !g.Done {
		t.Fatalf("GenerateResponse = %+v", g)
	}

	ch, err := c.Chat(context.Background(), &ChatRequest{
		Model:    "m",
		Messages: []Message{{Role: "user", Content: "hello"}},
	})
	if err != nil {
		t.Fatalf("Chat: %v", err)
	}
	if ch.Message.Role != "assistant" || ch.Message.Content != "echo: hello" {
		t.Fatalf("ChatResponse = %+v", ch)
	}
}

func TestTagsAndVersion(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/tags":
			_, _ = w.Write([]byte(`{"models":[{"name":"echo-7b","modified_at":"2026-09-22","size":42}]}`))
		case "/api/version":
			_, _ = w.Write([]byte(`{"version":"0.1.0-echo"}`))
		default:
			http.NotFound(w, r)
		}
	}))

	tags, err := c.Tags(context.Background())
	if err != nil {
		t.Fatalf("Tags: %v", err)
	}
	if len(tags.Models) != 1 || tags.Models[0].Name != "echo-7b" || tags.Models[0].Size != 42 {
		t.Fatalf("Tags = %+v", tags)
	}

	v, err := c.Version(context.Background())
	if err != nil {
		t.Fatalf("Version: %v", err)
	}
	if v != "0.1.0-echo" {
		t.Fatalf("Version = %q", v)
	}
}

func TestEchoEndpoints(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/api/echo/status":
			_ = json.NewEncoder(w).Encode(EchoStatus{
				Status: "SERVING", IdentityCoherence: 0.95,
				ServicesUp: 2, ServicesTotal: 2,
				Services: []EchoSvcHealth{{Service: "reservoir", Reachable: true}},
			})
		case "/api/echo/think":
			var req map[string]any
			_ = json.NewDecoder(r.Body).Decode(&req)
			_ = json.NewEncoder(w).Encode(map[string]any{"thought": req["prompt"]})
		case "/api/echo/feel":
			var req map[string]any
			_ = json.NewDecoder(r.Body).Decode(&req)
			_ = json.NewEncoder(w).Encode(map[string]any{"applied": req["emotion"]})
		case "/api/echo/remember":
			var req map[string]any
			_ = json.NewDecoder(r.Body).Decode(&req)
			_ = json.NewEncoder(w).Encode(map[string]any{"stored": req["key"]})
		default:
			http.NotFound(w, r)
		}
	}))

	st, err := c.EchoStatus(context.Background())
	if err != nil {
		t.Fatalf("EchoStatus: %v", err)
	}
	if st.Status != "SERVING" || st.IdentityCoherence != 0.95 || st.ServicesUp != 2 {
		t.Fatalf("EchoStatus = %+v", st)
	}

	think, err := c.EchoThink(context.Background(), "ponder")
	if err != nil || think["thought"] != "ponder" {
		t.Fatalf("EchoThink = %v, %v", think, err)
	}

	feel, err := c.EchoFeel(context.Background(), "joy", 0.8)
	if err != nil || feel["applied"] != "joy" {
		t.Fatalf("EchoFeel = %v, %v", feel, err)
	}

	rem, err := c.EchoRemember(context.Background(), "k", "v")
	if err != nil || rem["stored"] != "k" {
		t.Fatalf("EchoRemember = %v, %v", rem, err)
	}
}

func TestOrchestrationEndpoints(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/agents":
			_ = json.NewEncoder(w).Encode(map[string]any{
				"agents": []Agent{{ID: "agent-1", Name: "scholar", Type: "reflective"}},
			})
		case r.Method == http.MethodPost && r.URL.Path == "/tasks":
			var req OrchestrateRequest
			_ = json.NewDecoder(r.Body).Decode(&req)
			_ = json.NewEncoder(w).Encode(map[string]any{"id": "task-1", "type": req.Type})
		default:
			http.NotFound(w, r)
		}
	}))

	agents, err := c.ListAgents(context.Background())
	if err != nil {
		t.Fatalf("ListAgents: %v", err)
	}
	if len(agents) != 1 || agents[0].Name != "scholar" {
		t.Fatalf("ListAgents = %+v", agents)
	}

	out, err := c.Orchestrate(context.Background(), &OrchestrateRequest{Type: "cognitive", Input: "x"})
	if err != nil || out["id"] != "task-1" {
		t.Fatalf("Orchestrate = %v, %v", out, err)
	}
}

func TestErrorSurfacesStatusAndBody(t *testing.T) {
	c := newTestClient(t, http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "rate limit exceeded", http.StatusTooManyRequests)
	}))
	_, err := c.Version(context.Background())
	if err == nil {
		t.Fatal("expected error on 429")
	}
	if !strings.Contains(err.Error(), "429") || !strings.Contains(err.Error(), "rate limit") {
		t.Fatalf("error should carry status and body: %v", err)
	}
}
