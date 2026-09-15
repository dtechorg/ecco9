package server

import (
	"bufio"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dtechorg/ecco9-llm-gateway/gateway"
	"github.com/dtechorg/ecco9-sdk-go/health"
)

// newTestService builds a service with an explicit offline backend set so the
// tests do not depend on process environment variables.
func newTestService(t *testing.T) *Service {
	t.Helper()
	g, err := gateway.New([]*gateway.Backend{
		{Provider: gateway.ProviderAnthropic, APIKey: "k", BaseURL: "https://api.anthropic.com", Model: "claude", Priority: 100},
		{Provider: gateway.ProviderOpenAI, APIKey: "k", BaseURL: "https://api.openai.com", Model: "gpt", Priority: 70},
	}, &gateway.StubTransport{})
	if err != nil {
		t.Fatalf("gateway.New: %v", err)
	}
	return &Service{Gateway: g, Health: health.NewReporter()}
}

func TestHealthEndpoint(t *testing.T) {
	s := newTestService(t)
	req := httptest.NewRequest(http.MethodGet, "/healthz", nil)
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("healthz = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "identity_coherence") {
		t.Fatalf("healthz missing identity_coherence: %s", rec.Body.String())
	}
}

func TestGenerateEndpoint(t *testing.T) {
	s := newTestService(t)
	body := `{"request_id":"t1","messages":[{"role":"user","content":"hello echo"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/llm/generate", strings.NewReader(body))
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("generate = %d, want 200; body=%q", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"provider_used"`) {
		t.Fatalf("generate missing provider_used: %s", rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"request_id":"t1"`) {
		t.Fatalf("generate missing request_id: %s", rec.Body.String())
	}
}

func TestGenerateEndpointBadJSON(t *testing.T) {
	s := newTestService(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/llm/generate", strings.NewReader(`{not json`))
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("bad json = %d, want 400", rec.Code)
	}
}

func TestGenerateStreamEmitsNDJSONChunks(t *testing.T) {
	s := newTestService(t)
	body := `{"request_id":"s1","messages":[{"role":"user","content":"stream me"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/llm/generate/stream", strings.NewReader(body))
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	if ct := rec.Header().Get("Content-Type"); ct != "application/x-ndjson" {
		t.Fatalf("content-type = %q, want application/x-ndjson", ct)
	}
	lines := 0
	sc := bufio.NewScanner(rec.Body)
	for sc.Scan() {
		if strings.TrimSpace(sc.Text()) != "" {
			lines++
		}
	}
	if lines < 2 {
		t.Fatalf("expected at least 2 NDJSON chunks, got %d: %q", lines, rec.Body.String())
	}
}

func TestProviderStatusEndpoint(t *testing.T) {
	s := newTestService(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/llm/providers/status", nil)
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	for _, want := range []string{`"provider_health"`, `"provider_latency_ms"`, `"current_provider"`, `"anthropic"`, `"openai"`} {
		if !strings.Contains(body, want) {
			t.Fatalf("status missing %s: %s", want, body)
		}
	}
}
