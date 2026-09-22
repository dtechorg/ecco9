package gateway

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dtechorg/ecco9-sdk-go/contracts"
)

// newTestGateway points every backend at the given stub server.
func newTestGateway(t *testing.T, backend http.Handler) *Gateway {
	t.Helper()
	stub := httptest.NewServer(backend)
	t.Cleanup(stub.Close)

	backends := make(map[string]string, len(defaultBackends))
	for name := range defaultBackends {
		backends[name] = stub.URL
	}
	return NewWithConfig(backends, AuthConfig{}, 1000, 1000)
}

func TestResolveBackendsHonorsEnvOverride(t *testing.T) {
	t.Setenv("ECCO9_SVC_LLM_GATEWAY", "http://127.0.0.1:9999/")
	backends := ResolveBackends()
	if got := backends[SvcLLMGateway]; got != "http://127.0.0.1:9999" {
		t.Fatalf("override = %q, want trailing slash trimmed", got)
	}
	if got := backends[SvcReservoir]; got != defaultBackends[SvcReservoir] {
		t.Fatalf("default backend = %q, want %q", got, defaultBackends[SvcReservoir])
	}
}

func TestRouteTableCoversExternalSurfaces(t *testing.T) {
	paths := make(map[string]string)
	for _, r := range RouteTable() {
		paths[r.Path] = r.Service
	}
	want := map[string]string{
		"/api/generate":        SvcLLMGateway,
		"/api/chat":            SvcLLMGateway,
		"/api/tags":            SvcModels,
		"/v1/chat/completions": SvcLLMGateway,
		"/v1/models":           SvcModels,
		"/api/echo/think":      SvcReservoir,
		"/api/echo/feel":       SvcEmotion,
		"/api/echo/remember":   SvcMemory,
		"/agents":              SvcOrchestrator,
		"/tasks":               SvcOrchestrator,
		"/workflows":           SvcOrchestrator,
		"/v1/orchestrator/":    SvcOrchestrator,
	}
	for p, svc := range want {
		if got, ok := paths[p]; !ok || got != svc {
			t.Fatalf("route %s -> %q (ok=%v), want %q", p, got, ok, svc)
		}
	}
}

func TestProxyForwardsToBackend(t *testing.T) {
	backend := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path == "/api/generate" {
			w.Header().Set("Content-Type", "application/json")
			fmt.Fprint(w, `{"response":"echo"}`)
			return
		}
		http.NotFound(w, r)
	})
	g := newTestGateway(t, backend)

	req := httptest.NewRequest(http.MethodPost, "/api/generate", strings.NewReader(`{"prompt":"hi"}`))
	rec := httptest.NewRecorder()
	g.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK || !strings.Contains(rec.Body.String(), "echo") {
		t.Fatalf("proxy = %d; body=%q", rec.Code, rec.Body.String())
	}
}

func TestOrchestrationShortFormsRewriteToV1Base(t *testing.T) {
	var gotPaths []string
	backend := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		gotPaths = append(gotPaths, r.Method+" "+r.URL.Path)
		w.Header().Set("Content-Type", "application/json")
		fmt.Fprint(w, `{"id":"x"}`)
	})
	g := newTestGateway(t, backend)

	for _, path := range []string{"/agents", "/tasks", "/workflows"} {
		req := httptest.NewRequest(http.MethodGet, path, nil)
		rec := httptest.NewRecorder()
		g.Routes().ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("%s = %d, want 200; body=%q", path, rec.Code, rec.Body.String())
		}
	}
	// Long-form route passes through unchanged.
	req := httptest.NewRequest(http.MethodGet, "/v1/orchestrator/load", nil)
	rec := httptest.NewRecorder()
	g.Routes().ServeHTTP(rec, req)

	want := []string{
		"GET /v1/orchestrator/agents",
		"GET /v1/orchestrator/tasks",
		"GET /v1/orchestrator/workflows",
		"GET /v1/orchestrator/load",
	}
	if fmt.Sprint(gotPaths) != fmt.Sprint(want) {
		t.Fatalf("backend saw %v, want %v", gotPaths, want)
	}
}

