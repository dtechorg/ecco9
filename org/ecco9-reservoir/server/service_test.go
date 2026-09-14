package server

import (
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
