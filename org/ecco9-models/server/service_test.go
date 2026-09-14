package server

import (
	"bufio"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func newTestService(t *testing.T) *Service {
	t.Helper()
	svc, err := New(t.TempDir())
	if err != nil {
		t.Fatalf("New: %v", err)
	}
	return svc
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

func TestRegisterAndListAndDelete(t *testing.T) {
	s := newTestService(t)
	// Register a GGUF blob directly.
	req := httptest.NewRequest(http.MethodPost, "/v1/models/register/mymodel", strings.NewReader("GGUFweights"))
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("register = %d, want 200; body=%q", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"name":"mymodel"`) {
		t.Fatalf("register missing name: %s", rec.Body.String())
	}

	// List should include it.
	lreq := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	lrec := httptest.NewRecorder()
	s.Routes().ServeHTTP(lrec, lreq)
	if !strings.Contains(lrec.Body.String(), "mymodel") {
		t.Fatalf("list missing model: %s", lrec.Body.String())
	}

	// Delete should remove it.
	dreq := httptest.NewRequest(http.MethodDelete, "/v1/models/mymodel", nil)
	drec := httptest.NewRecorder()
	s.Routes().ServeHTTP(drec, dreq)
	if !strings.Contains(drec.Body.String(), `"deleted":true`) {
		t.Fatalf("expected deleted:true, got %s", drec.Body.String())
	}

	// List is now empty.
	lreq2 := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	lrec2 := httptest.NewRecorder()
	s.Routes().ServeHTTP(lrec2, lreq2)
	if strings.Contains(lrec2.Body.String(), "mymodel") {
		t.Fatalf("model still listed after delete: %s", lrec2.Body.String())
	}
}

func TestRegisterRejectsBadFormat(t *testing.T) {
	s := newTestService(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/models/register/bad", strings.NewReader("XXXXdata"))
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("register bad format = %d, want 400", rec.Code)
	}
}

func TestConvertLifecycleEndpoints(t *testing.T) {
	s := newTestService(t)
	body := `{"source_path":"/src/m.safetensors","target_format":"gguf","quantization":"q4_0"}`
	req := httptest.NewRequest(http.MethodPost, "/v1/models/convert", strings.NewReader(body))
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("convert = %d, want 200; body=%q", rec.Code, rec.Body.String())
	}
	var out struct {
		JobID string `json:"job_id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &out); err != nil {
		t.Fatalf("decode convert response: %v", err)
	}
	if out.JobID == "" {
		t.Fatal("expected a job_id from convert")
	}

	// Poll the job until done.
	sreq := httptest.NewRequest(http.MethodGet, "/v1/models/convert/"+out.JobID, nil)
	srec := httptest.NewRecorder()
	s.Routes().ServeHTTP(srec, sreq)
	if srec.Code != http.StatusOK {
		t.Fatalf("convert status = %d, want 200", srec.Code)
	}
	if !strings.Contains(srec.Body.String(), `"id":"`+out.JobID+`"`) {
		t.Fatalf("status missing job id: %s", srec.Body.String())
	}
}

func TestConvertUnknownJobReturns404(t *testing.T) {
	s := newTestService(t)
	req := httptest.NewRequest(http.MethodGet, "/v1/models/convert/convert-999", nil)
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("unknown job = %d, want 404", rec.Code)
	}
}

func TestPullFromSeedDir(t *testing.T) {
	// Seed a source directory with a model blob and point the service at it.
	seed := t.TempDir()
	if err := os.WriteFile(filepath.Join(seed, "seedmodel"), []byte("GGUFseed"), 0o644); err != nil {
		t.Fatalf("write seed: %v", err)
	}
	t.Setenv("ECCO9_MODEL_SEED_DIR", seed)

	s := newTestService(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/models/pull", strings.NewReader(`{"name":"seedmodel"}`))
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	if ct := rec.Header().Get("Content-Type"); ct != "application/x-ndjson" {
		t.Fatalf("pull content-type = %q, want application/x-ndjson", ct)
	}
	var sawDone bool
	sc := bufio.NewScanner(rec.Body)
	for sc.Scan() {
		if strings.Contains(sc.Text(), `"done":true`) {
			sawDone = true
		}
	}
	if !sawDone {
		t.Fatalf("expected done progress in pull stream: %q", rec.Body.String())
	}

	// Model should now be registered.
	lreq := httptest.NewRequest(http.MethodGet, "/v1/models", nil)
	lrec := httptest.NewRecorder()
	s.Routes().ServeHTTP(lrec, lreq)
	if !strings.Contains(lrec.Body.String(), "seedmodel") {
		t.Fatalf("pulled model not registered: %s", lrec.Body.String())
	}
}

func TestPushStreamsProgress(t *testing.T) {
	s := newTestService(t)
	// Register then push.
	rreq := httptest.NewRequest(http.MethodPost, "/v1/models/register/pushme", strings.NewReader("GGUFp"))
	rrec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rrec, rreq)
	if rrec.Code != http.StatusOK {
		t.Fatalf("register = %d", rrec.Code)
	}

	req := httptest.NewRequest(http.MethodPost, "/v1/models/push", strings.NewReader(`{"name":"pushme"}`))
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	if ct := rec.Header().Get("Content-Type"); ct != "application/x-ndjson" {
		t.Fatalf("push content-type = %q, want application/x-ndjson", ct)
	}
	if !strings.Contains(rec.Body.String(), `"done":true`) {
		t.Fatalf("expected done in push stream: %q", rec.Body.String())
	}
}

func TestPullWithoutRemoteReportsError(t *testing.T) {
	// No seed dir configured -> the dirSource Fetch will fail, and the service
	// should emit an error progress line.
	t.Setenv("ECCO9_MODEL_SEED_DIR", filepath.Join(t.TempDir(), "nonexistent"))
	s := newTestService(t)
	req := httptest.NewRequest(http.MethodPost, "/v1/models/pull", strings.NewReader(`{"name":"ghost"}`))
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	if !strings.Contains(rec.Body.String(), "error") {
		t.Fatalf("expected error progress for missing remote: %q", rec.Body.String())
	}
}
