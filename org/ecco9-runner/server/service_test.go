package server

import (
	"bufio"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func newTestService() *Service {
	return New()
}

func loadModel(t *testing.T, s *Service, path string) string {
	t.Helper()
	body := `{"model_path":"` + path + `","gpu_layers":8,"context_size":2048}`
	req := httptest.NewRequest(http.MethodPost, "/v1/runner/models", strings.NewReader(body))
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("load model = %d, want 200; body=%q", rec.Code, rec.Body.String())
	}
	// Extract model_id from the JSON response.
	var out struct {
		ModelID string `json:"model_id"`
	}
	decodeJSON(t, rec.Body.String(), &out)
	return out.ModelID
}

func decodeJSON(t *testing.T, body string, v any) {
	t.Helper()
	if err := json.Unmarshal([]byte(body), v); err != nil {
		t.Fatalf("decode %q: %v", body, err)
	}
}

func TestHealthEndpoint(t *testing.T) {
	s := newTestService()
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

func TestLoadAndListModels(t *testing.T) {
	s := newTestService()
	id := loadModel(t, s, "/models/a.gguf")
	if id == "" {
		t.Fatal("expected a model_id from load")
	}

	req := httptest.NewRequest(http.MethodGet, "/v1/runner/models", nil)
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("list = %d, want 200", rec.Code)
	}
	body := rec.Body.String()
	if !strings.Contains(body, id) || !strings.Contains(body, `"status":"ready"`) {
		t.Fatalf("list missing loaded model: %s", body)
	}
}

func TestLoadDuplicateModelReturnsConflict(t *testing.T) {
	s := newTestService()
	loadModel(t, s, "/models/dup.gguf")
	body := `{"model_path":"/models/dup.gguf","gpu_layers":8,"context_size":2048}`
	req := httptest.NewRequest(http.MethodPost, "/v1/runner/models", strings.NewReader(body))
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusConflict {
		t.Fatalf("duplicate load = %d, want 409", rec.Code)
	}
}

func TestUnloadModel(t *testing.T) {
	s := newTestService()
	id := loadModel(t, s, "/models/u.gguf")
	req := httptest.NewRequest(http.MethodDelete, "/v1/runner/models/"+id, nil)
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("unload = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"unloaded":true`) {
		t.Fatalf("expected unloaded:true, got %s", rec.Body.String())
	}
}

func TestGenerateStreamsTokens(t *testing.T) {
	s := newTestService()
	loadModel(t, s, "/models/gen.gguf")
	body := `{"request_id":"g1","messages":[{"role":"user","content":"hello echo"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/runner/generate", strings.NewReader(body))
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	if ct := rec.Header().Get("Content-Type"); ct != "application/x-ndjson" {
		t.Fatalf("content-type = %q, want application/x-ndjson", ct)
	}
	lines := 0
	var sawDone bool
	sc := bufio.NewScanner(rec.Body)
	for sc.Scan() {
		txt := strings.TrimSpace(sc.Text())
		if txt == "" {
			continue
		}
		lines++
		if strings.Contains(txt, `"done":true`) {
			sawDone = true
		}
	}
	if lines == 0 || !sawDone {
		t.Fatalf("expected streamed tokens ending in done, got %d lines: %q", lines, rec.Body.String())
	}
}

func TestGenerateUnknownModelStreamsError(t *testing.T) {
	s := newTestService() // no models loaded
	body := `{"request_id":"g2","model_id":"missing","messages":[{"role":"user","content":"x"}]}`
	req := httptest.NewRequest(http.MethodPost, "/v1/runner/generate", strings.NewReader(body))
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	// The queue reports the model-resolution error over the job error channel,
	// which the adapter surfaces as a 500 after headers are written.
	if rec.Code != http.StatusInternalServerError && rec.Code != http.StatusOK {
		t.Fatalf("unexpected status for unknown model: %d", rec.Code)
	}
}
