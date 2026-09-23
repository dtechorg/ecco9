package server

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/dtechorg/ecco9-reservoir/reservoir"
)

func newTestService() *Service {
	return New(32, reservoir.PersonaContemplativeScholar, 2)
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

func TestProcessEndpoint(t *testing.T) {
	s := newTestService()
	req := httptest.NewRequest(http.MethodPost, "/v1/reservoir/process", strings.NewReader(`{"input":[0.5,0.2,0.1]}`))
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("process = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"state"`) {
		t.Fatalf("process missing state: %s", rec.Body.String())
	}
}

func TestTrainEndpointRejectsMismatchedTargetDim(t *testing.T) {
	s := newTestService() // outDim = 2
	// Target of length 1 does not match outDim 2.
	req := httptest.NewRequest(http.MethodPost, "/v1/reservoir/train", strings.NewReader(`{"input":[0.5],"target":[0.9]}`))
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusBadRequest {
		t.Fatalf("mismatched target dim = %d, want 400; body=%q", rec.Code, rec.Body.String())
	}
}

func TestTrainEndpointAcceptsMatchingDim(t *testing.T) {
	s := newTestService() // outDim = 2
	req := httptest.NewRequest(http.MethodPost, "/v1/reservoir/train", strings.NewReader(`{"input":[0.5,0.2],"target":[0.9,0.1]}`))
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("train = %d, want 200; body=%q", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"samples_trained":1`) {
		t.Fatalf("expected samples_trained:1, got %s", rec.Body.String())
	}
}

func TestPersonaEndpoint(t *testing.T) {
	s := newTestService()
	req := httptest.NewRequest(http.MethodPost, "/v1/reservoir/persona", strings.NewReader(`{"persona":2}`))
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("persona = %d, want 200", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), `"applied":2`) {
		t.Fatalf("expected applied:2, got %s", rec.Body.String())
	}
}

func TestCheckpointValidateReloadFlow(t *testing.T) {
	s := newTestService() // outDim = 2
	// Train a few samples so there is something to checkpoint.
	for _, body := range []string{
		`{"input":[0.1,0.2],"target":[0.1,0.2]}`,
		`{"input":[0.3,0.4],"target":[0.3,0.4]}`,
		`{"input":[0.5,0.6],"target":[0.5,0.6]}`,
	} {
		req := httptest.NewRequest(http.MethodPost, "/v1/reservoir/train", strings.NewReader(body))
		rec := httptest.NewRecorder()
		s.Routes().ServeHTTP(rec, req)
		if rec.Code != http.StatusOK {
			t.Fatalf("train = %d, want 200; body=%q", rec.Code, rec.Body.String())
		}
	}

	// Capture a checkpoint.
	req := httptest.NewRequest(http.MethodPost, "/v1/reservoir/checkpoints", nil)
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("checkpoints = %d, want 200; body=%q", rec.Code, rec.Body.String())
	}
	var cpResp struct {
		Version  uint64 `json:"version"`
		Deployed uint64 `json:"deployed"`
	}
	if err := json.NewDecoder(rec.Body).Decode(&cpResp); err != nil {
		t.Fatalf("decode checkpoint: %v", err)
	}
	if cpResp.Version == 0 || cpResp.Deployed != cpResp.Version {
		t.Fatalf("first checkpoint should be deployed baseline: %+v", cpResp)
	}

	// Validate it against held-out samples.
	valBody := `{"version":` + itoa(cpResp.Version) + `,"held_out":[{"input":[0.2,0.3],"target":[0.2,0.3]}]}`
	req = httptest.NewRequest(http.MethodPost, "/v1/reservoir/checkpoints/validate", strings.NewReader(valBody))
	rec = httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("validate = %d, want 200; body=%q", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"held_out_mse"`) {
		t.Fatalf("validate missing held_out_mse: %s", rec.Body.String())
	}

	// Hot reload it.
	req = httptest.NewRequest(http.MethodPost, "/v1/reservoir/checkpoints/reload", strings.NewReader(`{"version":`+itoa(cpResp.Version)+`}`))
	rec = httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("reload = %d, want 200; body=%q", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"reloaded":true`) {
		t.Fatalf("expected reloaded:true, got %s", rec.Body.String())
	}
}

func TestValidateUnknownCheckpoint(t *testing.T) {
	s := newTestService()
	req := httptest.NewRequest(http.MethodPost, "/v1/reservoir/checkpoints/validate", strings.NewReader(`{"version":999,"held_out":[]}`))
	rec := httptest.NewRecorder()
	s.Routes().ServeHTTP(rec, req)
	if rec.Code != http.StatusNotFound {
		t.Fatalf("validate unknown = %d, want 404", rec.Code)
	}
}

func itoa(v uint64) string {
	return strings.TrimSpace(fmt.Sprintf("%d", v))
}