func TestNoBackendConfiguredReturns503(t *testing.T) {
	g := NewWithConfig(map[string]string{}, AuthConfig{}, 1000, 1000)
	req := httptest.NewRequest(http.MethodPost, "/api/generate", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	g.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusServiceUnavailable {
		t.Fatalf("no backend = %d, want 503", rec.Code)
	}
}

func TestBackendUnreachableReturns502(t *testing.T) {
	// Point at a server that is already closed.
	dead := httptest.NewServer(http.NotFoundHandler())
	deadURL := dead.URL
	dead.Close()

	g := NewWithConfig(map[string]string{SvcLLMGateway: deadURL}, AuthConfig{}, 1000, 1000)
	req := httptest.NewRequest(http.MethodPost, "/api/generate", strings.NewReader(`{}`))
	rec := httptest.NewRecorder()
	g.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadGateway {
		t.Fatalf("dead backend = %d, want 502; body=%q", rec.Code, rec.Body.String())
	}
}

func TestAuthMiddleware(t *testing.T) {
	backend := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{}`)
	})
	stub := httptest.NewServer(backend)
	t.Cleanup(stub.Close)

	backends := map[string]string{SvcModels: stub.URL}
	auth := AuthConfig{
		Enabled: true,
		Header:  "X-Ecco9-Key",
		Keys:    map[string]bool{"secret": true},
		Exempt:  []string{"/healthz", "/api/version"},
	}
	g := NewWithConfig(backends, auth, 1000, 1000)

	// Missing key -> 401.
	req := httptest.NewRequest(http.MethodGet, "/api/tags", nil)
	rec := httptest.NewRecorder()
	g.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("no key = %d, want 401", rec.Code)
	}

	// Valid key -> proxied.
	req = httptest.NewRequest(http.MethodGet, "/api/tags", nil)
	req.Header.Set("X-Ecco9-Key", "secret")
	rec = httptest.NewRecorder()
	g.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("valid key = %d, want 200", rec.Code)
	}

	// Exempt path skips auth even with no key.
	req = httptest.NewRequest(http.MethodGet, "/api/version", nil)
	rec = httptest.NewRecorder()
	g.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("exempt path = %d, want 200", rec.Code)
	}
}

func TestRateLimiterRejectsBeyondBurst(t *testing.T) {
	backend := http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		fmt.Fprint(w, `{}`)
	})
	stub := httptest.NewServer(backend)
	t.Cleanup(stub.Close)

	// rate ~0, burst 2: first two requests pass, third is rejected.
	g := NewWithConfig(map[string]string{SvcModels: stub.URL}, AuthConfig{}, 0.0001, 2)
	var codes []int
	for i := 0; i < 3; i++ {
		req := httptest.NewRequest(http.MethodGet, "/api/tags", nil)
		rec := httptest.NewRecorder()
		g.Handler().ServeHTTP(rec, req)
		codes = append(codes, rec.Code)
	}
	if codes[0] != 200 || codes[1] != 200 || codes[2] != http.StatusTooManyRequests {
		t.Fatalf("codes = %v, want [200 200 429]", codes)
	}
}

func TestEchoStatusAggregatesBackends(t *testing.T) {
	backend := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/healthz" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":             contracts.ServingStatusServing,
			"identity_coherence": 0.9,
			"cognitive_load":     0.1,
		})
	})
	g := newTestGateway(t, backend)

	req := httptest.NewRequest(http.MethodGet, "/api/echo/status", nil)
	rec := httptest.NewRecorder()
	g.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("echo status = %d, want 200", rec.Code)
	}
	var out struct {
		Status            string  `json:"status"`
		IdentityCoherence float64 `json:"identity_coherence"`
		ServicesUp        int     `json:"services_up"`
		ServicesTotal     int     `json:"services_total"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v; body=%q", err, rec.Body.String())
	}
	if out.Status != "SERVING" {
		t.Fatalf("status = %q, want SERVING", out.Status)
	}
	if out.ServicesUp != out.ServicesTotal || out.ServicesTotal != len(serviceOrder) {
		t.Fatalf("up/total = %d/%d, want %d/%d", out.ServicesUp, out.ServicesTotal, len(serviceOrder), len(serviceOrder))
	}
	if out.IdentityCoherence < 0.89 || out.IdentityCoherence > 0.91 {
		t.Fatalf("coherence = %v, want ~0.9", out.IdentityCoherence)
	}
}

func TestEchoStatusDegradedWhenSomeBackendsDown(t *testing.T) {
	live := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":             contracts.ServingStatusServing,
			"identity_coherence": 0.8,
		})
	}))
	t.Cleanup(live.Close)
	dead := httptest.NewServer(http.NotFoundHandler())
	deadURL := dead.URL
	dead.Close()

	backends := make(map[string]string, len(defaultBackends))
	for name := range defaultBackends {
		backends[name] = deadURL
	}
	backends[SvcReservoir] = live.URL

	g := NewWithConfig(backends, AuthConfig{}, 1000, 1000)
	req := httptest.NewRequest(http.MethodGet, "/api/echo/status", nil)
	rec := httptest.NewRecorder()
	g.Routes().ServeHTTP(rec, req)

	var out struct {
		Status     string `json:"status"`
		ServicesUp int    `json:"services_up"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if out.Status != "DEGRADED" || out.ServicesUp != 1 {
		t.Fatalf("status=%q up=%d, want DEGRADED up=1", out.Status, out.ServicesUp)
	}
}

func TestRootHandlerReportsIdentity(t *testing.T) {
	g := NewWithConfig(map[string]string{}, AuthConfig{}, 1000, 1000)
	req := httptest.NewRequest(http.MethodGet, "/", nil)
	rec := httptest.NewRecorder()
	g.Routes().ServeHTTP(rec, req)
	if !strings.Contains(rec.Body.String(), "Deep Tree Echo") {
		t.Fatalf("root missing identity: %s", rec.Body.String())
	}
}
